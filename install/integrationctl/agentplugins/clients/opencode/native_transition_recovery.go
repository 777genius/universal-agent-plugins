package opencode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type PendingNativeTransition struct {
	OperationID, InstallationID, BindingID, TargetPath, Phase, Digest, Root string
	// AuthorityDigest remains stable through the recorder's phase advances.
	AuthorityDigest string
}

func (t NativeTransitions) Pending() ([]PendingNativeTransition, error) {
	state, err := t.State.Load()
	if err != nil {
		return nil, err
	}
	roots, attempts := pendingTransitionAuthority(state)
	var pending []PendingNativeTransition
	for root := range roots {
		if err := appendPendingTransitionRoot(root, attempts, &pending); err != nil {
			return nil, err
		}
	}
	for _, attempt := range attempts {
		pending = append(pending, attempt)
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].OperationID < pending[j].OperationID })
	for i := 1; i < len(pending); i++ {
		if pending[i].OperationID == pending[i-1].OperationID {
			return nil, fmt.Errorf("duplicate native transition identity")
		}
	}
	return pending, nil
}

func pendingTransitionAuthority(state domain.StateFileV2) (map[string]bool, map[string]PendingNativeTransition) {
	roots := map[string]bool{}
	attempts := map[string]PendingNativeTransition{}
	for _, installation := range state.Installations {
		for _, b := range installation.Clients {
			if b.ClientID != "opencode" {
				continue
			}
			if root := storedTransitionRoot(b); root != "" {
				roots[root] = true
			}
			if b.NativeActivationAttempt != "" {
				attempts[b.NativeActivationAttempt] = PendingNativeTransition{OperationID: b.NativeActivationAttempt, InstallationID: installation.InstallationID, BindingID: b.ClientBindingID, TargetPath: b.TargetLocator, Phase: "native_attempt"}
			}
		}
	}
	return roots, attempts
}

func appendPendingTransitionRoot(root string, attempts map[string]PendingNativeTransition, pending *[]PendingNativeTransition) error {
	skills := filepath.Join(root, "skills")
	if err := pathpolicy.RequireContainedChild(root, skills); err != nil {
		return err
	}
	entries, err := os.ReadDir(skills)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".agentplugins-native-") {
			continue
		}
		item, err := pendingTransitionEntry(root, skills, entry.Name())
		if err != nil {
			return err
		}
		if item == nil {
			continue
		}
		*pending = append(*pending, *item)
		delete(attempts, item.OperationID)
		if len(*pending) > nativeconfig.MaxTransitionEntries {
			return fmt.Errorf("too many native transition records")
		}
	}
	return nil
}

func pendingTransitionEntry(root, skills, name string) (*PendingNativeTransition, error) {
	dir := filepath.Join(skills, name)
	if err := pathpolicy.RequireContainedChild(root, dir); err != nil {
		return nil, err
	}
	if _, err := os.Lstat(filepath.Join(dir, transitionRecordFile)); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	r, err := readTransitionRecord(dir)
	if err != nil {
		return nil, err
	}
	if r.Identity.NativeRoot != root {
		return nil, fmt.Errorf("transition root identity mismatch")
	}
	_, b, err := transitionBinding(r.OldState, r.Identity)
	if err != nil {
		return nil, err
	}
	return &PendingNativeTransition{OperationID: r.Identity.OperationID, InstallationID: r.Identity.InstallationID, BindingID: r.Identity.BindingID, TargetPath: b.TargetLocator, Phase: r.Phase, Digest: r.Hash, Root: dir, AuthorityDigest: transitionAuthorityHash(r)}, nil

}

func (t NativeTransitions) Recover(ctx context.Context) error {
	pending, err := t.Pending()
	if err != nil {
		return err
	}
	for _, p := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}
		if p.Root == "" {
			return fmt.Errorf("native attempt has no durable transition record; recovery required")
		}
		if _, err := t.Reconcile(ctx, p.Root); err != nil {
			return err
		}
	}
	return nil
}

type transitionReconciler struct {
	transitions NativeTransitions
	ctx         context.Context
	record      transitionRecord
	effect      domain.NativeEffectState
}

func (t NativeTransitions) Reconcile(ctx context.Context, root string) (domain.NativeEffectState, error) {
	r, err := readTransitionRecord(root)
	if err != nil {
		return domain.NativeEffectUncertain, err
	}
	q := transitionReconciler{transitions: t, ctx: ctx, record: r, effect: domain.NativeEffectUncertain}
	observation, reconcileErr := t.Kernel.ReconcileDialectTransition(nativeconfig.TransitionRecoveryRequest{
		Paths: transitionPaths(r.Identity.NativeRoot), Prepared: nativePrepared(r.Native),
		Decide: q.decide, Complete: q.complete,
	})
	if observation == nativeconfig.TransitionTarget && q.effect == domain.NativeEffectCommitted && reconcileErr != nil && !nativeconfig.IsCommittedCleanup(reconcileErr) {
		reconcileErr = &nativeconfig.CommittedCleanupError{Err: reconcileErr}
	}
	return q.effect, reconcileErr
}

func (q *transitionReconciler) decide(observation nativeconfig.TransitionObservation) (nativeconfig.TransitionStateDecision, error) {
	if err := q.ctx.Err(); err != nil {
		return nativeconfig.TransitionStateUnknown, err
	}
	current, err := q.transitions.State.Load()
	if err != nil {
		return nativeconfig.TransitionStateUnknown, err
	}
	source := transitionStateMatches(current, q.record.OldState, q.record.Identity) || transitionStateMatches(current, sourceTransitionState(q.record), q.record.Identity)
	target := transitionStateMatches(current, q.record.TargetState, q.record.Identity)
	if !source && !target {
		return nativeconfig.TransitionStateUnknown, fmt.Errorf("transition state differs from bound source and target")
	}
	_, binding, _ := transitionBinding(q.record.TargetState, q.record.Identity)
	if q.transitions.PackageVerifier == nil {
		return nativeconfig.TransitionStateUnknown, fmt.Errorf("native package verifier missing")
	}
	if err := q.transitions.PackageVerifier.Verify(q.ctx, binding.TargetLocator, shared.ManagedPackageDigest(binding)); err != nil {
		return nativeconfig.TransitionStateUnknown, err
	}
	return q.decideObservedTransition(observation, current, source, target)

}

func (q *transitionReconciler) decideObservedTransition(observation nativeconfig.TransitionObservation, current domain.StateFileV2, source, target bool) (nativeconfig.TransitionStateDecision, error) {
	_, binding, _ := transitionBinding(q.record.TargetState, q.record.Identity)
	if observation == nativeconfig.TransitionSource {
		if !source {
			return nativeconfig.TransitionStateUnknown, fmt.Errorf("source bytes with target state")
		}
		return nativeconfig.TransitionStateOld, nil
	}
	// Prove projection and all desired skill effects before publishing receipts.

	projection, err := readTransitionFile(binding.TargetLocator, filepath.Join(binding.TargetLocator, OpenCodeProjectionFile), nativeconfig.MaxTransitionConfigBytes)
	if err != nil || transitionHash(projection) != q.record.ProjectionHash {
		return nativeconfig.TransitionStateUnknown, fmt.Errorf("transition package projection changed")
	}
	if err := verifyTransitionTargetSkills(q.record); err != nil {
		return nativeconfig.TransitionStateUnknown, err
	}
	q.record.Phase = "native_committed"
	if err := writeTransitionRecord(q.record); err != nil {
		return nativeconfig.TransitionStateUnknown, err
	}
	desired, err := transitionPublicationState(current, q.record.TargetState, q.record.Identity)
	if err != nil {
		return nativeconfig.TransitionStateUnknown, err
	}
	disposition, err := q.transitions.State.PersistStateDecisionWithDisposition(current, desired)
	switch disposition {
	case domain.StateDecisionDesired:
		q.effect = domain.NativeEffectCommitted
		return nativeconfig.TransitionStateDesired, err
	case domain.StateDecisionOld:
		// Old only means the exact caller preimage remains. If that preimage was
		// already target state, it grants no source restore authority.
		if target {
			return nativeconfig.TransitionStateUnknown, err
		}
		return nativeconfig.TransitionStateOld, err
	default:
		return nativeconfig.TransitionStateUnknown, err
	}
}

func (q *transitionReconciler) complete(observation nativeconfig.TransitionObservation) error {
	if observation == nativeconfig.TransitionSource {
		if err := q.restoreTransitionSource(); err != nil {
			return err
		}
		q.effect = domain.NativeEffectUnchanged
	} else {
		q.record.Phase = "state_committed"
		if err := writeTransitionRecord(q.record); err != nil {
			return err
		}
	}
	q.record.Phase = "cleanup_pending"
	if err := writeTransitionRecord(q.record); err != nil {
		return err
	}
	return cleanupTransition(q.record)
}

func (q *transitionReconciler) restoreTransitionSource() error {
	if err := restoreTransitionSkills(q.record); err != nil {
		return err
	}
	current, err := q.transitions.State.Load()
	if err != nil {
		return err
	}
	if !transitionStateMatches(current, q.record.OldState, q.record.Identity) && !transitionStateMatches(current, sourceTransitionState(q.record), q.record.Identity) {
		return fmt.Errorf("source state changed before skill recovery")
	}
	desired, err := transitionPublicationState(current, sourceTransitionState(q.record), q.record.Identity)
	if err != nil {
		return err
	}
	disposition, err := q.transitions.State.PersistStateDecisionWithDisposition(current, desired)
	if err != nil || disposition != domain.StateDecisionDesired {
		return errors.Join(err, fmt.Errorf("source outcome durability unresolved"))
	}
	return nil
}

func (t NativeTransitions) HoldsActivation(installationID, bindingID string) (bool, error) {
	pending, err := t.Pending()
	if err != nil {
		return false, err
	}
	for _, p := range pending {
		if p.Root != "" && p.InstallationID == installationID && p.BindingID == bindingID {
			return true, nil
		}
	}
	return false, nil
}
