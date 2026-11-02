package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// Inspect is read-only. It does not recover journals or invoke host seams.
func (e *Engine) Inspect(ctx context.Context) (Inspection, error) {
	out, err := e.observe()
	if err != nil {
		return out, err
	}
	stager := e.lifecycle(nil, BindingFacts{}, nil).Stager
	for i := range out.Installations {
		for j := range out.Installations[i].Bindings {
			b := &out.Installations[i].Bindings[j]
			if b.ProjectionDigest == "" {
				b.ProjectionStatus = "projection_missing"
				continue
			}
			if _, err := os.Stat(b.TargetPath); os.IsNotExist(err) {
				b.ProjectionStatus = "projection_missing"
			} else if err := stager.Verify(ctx, b.TargetPath, b.ProjectionDigest); err != nil {
				b.ProjectionStatus = "projection_mismatch"
			}
		}
	}
	return out, nil
}

func (e *Engine) observe() (Inspection, error) {
	out := Inspection{StateRoot: e.cfg.StateRoot}
	if err := e.CheckProjectNamespace(); err != nil {
		return out, err
	}
	state, err := e.store.Load()
	if err != nil {
		out.Recovery.Required = true
		out.Recovery.Reason = err.Error()
		return out, err
	}
	out.Recovery.StateDigest, err = recoveryStateDigest(state)
	if err != nil {
		return out, err
	}
	installations, bindingIndex := inspectInstallations(state)
	out.Installations = installations
	out.EnvironmentReceipts = append([]domain.EnvironmentReceipt(nil), state.EnvironmentReceipts...)
	out.Recovery.NativeIntents, err = inspectNativeIntents(state)
	if err != nil {
		out.Recovery.Required = true
		out.Recovery.Reason = err.Error()
		return out, err
	}
	open, err := dirswap.Manager{Namespace: e.cfg.StateRoot, JournalDir: e.cfg.OperationsDir}.ListOpen()
	if err != nil {
		out.Recovery.Required = true
		out.Recovery.Reason = err.Error()
		return out, err
	}
	openIDs := make(map[string]dirswap.Receipt, len(open))
	for _, journal := range open {
		openIDs[journal.OperationID] = journal
		located := bindingIndex[journal.ClientBindingID]
		out.Recovery.Journals = append(out.Recovery.Journals, PendingJournal{
			EnvironmentID:  journal.EnvironmentID,
			OperationID:    journal.OperationID,
			Digest:         journalDigest(journal),
			BindingID:      journal.ClientBindingID,
			InstallationID: located.InstallationID,
			TargetPath:     firstNonEmpty(journal.ActivePath, located.TargetPath),
			Phase:          journal.Phase,
		})
	}
	out.Recovery.Receipts = unfinishedReceipts(state, openIDs, bindingIndex)
	if err := e.appendPendingTransitions(&out); err != nil {
		out.Recovery.Required = true
		out.Recovery.Reason = err.Error()
		return out, err
	}
	sort.Slice(out.Recovery.Journals, func(i, j int) bool {
		return out.Recovery.Journals[i].OperationID < out.Recovery.Journals[j].OperationID
	})
	out.Recovery.Required = len(out.Recovery.Journals) > 0 || len(out.Recovery.Receipts) > 0 || len(out.Recovery.NativeIntents) > 0
	return out, nil
}

func inspectInstallations(state domain.StateFileV2) ([]InspectedInstallation, map[string]observedBinding) {
	installations := make([]InspectedInstallation, 0, len(state.Installations))
	bindingIndex := map[string]observedBinding{}
	for _, installation := range state.Installations {
		item := InspectedInstallation{
			InstallationID: installation.InstallationID,
			DataRetained:   installation.DataRetained,
			TreeDigest:     installation.Source.TreeDigest,
		}
		item.SourceKey = "github:" + strings.ToLower(installation.Source.Repository) + ":" + installation.Source.PackageSubpath
		if installation.Source.StableSourceKey != "" {
			item.SourceKey = installation.Source.StableSourceKey
		} else if installation.Source.SourceKind == "git" || installation.Source.SourceKind == "archive" {
			item.SourceKey = sourcespec.Key(sourcespec.Kind(installation.Source.SourceKind), installation.Source.SourceURL, installation.Source.PackageSubpath)
		}
		if installation.Directory != nil {
			item.SourceKey = "directory:" + strings.ToLower(installation.Directory.DistributionID)
		}
		for _, binding := range installation.Clients {
			receipt := installation.DataReceipts[binding.DataReceiptID]
			digest := recordedBindingDigest(binding, installation.Source.TreeDigest)
			item.Bindings = append(item.Bindings, InspectedBinding{
				LifecycleConverged: binding.LifecycleConverged(), ClientID: binding.ClientID, BindingID: binding.ClientBindingID, Scope: binding.Scope,
				TargetPath: binding.TargetLocator, DataRoot: receipt.Locator,
				TreeDigest: digest, Materialization: string(binding.Materialization), Activation: string(binding.Activation),
				Policy: string(binding.Policy), ManagedPaths: inspectedManagedPaths(binding),
				Authentication: string(binding.Authentication), Verification: string(binding.Verification),
			})
			inspected := &item.Bindings[len(item.Bindings)-1]
			populateInspectedBinding(inspected, binding, state.EnvironmentReceipts, installation.InstallationID)
			bindingIndex[binding.ClientBindingID] = observedBinding{
				InstallationID: installation.InstallationID,
				TargetPath:     binding.TargetLocator,
			}
		}
		if installation.DataRetained {
			for _, receipt := range installation.DataReceipts {
				if receipt.Locator != "" {
					item.DataRoots = append(item.DataRoots, receipt.Locator)
				}
			}
			sort.Strings(item.DataRoots)
		}
		installations = append(installations, item)
	}
	return installations, bindingIndex
}

type observedBinding struct {
	InstallationID, TargetPath string
}

func unfinishedReceipts(state domain.StateFileV2, open map[string]dirswap.Receipt, bindings map[string]observedBinding) []PendingReceipt {
	var out []PendingReceipt
	add := func(receipt domain.MutationReceipt, installationID, targetPath string) {
		if receipt.Phase == "" || receipt.Phase == transaction.ReceiptPhaseCommitted {
			return
		}
		located := bindings[receipt.ClientBindingID]
		if installationID == "" {
			installationID = located.InstallationID
		}
		if targetPath == "" {
			targetPath = firstNonEmpty(receipt.ActivePath, located.TargetPath)
		}
		_, present := open[receipt.OperationID]
		out = append(out, PendingReceipt{
			OperationID: receipt.OperationID, BindingID: receipt.ClientBindingID,
			InstallationID: installationID, TargetPath: targetPath, Phase: receipt.Phase,
			JournalPresent: present,
		})
	}
	for _, receipt := range state.TransactionReceipts {
		add(receipt, "", "")
	}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			for _, receipt := range binding.Receipts {
				add(receipt, installation.InstallationID, binding.TargetLocator)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OperationID != out[j].OperationID {
			return out[i].OperationID < out[j].OperationID
		}
		return out[i].Phase < out[j].Phase
	})
	return out
}

func journalDigest(receipt dirswap.Receipt) string {
	// Receipt is a concrete JSON value with no fallible MarshalJSON method.
	raw, _ := json.Marshal(receipt)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func recoveryStateDigest(state domain.StateFileV2) (string, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func pendingIdentity(obs RecoveryObservation) []string {
	keys := make([]string, 0, len(obs.Journals)+len(obs.Receipts))
	for _, journal := range obs.Journals {
		keys = append(keys, "journal|"+journal.OperationID+"|"+journal.Phase+"|"+journal.Digest)
	}
	for _, receipt := range obs.Receipts {
		keys = append(keys, "receipt|"+receipt.OperationID+"|"+receipt.Phase+"|"+receipt.BindingID)
	}
	for _, pending := range obs.NativeIntents {
		keys = append(keys, nativeObservationIdentity(pending))
	}
	sort.Strings(keys)
	return keys
}

func leftoverSwapArtifacts(obs RecoveryObservation) bool {
	for _, journal := range obs.Journals {
		if journal.TargetPath == "" {
			return true
		}
		entries, err := os.ReadDir(filepath.Dir(journal.TargetPath))
		if err != nil {
			return true
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".agentplugins-staging-") || strings.HasPrefix(name, ".agentplugins-backup-") {
				return true
			}
		}
	}
	return false
}

func liveWithinObserved(live, observed RecoveryObservation) bool {
	if len(live.NativeIntents) != 0 || len(observed.NativeIntents) != 0 {
		return reflect.DeepEqual(live, observed)
	}
	// Historical journal recovery permits work already recovered by another
	// cooperating process, but every remaining scope must still match exactly.
	if live.Required && live.StateDigest != observed.StateDigest {
		return false
	}
	for _, journal := range live.Journals {
		if !containsJournal(observed.Journals, journal) {
			return false
		}
	}
	for _, receipt := range live.Receipts {
		if !containsReceipt(observed.Receipts, receipt) {
			return false
		}
	}
	return true
}
func containsJournal(items []PendingJournal, want PendingJournal) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
func containsReceipt(items []PendingReceipt, want PendingReceipt) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

// Recover finishes already recorded UAP transactions for the observed scope.
// It reconciles only persisted selected native intents, without rediscovery. A new pending
// operation that was not in observed returns ErrPlanChanged without recovery.
func (e *Engine) Recover(ctx context.Context, observed Inspection) (Result, error) {
	return e.recover(ctx, observed, "")
}

// RecoverEnvironment admits only a complete pending scope owned by this environment.
func (e *Engine) RecoverEnvironment(ctx context.Context, observed Inspection, environmentID string) (Result, error) {
	if environmentID == "" {
		return Result{}, ErrInvalidRequest
	}
	return e.recover(ctx, observed, environmentID)
}

func (e *Engine) recover(ctx context.Context, observed Inspection, environmentID string) (Result, error) {
	if ctx == nil {
		return Result{Outcome: OutcomeIncomplete, Reason: "context is required"}, fmt.Errorf("%w: context is required", ErrInvalidRequest)
	}
	if observed.StateRoot != e.cfg.StateRoot {
		return Result{Outcome: OutcomeConflict, Reason: "plan_changed"}, ErrPlanChanged
	}
	initial, err := e.observe()
	if err != nil {
		return Result{Outcome: OutcomeRecovery, Reason: err.Error(), Recovery: classifyRecovery(observed.Recovery, initial.Recovery, false, err)}, fmt.Errorf("%w: %w", ErrRecoveryRequired, err)
	}
	if !liveWithinObserved(initial.Recovery, observed.Recovery) {
		return Result{Outcome: OutcomeConflict, Reason: "plan_changed"}, ErrPlanChanged
	}
	if err := e.checkEnvironmentRecovery(initial, environmentID); err != nil {
		return Result{Outcome: OutcomeRecovery, Reason: err.Error()}, err
	}
	if result, err := e.prepareRecovery(ctx); err != nil {
		return result, err
	}
	svc := e.lifecycle(nil, BindingFacts{}, nil)
	release, err := svc.Lock.Acquire(ctx)
	if err != nil {
		return Result{Outcome: OutcomeIncomplete, Reason: err.Error()}, err
	}
	defer func() { _ = release() }()
	if observed.StateRoot != "" && observed.StateRoot != e.cfg.StateRoot {
		return Result{Outcome: OutcomeConflict, Reason: "plan_changed"}, ErrPlanChanged
	}
	live, err := e.observe()
	if err != nil {
		result := Result{Outcome: OutcomeRecovery, Reason: live.Recovery.Reason, Recovery: classifyRecovery(observed.Recovery, live.Recovery, false, err)}
		if result.Reason == "" {
			result.Reason = err.Error()
		}
		return result, fmt.Errorf("%w: %w", ErrRecoveryRequired, err)
	}
	if live.Recovery.Reason != "" {
		return Result{Outcome: OutcomeRecovery, Reason: live.Recovery.Reason, Recovery: classifyRecovery(observed.Recovery, live.Recovery, true, ErrRecoveryRequired)}, ErrRecoveryRequired
	}
	if !liveWithinObserved(live.Recovery, observed.Recovery) {
		return Result{Outcome: OutcomeConflict, Reason: "plan_changed", Recovery: classifyRecovery(observed.Recovery, live.Recovery, true, ErrPlanChanged)}, ErrPlanChanged
	}
	if err := e.checkEnvironmentRecovery(live, environmentID); err != nil {
		return Result{Outcome: OutcomeRecovery, Reason: err.Error()}, err
	}
	return e.applyObservedRecovery(ctx, svc, observed, live)
}

func (e *Engine) finishRecovery(observed Inspection) (Result, error) {
	after, afterErr := e.observe()
	if afterErr != nil || after.Recovery.Required {
		reason := after.Recovery.Reason
		if afterErr != nil && reason == "" {
			reason = afterErr.Error()
		}
		if reason == "" {
			reason = "pending transactions remain"
		}
		return Result{Outcome: OutcomeRecovery, Reason: reason, Recovery: classifyRecovery(observed.Recovery, after.Recovery, afterErr == nil, fmt.Errorf("%s", reason))}, fmt.Errorf("%w: %s", ErrRecoveryRequired, reason)
	}
	return Result{Outcome: OutcomeCompleted, Recovery: classifyRecovery(observed.Recovery, after.Recovery, true, nil)}, nil
}

// RecoverCurrent inspects this root and recovers that exact live scope.
func (e *Engine) RecoverCurrent(ctx context.Context) (Result, error) {
	view, err := e.Inspect(ctx)
	if err != nil {
		reason := view.Recovery.Reason
		if reason == "" {
			reason = err.Error()
		}
		return Result{Outcome: OutcomeRecovery, Reason: reason, Recovery: classifyRecovery(view.Recovery, view.Recovery, false, err)}, err
	}
	return e.Recover(ctx, view)
}

func classifyRecovery(before, after RecoveryObservation, observedAfter bool, recoverErr error) RecoveryReport {
	afterKeys := map[string]bool{}
	if observedAfter {
		for _, key := range pendingIdentity(after) {
			afterKeys[key] = true
		}
	}
	report := RecoveryReport{}
	place := func(item PendingReceipt, key string) {
		switch {
		case !observedAfter:
			report.Unknown = append(report.Unknown, item)
		case afterKeys[key]:
			report.Remaining = append(report.Remaining, item)
		case recoverErr != nil:
			report.Unknown = append(report.Unknown, item)
		default:
			report.Resolved = append(report.Resolved, item)
		}
	}
	for _, journal := range before.Journals {
		place(PendingReceipt{
			OperationID: journal.OperationID, BindingID: journal.BindingID,
			InstallationID: journal.InstallationID, TargetPath: journal.TargetPath,
			Phase: journal.Phase, JournalPresent: true,
		}, "journal|"+journal.OperationID+"|"+journal.Phase+"|"+journal.Digest)
	}
	for _, receipt := range before.Receipts {
		place(receipt, "receipt|"+receipt.OperationID+"|"+receipt.Phase+"|"+receipt.BindingID)
	}
	for _, pending := range before.NativeIntents {
		b := pending.Binding
		place(PendingReceipt{OperationID: pending.Intent.AttemptID, InstallationID: b.InstallationID, BindingID: b.BindingID, TargetPath: b.TargetPath, Phase: "native_" + string(pending.Intent.Direction)}, nativeObservationIdentity(pending))
	}
	return report
}

func recordedBindingDigest(binding domain.ClientBinding, fallback string) string {
	if binding.PackageRevision != nil && binding.PackageRevision.TreeDigest != "" {
		return binding.PackageRevision.TreeDigest
	}
	return fallback
}

func planClientDigest(plan Plan, clientID string) string {
	for _, target := range plan.Targets {
		if target.ClientID == clientID && target.TreeDigest != "" {
			return target.TreeDigest
		}
	}
	return plan.TreeDigest
}

func liveClientResult(binding domain.ClientBinding, required []string, fallbackDigest string) ClientResult {
	return ClientResult{ProfileAuthority: domain.CloneProfileAuthority(binding.ProfileAuthority),
		SelectedDelivery:   binding.SelectedDelivery,
		ClientID:           binding.ClientID,
		BindingID:          binding.ClientBindingID,
		TreeDigest:         recordedBindingDigest(binding, fallbackDigest),
		Materialization:    string(binding.Materialization),
		Activation:         string(binding.Activation),
		Authentication:     string(binding.Authentication),
		Policy:             string(binding.Policy),
		Verification:       string(binding.Verification),
		RequiredComponents: append([]string(nil), required...),
	}
}

func (e *Engine) prepareRecovery(ctx context.Context) (Result, error) {
	if err := e.lifecycle(nil, BindingFacts{}, nil).Kernel.PrevalidateRecovery(ctx); err != nil {
		return Result{Outcome: OutcomeRecovery, Reason: err.Error()}, err
	}
	if err := e.ensureDirs(); err != nil {
		return Result{Outcome: OutcomeIncomplete, Reason: err.Error()}, err
	}
	return Result{}, nil
}

func (e *Engine) checkEnvironmentRecovery(observed Inspection, environmentID string) error {
	state, err := e.store.Load()
	if err != nil {
		return err
	}
	var authority *domain.EnvironmentAuthority
	if environmentID != "" {
		authority = &domain.EnvironmentAuthority{EnvironmentID: environmentID}
	}
	if err := transaction.ValidateEnvironmentRecoveryScope(state, authority); err != nil {
		return fmt.Errorf("%w: %w", ErrRecoveryRequired, err)
	}
	for _, j := range observed.Recovery.Journals {
		owner := j.EnvironmentID
		if owner != environmentID {
			return fmt.Errorf("%w: foreign or indeterminate journal %s", ErrRecoveryRequired, j.OperationID)
		}
	}
	return nil
}

func (e *Engine) applyObservedRecovery(ctx context.Context, svc usecase.Service, observed, live Inspection) (Result, error) {
	if !live.Recovery.Required {
		if observed.Recovery.Required && leftoverSwapArtifacts(observed.Recovery) {
			return Result{
				Outcome: OutcomeRecovery, Reason: "incomplete_recovery",
				Recovery: classifyRecovery(observed.Recovery, live.Recovery, true, ErrRecoveryRequired),
			}, fmt.Errorf("%w: journal vanished with leftover swap artifacts", ErrRecoveryRequired)
		}
		return Result{Outcome: OutcomeUnchanged, Reason: "already_recovered", Recovery: classifyRecovery(observed.Recovery, live.Recovery, true, nil)}, nil
	}
	if err := e.recoverNativeIntents(ctx, svc, live.Recovery); err != nil {
		after, afterErr := e.observe()
		result := Result{Outcome: OutcomeRecovery, Reason: err.Error(), Recovery: classifyRecovery(observed.Recovery, after.Recovery, afterErr == nil, err)}
		if errors.Is(err, ErrPlanChanged) {
			result.Outcome, result.Reason = OutcomeConflict, "plan_changed"
		}
		return result, fmt.Errorf("%w: %w", ErrRecoveryRequired, err)
	}
	return e.finishRecovery(observed)
}

func populateInspectedBinding(inspected *InspectedBinding, binding domain.ClientBinding, owners []domain.EnvironmentReceipt, installationID string) {
	if binding.PackageRevision != nil {
		inspected.Revision = binding.PackageRevision.ResolvedRevision
	}
	for _, object := range binding.NativeObjects {
		if object.Kind == "managed_package_directory" {
			inspected.ProjectionDigest = object.ManagedDigest
		}
	}
	for _, owner := range owners {
		if owner.InstallationID == installationID && owner.ClientBindingID == binding.ClientBindingID {
			cloned := owner
			inspected.Owner = &cloned
		}
	}
}

func (e *Engine) appendPendingTransitions(out *Inspection) error {
	pending, err := (opencode.NativeTransitions{ProjectRoot: e.cfg.ProjectRoot, State: transaction.Kernel{StateStore: e.store}}).Pending()
	if err != nil {
		return err
	}
	for _, p := range pending {
		if p.Root == "" {
			out.Recovery.Receipts = append(out.Recovery.Receipts, PendingReceipt{OperationID: p.OperationID, InstallationID: p.InstallationID, BindingID: p.BindingID, TargetPath: p.TargetPath, Phase: p.Phase})
		} else {
			out.Recovery.Journals = append(out.Recovery.Journals, PendingJournal{OperationID: p.OperationID, InstallationID: p.InstallationID, BindingID: p.BindingID, TargetPath: p.TargetPath, Phase: p.Phase, Digest: p.Digest})
		}
	}
	return nil
}
