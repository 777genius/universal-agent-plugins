package installer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// PendingNativeIntent exposes the exact persisted attempt and selected facts.
// Diagnostic serialization cannot grant selected-delivery authority. Digest
// binds the complete binding (including ownership and revision), not settings
// document bytes: unrelated foreign edits are reconciled by the native adapter.
type PendingNativeIntent struct {
	Binding           BindingFacts
	Intent            domain.PendingNativeIntent `json:"-"`
	NativeProfileRoot string
	Digest            string
}

func inspectNativeIntents(state domain.StateFileV2, observed ...ports.NativeTransitionObservation) ([]PendingNativeIntent, error) {
	var out []PendingNativeIntent
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if binding.PendingNativeIntent == nil {
				// Legacy user native-config attempts predate persisted selected intents.
				// Inspection preserves their uncertainty without granting mutation authority.
				legacyNativeConfig := binding.SelectedDelivery.IsZero() && binding.Scope == string(domain.ScopeUser) && binding.SelectedDelivery.EffectiveTraitsForScope(domain.ClientID(binding.ClientID), domain.ScopeUser).LifecycleKind == domain.LifecycleNativeConfig
				if binding.NativeActivationAttempt != "" && !legacyNativeConfig && (binding.ClientID != "opencode" || !binding.SelectedDelivery.IsZero()) && !observedNativeAttempt(observed, installation.InstallationID, binding) {
					return out, fmt.Errorf("native attempt has no persisted intent; retain uncertainty")
				}
				continue
			}
			pending, err := inspectNativeIntent(installation, binding)
			if err != nil {
				return out, err
			}
			out = append(out, pending)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Binding.InstallationID != out[j].Binding.InstallationID {
			return out[i].Binding.InstallationID < out[j].Binding.InstallationID
		}
		return out[i].Binding.BindingID < out[j].Binding.BindingID
	})
	return out, nil
}

func inspectNativeIntent(installation domain.Installation, binding domain.ClientBinding) (PendingNativeIntent, error) {
	if err := validateSelectedBindingIdentity(binding); err != nil {
		return PendingNativeIntent{}, err
	}
	intent := *binding.PendingNativeIntent
	if err := intent.Validate(binding); err != nil {
		return PendingNativeIntent{}, err
	}
	if !reflect.DeepEqual(intent.Delivery, binding.SelectedDelivery) {
		return PendingNativeIntent{}, fmt.Errorf("native intent differs from complete binding selection")
	}
	body, err := json.Marshal(binding)
	if err != nil {
		return PendingNativeIntent{}, err
	}
	sum := sha256.Sum256(body)
	receipt := installation.DataReceipts[binding.DataReceiptID]
	return PendingNativeIntent{
		Binding: BindingFacts{InstallationID: installation.InstallationID, ClientID: binding.ClientID, BindingID: binding.ClientBindingID, Scope: binding.Scope, TargetPath: binding.TargetLocator, DataRoot: receipt.Locator, DataReceiptID: binding.DataReceiptID, TreeDigest: recordedBindingDigest(binding, installation.Source.TreeDigest), SelectedDelivery: binding.SelectedDelivery},
		Intent:  intent, NativeProfileRoot: binding.NativeProfileRoot, Digest: fmt.Sprintf("sha256:%x", sum),
	}, nil
}

func nativeObservationIdentity(pending PendingNativeIntent) string {
	b := pending.Binding
	return "native|" + b.InstallationID + "|" + b.BindingID + "|" + pending.Intent.AttemptID + "|" + string(pending.Intent.Direction) + "|" + pending.Digest
}

// The facade holds the process lock throughout observation validation, journal
// recovery and all selected reconciliations. Service retains its existing
// beginMutation validation/recovery, borrowing this acquisition, never nesting.
type retainedMutationLock struct{ validate func() error }

func (l retainedMutationLock) Acquire(ctx context.Context) (ports.UnlockFunc, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := l.validate(); err != nil {
		return nil, err
	}
	return func() error { return nil }, nil
}

func (e *Engine) recoverNativeIntents(ctx context.Context, svc usecase.Service, observed RecoveryObservation) error {
	// Resolve every selected capability before journal or native effects.
	reconcilers := make([]usecase.NativeIntentReconciler, len(observed.NativeIntents))
	for i, pending := range observed.NativeIntents {
		r, err := e.selectedReconciler(pending.Binding.ClientID, pending.Intent.Delivery)
		if err != nil {
			return err
		}
		reconcilers[i] = r
	}
	scope := &nativeRecoveryScope{engine: e, expected: observed}
	// OpenCode's durable recorder publishes its binding itself. Directory and
	// selected-intent recovery must retain those journals until its own phase.
	nativeTransitions := svc.Kernel.NativeRecovery
	svc.Kernel.NativeRecovery = nil
	if err := scope.RequireMutationReady(); err != nil {
		return err
	}
	// Fence the first kernel as well as every later acknowledgement.
	svc.StateStore, svc.Kernel.StateStore = scope, scope
	if err := e.recoverObservedJournals(ctx, svc, scope); err != nil {
		return err
	}
	// The existing service borrows the retained acquisition. Its first state Load
	// fences the kernel, and Save fences acknowledgement and records only that
	// successful service transition. No callback may broaden the expected scope.
	svc.Lock = retainedMutationLock{validate: scope.confirm}
	pending := scope.expected.NativeIntents
	for i, item := range pending {
		if err := scope.confirm(); err != nil {
			return err
		}
		b := item.Binding
		if err := svc.RecoverNativeIntent(ctx, b.InstallationID, b.BindingID, item.Intent.AttemptID, scopedNativeReconciler{scope: scope, inner: reconcilers[i], pending: item}); err != nil {
			return errors.Join(err, scope.confirm())
		}
		if err := scope.confirm(); err != nil {
			return err
		}
		if len(scope.expected.NativeIntents) != len(pending)-i-1 || (len(scope.expected.NativeIntents) != 0 && !reflect.DeepEqual(scope.expected.NativeIntents, pending[i+1:])) {
			return ErrPlanChanged
		}
	}
	return e.recoverObservedTransitions(ctx, nativeTransitions, scope.expected)
}

// Only existing state_committed receipts can advance during successful kernel
// recovery. Compare the full resulting state; unrelated bindings and receipts
// cannot be adopted as a new baseline. No native callback runs in this phase.
func (e *Engine) recoverObservedJournals(ctx context.Context, svc usecase.Service, scope *nativeRecoveryScope) error {
	state, err := scope.Load()
	if err != nil {
		return err
	}
	if err := scope.confirm(); err != nil {
		return err
	}
	directoryObservation := scope.expected
	directoryObservation.Journals = nil
	open, err := svc.Kernel.Directory.ListOpen()
	if err != nil {
		return err
	}
	for _, receipt := range open {
		for _, journal := range scope.expected.Journals {
			if receipt.OperationID == journal.OperationID {
				directoryObservation.Journals = append(directoryObservation.Journals, journal)
			}
		}
	}
	scope.journals = newJournalRecoveryFence(state, directoryObservation)
	if err := svc.Kernel.Recover(journalRecoveryContext{Context: ctx, scope: scope}); err != nil {
		return errors.Join(err, scope.confirm())
	}
	if err := scope.advanceJournals(); err != nil {
		return err
	}
	scope.journals = nil
	recoverStateReceipts(&state)
	digest, err := recoveryStateDigest(state)
	if err != nil {
		return err
	}
	after, err := e.observe()
	if err != nil {
		return err
	}
	if after.Recovery.StateDigest != digest || !reflect.DeepEqual(after.Recovery.Journals, scope.expected.Journals) {
		return ErrPlanChanged
	}
	for _, receipt := range after.Recovery.Receipts {
		if receipt.Phase != "native_attempt" || !containsReceipt(scope.expected.Receipts, receipt) {
			return ErrRecoveryRequired
		}
	}
	scope.expected = after.Recovery
	return nil
}

func recoverStateReceipts(state *domain.StateFileV2) {
	for i := range state.TransactionReceipts {
		recoveredReceipt(&state.TransactionReceipts[i])
	}
	for _, installation := range state.Installations {
		for key, binding := range installation.Clients {
			for i := range binding.Receipts {
				recoveredReceipt(&binding.Receipts[i])
			}
			installation.Clients[key] = binding
		}
	}
}

func recoveredReceipt(receipt *domain.MutationReceipt) {
	if receipt.Phase == transaction.ReceiptPhaseStateCommitted {
		receipt.Phase = transaction.ReceiptPhaseCommitted
	}
}
