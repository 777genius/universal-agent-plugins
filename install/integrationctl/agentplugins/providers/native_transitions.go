package providers

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// NativeTransitions only delegates existing recorders. Each adapter receives
// the same instance used by mutation recovery and the observed facade fence.
type NativeTransitions struct {
	state    ports.NativeTransitionState
	ordered  []ports.ObservedNativeTransitionRecovery
	byClient map[domain.ClientID]ports.ObservedNativeTransitionRecovery
}

func NewNativeTransitions(registry *clients.Registry, authority clients.NativeTransitionContext, existing ports.ObservedNativeTransitionRecovery) NativeTransitions {
	result := NativeTransitions{state: authority.State, byClient: map[domain.ClientID]ports.ObservedNativeTransitionRecovery{}}
	if existing != nil {
		result.ordered = append(result.ordered, existing)
	}
	for _, adapter := range registry.All() {
		if factory, ok := adapter.(clients.NativeTransitionProvider); ok {
			recorder := factory.NewNativeTransitions(authority)
			result.byClient[adapter.ID()] = recorder
			result.ordered = append(result.ordered, recorder)
		}
	}
	return result
}

func (transitions NativeTransitions) ForClient(id domain.ClientID) ports.NativeTransitionRecovery {
	return transitions.byClient[id]
}

func (transitions NativeTransitions) Pending() ([]ports.NativeTransitionObservation, error) {
	if transitions.state == nil {
		return nil, fmt.Errorf("native transition state authority is unavailable")
	}
	state, err := transitions.state.Load()
	if err != nil {
		return nil, err
	}
	var result []ports.NativeTransitionObservation
	operations, bindings := map[string]bool{}, map[string]bool{}
	for _, recorder := range transitions.ordered {
		if recorder == nil {
			return nil, fmt.Errorf("native transition recorder is unavailable")
		}
		pending, err := recorder.Pending()
		if err != nil {
			return nil, err
		}
		for _, item := range pending {
			key := item.InstallationID + "\x00" + item.BindingID
			owner, valid := ports.NativeTransitionOwner(state, item)
			if !valid || item.OperationID == "" || item.InstallationID == "" || item.BindingID == "" || operations[item.OperationID] || bindings[key] {
				return nil, fmt.Errorf("native transition observation is duplicated or lacks exact ownership")
			}
			item.EnvironmentID = owner
			operations[item.OperationID], bindings[key] = true, true
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].OperationID < result[j].OperationID })
	return result, nil
}

func (transitions NativeTransitions) Recover(ctx context.Context) error {
	if _, err := transitions.Pending(); err != nil {
		return err
	}
	for _, recorder := range transitions.ordered {
		if err := recorder.Recover(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (transitions NativeTransitions) HoldsActivation(installationID, bindingID string) (bool, error) {
	pending, err := transitions.Pending()
	if err != nil {
		return false, err
	}
	for _, item := range pending {
		if item.Root != "" && item.InstallationID == installationID && item.BindingID == bindingID {
			return true, nil
		}
	}
	return false, nil
}

func (transitions NativeTransitions) PrepareNativeRemovalDecision(ctx context.Context, installationID, bindingID string, before, desired domain.StateFileV2) (bool, error) {
	if _, err := transitions.Pending(); err != nil {
		return false, err
	}
	for _, recorder := range transitions.ordered {
		if decision, ok := recorder.(ports.NativeRemovalDecisionRecorder); ok {
			handled, err := decision.PrepareNativeRemovalDecision(ctx, installationID, bindingID, before, desired)
			if handled || err != nil {
				return handled, err
			}
		}
	}
	return false, nil
}

func (transitions NativeTransitions) ReconcileObserved(ctx context.Context, observed ports.NativeTransitionObservation, state ports.NativeTransitionState) error {
	live, err := transitions.Pending()
	if err != nil {
		return err
	}
	found := false
	for _, item := range live {
		if reflect.DeepEqual(item, observed) {
			found = true
		}
	}
	if !found || observed.Root == "" {
		return fmt.Errorf("native transition differs from the exact observed scope")
	}
	for _, recorder := range transitions.ordered {
		pending, err := recorder.Pending()
		if err != nil {
			return err
		}
		for _, item := range pending {
			item.EnvironmentID = observed.EnvironmentID
			if reflect.DeepEqual(item, observed) {
				return recorder.ReconcileObserved(ctx, item, state)
			}
		}
	}
	return fmt.Errorf("native transition owner disappeared")
}
