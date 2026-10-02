package installer

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func selectedNativeOnly(selected domain.SelectedDelivery) bool {
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
	if binding.ClientID != string(client.ClientID) || binding.Scope != string(domain.ScopeUser) || binding.NativeActivationAttempt != "" || binding.PendingNativeIntent != nil {
		return fmt.Errorf("%w: removal binding scope or native attempt differs", ErrInvalidRequest)
	}
	if err := binding.SelectedDelivery.Validate(); err != nil {
		return err
	}
	if binding.SelectedDelivery.EffectiveTraits(client.ClientID).BindsNativeProfileRoot && (client.ConfigRoot == "" || client.ConfigRoot != binding.NativeProfileRoot) {
		return fmt.Errorf("%w: removal profile differs from persisted native root", ErrInvalidRequest)
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
			if err := e.confirmRemovalPreflight(ctx, client, binding, receipt, state); err != nil {
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

type removalConfirmationLock struct {
	inner    ports.MutationLock
	validate func() error
}

func (l removalConfirmationLock) Acquire(ctx context.Context) (ports.UnlockFunc, error) {
	release, err := l.inner.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	if err := l.validate(); err != nil {
		_ = release()
		return nil, err
	}
	return release, nil
}

func removalConflict(result Result, err error) (Result, error) {
	result.Outcome, result.Reason = OutcomeConflict, "plan_changed"
	return result, err
}

// Guard the actual dispatch as well as the acquisition: a preceding read-only
// native observer may have changed a later target. Only this service's durable
// removal attempt is allowed between confirmation and that target's dispatch.
type removalConfirmationActivator struct {
	ports.ClientActivator
	engine   *Engine
	prepared *PreparedOperation
}

func (a removalConfirmationActivator) Deactivate(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	scope, err := a.engine.confirmRemoveDispatch(ctx, a.prepared, request)
	if err != nil {
		return domain.DeactivationOutcome{}, err
	}
	// A standard child keeps cancellation, deadline and values intact. Only
	// Err's forwarding boundary is fenced; domain conflicts remain return errors.
	child, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	guarded := removalDispatchContext{Context: child, caller: ctx, scope: scope, cancel: cancel}
	outcome, err := a.ClientActivator.Deactivate(guarded, request)
	cancel(nil)
	return outcome, scope.confirm(err)
}

// This is the approved post-attempt state, captured before observational
// preflight. Neither a context callback nor the adapter's return may replace it.
type removalDispatchScope struct {
	engine   *Engine
	state    domain.StateFileV2
	journals []dirswap.Receipt
}

func (s removalDispatchScope) confirm(cause error) error {
	journals, err := (dirswap.Manager{JournalDir: s.engine.cfg.OperationsDir}).ListOpen()
	if err != nil || !reflect.DeepEqual(journals, s.journals) {
		cause = errors.Join(cause, ErrPlanChanged, err)
	}
	return s.engine.confirmRemovalState(s.state, cause)
}

// The provider and selected adapter can call Err immediately before a native
// effect. Fence both sides of each caller callback without calling this wrapper
// recursively. Refusal closes the child's Done and gives it standard Err
// semantics; the outer complete-scope comparison supplies ErrPlanChanged.
type removalDispatchContext struct {
	context.Context
	caller context.Context
	scope  removalDispatchScope
	cancel context.CancelCauseFunc
}

func (c removalDispatchContext) Err() error {
	cause := c.scope.confirm(nil)
	if cause == nil {
		cause = c.scope.confirm(c.caller.Err())
	}
	if cause != nil {
		c.cancel(cause)
	}
	return c.Context.Err()
}

func (e *Engine) confirmRemoveDispatch(ctx context.Context, prepared *PreparedOperation, request domain.DeactivationRequest) (removalDispatchScope, error) {
	scope := removalDispatchScope{engine: e}
	state, err := e.store.Load()
	if err != nil {
		return scope, err
	}
	scope.state = state
	scope.journals, err = (dirswap.Manager{JournalDir: e.cfg.OperationsDir}).ListOpen()
	if err != nil {
		return scope, errors.Join(ErrPlanChanged, err)
	}
	oldInstall, _ := findInstall(prepared.recorded, prepared.plan.InstallationID)
	current, ok := findInstall(state, prepared.plan.InstallationID)
	if !ok {
		return scope, ErrPlanChanged
	}
	old, oldReceipt, found := findBinding(oldInstall, request.Client.ClientID)
	binding, receipt, live := findBinding(current, request.Client.ClientID)
	if !found || !live {
		return scope, ErrPlanChanged
	}
	// beginNativeAttempt is the existing service's own permitted transition.
	if binding.PendingNativeIntent != nil {
		if err := binding.PendingNativeIntent.Validate(binding); err != nil {
			return scope, err
		}
		if binding.PendingNativeIntent.Direction != domain.NativeIntentRemove || !reflect.DeepEqual(binding.PendingNativeIntent.Delivery, old.SelectedDelivery) {
			return scope, ErrPlanChanged
		}
	}
	binding.NativeActivationAttempt, binding.PendingNativeIntent = "", nil
	if !reflect.DeepEqual(binding, old) || !reflect.DeepEqual(receipt, oldReceipt) || !reflect.DeepEqual(request.SelectedDelivery, old.SelectedDelivery) || !reflect.DeepEqual(request.NativeObjects, old.NativeObjects) {
		return scope, ErrPlanChanged
	}
	// Keep the already recorded Service attempt in the comparison. An observer
	// cannot replace any binding, settings identity or receipt authority, even
	// when it returns an error or cancellation. Check before baseline capture.
	return scope, scope.confirm(e.confirmRemovalPreflight(ctx, request.Client, binding, receipt, state))
}

func (e *Engine) confirmRemovalPreflight(ctx context.Context, client domain.DetectedClient, binding domain.ClientBinding, receipt domain.DataReceipt, approved domain.StateFileV2) error {
	manager := dirswap.Manager{JournalDir: e.cfg.OperationsDir}
	journals, err := manager.ListOpen()
	if err != nil {
		return errors.Join(ErrPlanChanged, err)
	}
	cause := e.removalPreflight(ctx, client, binding, receipt)
	after, err := manager.ListOpen()
	if err != nil || !reflect.DeepEqual(after, journals) {
		cause = errors.Join(cause, ErrPlanChanged, err)
	}
	return e.confirmRemovalState(approved, cause)
}

func (e *Engine) confirmRemovalState(approved domain.StateFileV2, cause error) error {
	after, err := e.store.Load()
	if err != nil {
		return errors.Join(cause, ErrPlanChanged, err)
	}
	if !reflect.DeepEqual(after, approved) {
		return errors.Join(cause, ErrPlanChanged)
	}
	return cause
}

func validateSelectedBindingIdentity(binding domain.ClientBinding) error {
	facts, ok := binding.SelectedDelivery.LocalFacts()
	if !ok || binding.Scope != string(domain.ScopeUser) || facts.ProfileRoot != binding.NativeProfileRoot || facts.Registration.Selector != binding.TargetLocator || facts.ProjectionDigest != managedPackageDigest(binding) || binding.PackageRevision == nil || facts.CanonicalDigest != binding.PackageRevision.TreeDigest {
		return fmt.Errorf("%w: selected package/profile authority differs", ErrInvalidRequest)
	}
	return nil
}

func removalClientResult(facts BindingFacts) ClientResult {
	return ClientResult{ClientID: facts.ClientID, BindingID: facts.BindingID, TreeDigest: facts.TreeDigest, SelectedDelivery: facts.SelectedDelivery}
}

// selectedNativeInspection is a read-only registered capability. Historical
// CLI registry inspection is ineligible even if an adapter has a reconciler.
// The adapter owns its profile parser; no constructor plan or ambient discovery
// becomes removal authority, and no Deactivate preview is used as preflight.
type selectedNativeInspection interface {
	usecase.NativeIntentReconciler
	clients.RegistryInspector
}

func (e *Engine) confirmSelectedNativeEntry(ctx context.Context, client domain.DetectedClient, binding domain.ClientBinding) error {
	if binding.SelectedDelivery.IsZero() {
		return nil
	}
	adapter, ok := clients.As[selectedNativeInspection](e.cfg.Registry, client.ClientID)
	if !ok || adapter.UsesNativeRegistryExecutable() {
		return fmt.Errorf("%w: selected client has no read-only profile inspection", ErrUnsupported)
	}
	facts, _ := binding.SelectedDelivery.LocalFacts()
	plan := domain.DeliveryPlan{ClientID: client.ClientID, Scope: domain.ScopeUser, ActivePath: binding.TargetLocator,
		PhysicalArtifactID: binding.PhysicalArtifact, SelectedDelivery: binding.SelectedDelivery, NativeRegistryRoot: facts.ProfileRoot}
	finding, err := adapter.InspectNativeRegistry(ctx, clients.Env{NativeConfig: nativeconfig.New()}, client, plan, &binding)
	if err != nil {
		return err
	}
	expected := clients.RegistryClear
	if binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) {
		expected = clients.RegistryExpected
	}
	if finding != expected {
		return fmt.Errorf("%w: selected native entry no longer matches persisted ownership", ErrPlanChanged)
	}
	return nil
}
