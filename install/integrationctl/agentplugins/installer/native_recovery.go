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

func inspectNativeIntents(state domain.StateFileV2) ([]PendingNativeIntent, error) {
	var out []PendingNativeIntent
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if binding.PendingNativeIntent == nil {
				if binding.NativeActivationAttempt != "" {
					return out, fmt.Errorf("native attempt has no persisted intent; retain uncertainty")
				}
				continue
			}
			if err := validateSelectedBindingIdentity(binding); err != nil {
				return out, err
			}
			intent := *binding.PendingNativeIntent
			if err := intent.Validate(binding); err != nil {
				return out, err
			}
			if !reflect.DeepEqual(intent.Delivery, binding.SelectedDelivery) {
				return out, fmt.Errorf("native intent differs from complete binding selection")
			}
			body, err := json.Marshal(binding)
			if err != nil {
				return out, err
			}
			sum := sha256.Sum256(body)
			receipt := installation.DataReceipts[binding.DataReceiptID]
			out = append(out, PendingNativeIntent{
				Binding: BindingFacts{InstallationID: installation.InstallationID, ClientID: binding.ClientID, BindingID: binding.ClientBindingID, Scope: binding.Scope, TargetPath: binding.TargetLocator, DataRoot: receipt.Locator, DataReceiptID: binding.DataReceiptID, TreeDigest: recordedBindingDigest(binding, installation.Source.TreeDigest), SelectedDelivery: binding.SelectedDelivery},
				Intent:  intent, NativeProfileRoot: binding.NativeProfileRoot, Digest: fmt.Sprintf("sha256:%x", sum),
			})
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
	return nil
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
	scope.journals = newJournalRecoveryFence(state, scope.expected)
	if err := svc.Kernel.Recover(journalRecoveryContext{Context: ctx, scope: scope}); err != nil {
		return errors.Join(err, scope.confirm())
	}
	if err := scope.advanceJournals(); err != nil {
		return err
	}
	scope.journals = nil
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
	digest, err := recoveryStateDigest(state)
	if err != nil {
		return err
	}
	after, err := e.observe()
	if err != nil {
		return err
	}
	if after.Recovery.StateDigest != digest || len(after.Recovery.Journals) != 0 {
		return ErrPlanChanged
	}
	if len(after.Recovery.Receipts) != 0 {
		return ErrRecoveryRequired
	}
	scope.expected = after.Recovery
	return nil
}

func recoveredReceipt(receipt *domain.MutationReceipt) {
	if receipt.Phase == transaction.ReceiptPhaseStateCommitted {
		receipt.Phase = transaction.ReceiptPhaseCommitted
	}
}

// A guard around the existing store, not an alternate recovery/state engine.
// The facade retains the actual lock. Each kernel read and acknowledgement
// write must still see the complete last approved or recorded-successful scope.
type nativeRecoveryScope struct {
	engine   *Engine
	expected RecoveryObservation
	journals *journalRecoveryFence
}

func (s *nativeRecoveryScope) confirm() error {
	view, err := s.engine.observe()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPlanChanged, err)
	}
	if !reflect.DeepEqual(view.Recovery, s.expected) {
		return ErrPlanChanged
	}
	return nil
}
func (s *nativeRecoveryScope) RequireMutationReady() error {
	if guard, ok := s.engine.store.(interface{ RequireMutationReady() error }); ok {
		if err := guard.RequireMutationReady(); err != nil {
			return err
		}
	}
	return s.confirm()
}
func (s *nativeRecoveryScope) Load() (domain.StateFileV2, error) {
	if err := s.confirm(); err != nil {
		return domain.StateFileV2{}, err
	}
	state, err := s.engine.store.Load()
	if err != nil {
		return state, err
	}
	digest, err := recoveryStateDigest(state)
	if err != nil {
		return domain.StateFileV2{}, err
	}
	if digest != s.expected.StateDigest {
		return domain.StateFileV2{}, ErrPlanChanged
	}
	if err := s.confirm(); err != nil {
		return domain.StateFileV2{}, err
	}
	return state, nil
}
func (s *nativeRecoveryScope) Save(state domain.StateFileV2) error {
	if err := s.advanceJournals(); err != nil {
		return err
	}
	if err := s.confirm(); err != nil {
		return err
	}
	if s.journals != nil {
		if err := s.journals.validateState(state); err != nil {
			return err
		}
	}
	digest, err := recoveryStateDigest(state)
	if err != nil {
		return err
	}
	if err := s.engine.store.Save(state); err != nil {
		return err
	}
	after, err := s.engine.observe()
	if err != nil {
		return err
	}
	if after.Recovery.StateDigest != digest || !reflect.DeepEqual(after.Recovery.Journals, s.expected.Journals) {
		return ErrPlanChanged
	}
	s.expected = after.Recovery
	return nil
}

// MarshalJSON renders the existing domain intent for diagnostics. Unmarshal
// does not restore Intent or Binding.SelectedDelivery (both json:"-"); a JSON
// round trip cannot create the typed observation accepted by Recover.
func (pending PendingNativeIntent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Binding           BindingFacts
		NativeProfileRoot string
		Digest            string
		Intent            domain.PendingNativeIntent
	}{pending.Binding, pending.NativeProfileRoot, pending.Digest, pending.Intent})
}

// Fence the reconciler's return before Service acknowledges ownership. A late
// foreign binding edit must retain its attempt, even after a native read/write.
type scopedNativeReconciler struct {
	scope   *nativeRecoveryScope
	inner   usecase.NativeIntentReconciler
	pending PendingNativeIntent
}

func (r scopedNativeReconciler) ReconcileNativeIntent(ctx context.Context, intent domain.PendingNativeIntent) (domain.ActivationOutcome, error) {
	if !reflect.DeepEqual(intent, r.pending.Intent) {
		return domain.ActivationOutcome{}, ErrPlanChanged
	}
	if err := r.confirm(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome, err := r.inner.ReconcileNativeIntent(ctx, intent)
	if scopeErr := r.confirm(); scopeErr != nil {
		return outcome, errors.Join(scopeErr, err)
	}
	return outcome, err
}
func (r scopedNativeReconciler) confirm() error { return r.scope.confirm() }
