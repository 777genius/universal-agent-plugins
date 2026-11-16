package installer

import (
	"context"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

// The facade retains its mutation lock and exact observed scope. The existing
// client recorder owns native classification, state publication and cleanup;
// this guard permits only its current journal and binding to advance.
func (e *Engine) recoverObservedTransitions(ctx context.Context, recovery ports.NativeTransitionRecovery, observed RecoveryObservation) error {
	t, ok := recovery.(ports.ObservedNativeTransitionRecovery)
	if !ok {
		return fmt.Errorf("native transition observation is unavailable")
	}
	pending, err := t.Pending()
	if err != nil {
		return err
	}
	for _, item := range pending {
		if item.Root == "" {
			return fmt.Errorf("native attempt has no durable transition record; recovery required")
		}
		journal := PendingJournal{EnvironmentID: item.EnvironmentID, OperationID: item.OperationID, InstallationID: item.InstallationID, BindingID: item.BindingID, TargetPath: item.TargetPath, Phase: item.Phase, Digest: item.Digest}
		if !containsJournal(observed.Journals, journal) {
			return ErrPlanChanged
		}
		guard := &nativeTransitionRecoveryState{engine: e, expected: observed, active: journal, authorityDigest: item.AuthorityDigest, pending: t.Pending}
		if err := guard.confirm(false); err != nil {
			return err
		}
		if err := t.ReconcileObserved(ctx, item, guard); err != nil {
			return err
		}
		if err := guard.confirm(true); err != nil {
			return err
		}
		after, err := e.observe()
		if err != nil {
			return err
		}
		observed = after.Recovery
	}
	return nil
}

type nativeTransitionRecoveryState struct {
	engine          *Engine
	expected        RecoveryObservation
	active          PendingJournal
	authorityDigest string
	pending         func() ([]ports.NativeTransitionObservation, error)
}

func (s *nativeTransitionRecoveryState) confirm(completed bool) error {
	view, err := s.engine.observe()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPlanChanged, err)
	}
	pending, err := s.pending()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPlanChanged, err)
	}
	for _, item := range pending {
		if item.OperationID == s.active.OperationID && (completed || item.Root == "" || item.AuthorityDigest != s.authorityDigest) {
			return ErrPlanChanged
		}
	}
	want, err := s.withoutActiveJournal(s.expected, true, completed)
	if err != nil {
		return err
	}
	actual, err := s.withoutActiveJournal(view.Recovery, false, completed)
	if err != nil || !reflect.DeepEqual(actual, want) {
		return ErrPlanChanged
	}
	return nil
}

// The recorder validates the complete active journal, including its hash
// and prepared authority. Only its own phase/hash can change in this guard.
func (s *nativeTransitionRecoveryState) withoutActiveJournal(observation RecoveryObservation, expected, completed bool) (RecoveryObservation, error) {
	filtered := observation
	filtered.Journals = nil
	found := false
	for _, journal := range observation.Journals {
		if journal.OperationID != s.active.OperationID {
			filtered.Journals = append(filtered.Journals, journal)
			continue
		}
		if found || journal.EnvironmentID != s.active.EnvironmentID || journal.InstallationID != s.active.InstallationID || journal.BindingID != s.active.BindingID || journal.TargetPath != s.active.TargetPath {
			return filtered, ErrPlanChanged
		}
		found = true
	}
	if (!expected && completed && found) || ((expected || !completed) && !found) {
		return filtered, ErrPlanChanged
	}
	filtered.Required = len(filtered.Journals)+len(filtered.Receipts)+len(filtered.NativeIntents) != 0
	return filtered, nil
}

func (s *nativeTransitionRecoveryState) Load() (domain.StateFileV2, error) {
	if err := s.confirm(false); err != nil {
		return domain.StateFileV2{}, err
	}
	state, err := s.engine.store.Load()
	if err != nil {
		return state, err
	}
	digest, err := recoveryStateDigest(state)
	if err != nil || digest != s.expected.StateDigest {
		return domain.StateFileV2{}, ErrPlanChanged
	}
	return state, s.confirm(false)
}

func (s *nativeTransitionRecoveryState) PersistStateDecisionWithDisposition(before, desired domain.StateFileV2) (domain.StateDecisionDisposition, error) {
	if err := s.confirm(false); err != nil {
		return domain.StateDecisionUnknown, err
	}
	digest, err := recoveryStateDigest(before)
	if err != nil || digest != s.expected.StateDigest {
		return domain.StateDecisionUnknown, ErrPlanChanged
	}
	// The provider may publish only this binding. Sibling lifecycle state,
	// directory receipts, package revision and installation metadata stay exact.
	normalized := desired
	normalized.Installations = append([]domain.Installation(nil), desired.Installations...)
	found := false
	for i, installation := range normalized.Installations {
		if installation.InstallationID != s.active.InstallationID {
			continue
		}
		for _, original := range before.Installations {
			if original.InstallationID != installation.InstallationID {
				continue
			}
			old, ok := original.Clients[s.active.BindingID]
			if !ok {
				return domain.StateDecisionUnknown, ErrPlanChanged
			}
			installation.Clients = make(map[string]domain.ClientBinding, len(installation.Clients))
			for key, binding := range desired.Installations[i].Clients {
				installation.Clients[key] = binding
			}
			if _, ok := installation.Clients[s.active.BindingID]; !ok {
				return domain.StateDecisionUnknown, ErrPlanChanged
			}
			installation.Clients[s.active.BindingID] = old
			normalized.Installations[i] = installation
			found = true
		}
	}
	normalizedDigest, err := recoveryStateDigest(normalized)
	if err != nil || !found || normalizedDigest != digest {
		return domain.StateDecisionUnknown, ErrPlanChanged
	}
	desiredDigest, err := recoveryStateDigest(desired)
	if err != nil {
		return domain.StateDecisionUnknown, err
	}
	decision, err := (transaction.Kernel{StateStore: s.engine.store}).PersistStateDecisionWithDisposition(before, desired)
	if decision == domain.StateDecisionDesired {
		s.expected.StateDigest = desiredDigest
	}
	return decision, err
}
