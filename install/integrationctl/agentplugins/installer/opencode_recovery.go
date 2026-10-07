package installer

import (
	"context"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// The facade retains its mutation lock and exact observed scope. The existing
// OpenCode recorder owns native classification, state publication and cleanup;
// this guard permits only its current journal and binding to advance.
func (e *Engine) recoverObservedOpenCode(ctx context.Context, svc usecase.Service, observed RecoveryObservation) error {
	t := opencode.NativeTransitions{PackageVerifier: svc.Stager, State: transaction.Kernel{StateStore: e.store}, Kernel: nativeconfig.New()}
	pending, err := t.Pending()
	if err != nil {
		return err
	}
	for _, item := range pending {
		if item.Root == "" {
			return fmt.Errorf("native attempt has no durable transition record; recovery required")
		}
		journal := PendingJournal{OperationID: item.OperationID, InstallationID: item.InstallationID, BindingID: item.BindingID, TargetPath: item.TargetPath, Phase: item.Phase, Digest: item.Digest}
		if !containsJournal(observed.Journals, journal) {
			return ErrPlanChanged
		}
		guard := &openCodeRecoveryState{engine: e, expected: observed, active: journal, authorityDigest: item.AuthorityDigest}
		if err := guard.confirm(false); err != nil {
			return err
		}
		t.State = guard
		if _, err := t.Reconcile(ctx, item.Root); err != nil {
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

type openCodeRecoveryState struct {
	engine          *Engine
	expected        RecoveryObservation
	active          PendingJournal
	authorityDigest string
}

func (s *openCodeRecoveryState) confirm(completed bool) error {
	view, err := s.engine.observe()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPlanChanged, err)
	}
	pending, err := (opencode.NativeTransitions{State: transaction.Kernel{StateStore: s.engine.store}}).Pending()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPlanChanged, err)
	}
	for _, item := range pending {
		if item.OperationID == s.active.OperationID && (completed || item.Root == "" || item.AuthorityDigest != s.authorityDigest) {
			return ErrPlanChanged
		}
	}
	// The recorder validates the complete active journal, including its hash
	// and prepared authority. Only its own phase/hash can change in this guard.
	withoutActive := func(observation RecoveryObservation, expected bool) (RecoveryObservation, error) {
		filtered := observation
		filtered.Journals = nil
		found := false
		for _, journal := range observation.Journals {
			if journal.OperationID != s.active.OperationID {
				filtered.Journals = append(filtered.Journals, journal)
				continue
			}
			if found || journal.InstallationID != s.active.InstallationID || journal.BindingID != s.active.BindingID || journal.TargetPath != s.active.TargetPath {
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
	want, err := withoutActive(s.expected, true)
	if err != nil {
		return err
	}
	actual, err := withoutActive(view.Recovery, false)
	if err != nil || !reflect.DeepEqual(actual, want) {
		return ErrPlanChanged
	}
	return nil
}

func (s *openCodeRecoveryState) Load() (domain.StateFileV2, error) {
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

func (s *openCodeRecoveryState) PersistStateDecisionWithDisposition(before, desired domain.StateFileV2) (domain.StateDecisionDisposition, error) {
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
