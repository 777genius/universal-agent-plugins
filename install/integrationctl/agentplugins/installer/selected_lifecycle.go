package installer

import (
	"context"
	"fmt"
	"reflect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func selectedNativeOnly(selected domain.SelectedDelivery) bool {
	if _, ok := selected.CursorFacts(); ok {
		return true
	}
	facts, ok := selected.LocalFacts()
	return ok && len(facts.MCPServers) == 0
}

func (e *Engine) preparedHelper(prepared *PreparedOperation) (*managedstdio.Source, error) {
	// Historical operations and configured helper validation retain their contract.
	if e.cfg.HelperExecutable != "" || !selectedNativeOnly(prepared.plan.SelectedDelivery) {
		return e.helper()
	}
	for _, target := range prepared.plan.Targets {
		if !selectedNativeOnly(target.SelectedDelivery) {
			return e.helper()
		}
	}
	return nil, nil
}

func (e *Engine) selectedReconciler(clientID string, selected domain.SelectedDelivery) (usecase.NativeIntentReconciler, error) {
	if selected.IsZero() {
		return nil, fmt.Errorf("%w: selected native authority is missing", ErrUnsupported)
	}
	if err := selected.Validate(); err != nil {
		return nil, err
	}
	if err := selected.ValidateClient(domain.ClientID(clientID)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	adapter, ok := e.cfg.Registry.Lookup(domain.ClientID(clientID))
	if !ok {
		return nil, fmt.Errorf("%w: selected client is not registered", ErrUnsupported)
	}
	// Historical adapters do not implement this typed capability. Never dispatch
	// a persisted Local decision to their shared CLI lifecycle.
	reconciler, ok := adapter.(usecase.NativeIntentReconciler)
	if !ok {
		return nil, fmt.Errorf("%w: registered client has no selected delivery native lifecycle", ErrUnsupported)
	}
	return reconciler, nil
}

func (e *Engine) validateRemovalBinding(client domain.DetectedClient, binding domain.ClientBinding) error {
	if err := binding.ValidateLocalEntryObservation(); err != nil {
		return err
	}
	if binding.ClientID != string(client.ClientID) || binding.Scope != string(e.scope()) || binding.NativeActivationAttempt != "" || binding.PendingNativeIntent != nil {
		return fmt.Errorf("%w: removal binding scope or native attempt differs", ErrInvalidRequest)
	}
	if err := binding.SelectedDelivery.Validate(); err != nil {
		return err
	}
	if binding.SelectedDelivery.EffectiveTraitsForScope(client.ClientID, e.scope()).BindsNativeProfileRoot {
		root, err := e.nativeRegistryRoot(client)
		if err != nil || root == "" || root != binding.NativeProfileRoot {
			return fmt.Errorf("%w: removal profile differs from persisted native root", ErrInvalidRequest)
		}
	}
	if binding.SelectedDelivery.IsZero() {
		return nil
	}
	if _, err := e.selectedReconciler(binding.ClientID, binding.SelectedDelivery); err != nil {
		return err
	}
	return validateSelectedBindingIdentity(binding)
}

// Compare every persisted target before the first observer or mutation. Removal
// never replans from constructor facts: its complete recorded binding is frozen.
func (e *Engine) confirmRemoveBindings(ctx context.Context, prepared *PreparedOperation) error {
	state, err := e.store.Load()
	if err != nil {
		return err
	}
	before, _ := findInstall(prepared.recorded, prepared.plan.InstallationID)
	live, ok := findInstall(state, prepared.plan.InstallationID)
	if !ok {
		return fmt.Errorf("%w: removal installation disappeared", ErrPlanChanged)
	}
	removalClients := prepared.clients
	if len(removalClients) == 0 {
		removalClients = []domain.DetectedClient{prepared.client}
	}
	for _, client := range removalClients {
		old, oldReceipt, oldOK := findBinding(before, client.ClientID)
		binding, receipt, found := findBinding(live, client.ClientID)
		if oldOK != found || !reflect.DeepEqual(old, binding) || !reflect.DeepEqual(oldReceipt, receipt) {
			return fmt.Errorf("%w: persisted removal authority changed", ErrPlanChanged)
		}
		if found {
			if err := e.confirmRemovalPreflight(ctx, client, live.DeclaredName, binding, receipt, state); err != nil {
				return fmt.Errorf("%w: %w", ErrPlanChanged, err)
			}
		}
	}
	// The registered inspectors are observational callbacks. A state change
	// during any target's inspection cannot become the next target's authority.
	return e.confirmRemovalState(state, nil)
}

// The use case owns the one acquisition. Repeat validation under that lock,
// before its journal recovery or first native observer, without nesting locks.
func (e *Engine) removalLifecycle(ctx context.Context, prepared *PreparedOperation, svc usecase.Service) usecase.Service {
	svc.Activator = removalConfirmationActivator{ClientActivator: svc.Activator, engine: e, prepared: prepared}
	svc.Lock = removalConfirmationLock{inner: svc.Lock, validate: func() error { return e.confirmRemoveBindings(ctx, prepared) }}
	return svc
}
