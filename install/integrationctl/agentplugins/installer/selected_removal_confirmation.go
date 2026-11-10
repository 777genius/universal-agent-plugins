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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

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
	journals, err := (dirswap.Manager{Namespace: s.engine.cfg.StateRoot, JournalDir: s.engine.cfg.OperationsDir}).ListOpen()
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
	scope.journals, err = (dirswap.Manager{Namespace: e.cfg.StateRoot, JournalDir: e.cfg.OperationsDir}).ListOpen()
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
	if !reflect.DeepEqual(binding, old) || !reflect.DeepEqual(receipt, oldReceipt) || !reflect.DeepEqual(request.SelectedDelivery, old.SelectedDelivery) || !reflect.DeepEqual(request.NativeObjects, old.NativeObjects) || !request.LocalEntryObservation.Equal(old.LocalEntryObservation) {
		return scope, ErrPlanChanged
	}
	// Keep the already recorded Service attempt in the comparison. An observer
	// cannot replace any binding, settings identity or receipt authority, even
	// when it returns an error or cancellation. Check before baseline capture.
	return scope, scope.confirm(e.confirmRemovalPreflight(ctx, request.Client, current.DeclaredName, binding, receipt, state))
}

func (e *Engine) confirmRemovalPreflight(ctx context.Context, client domain.DetectedClient, declaredName string, binding domain.ClientBinding, receipt domain.DataReceipt, approved domain.StateFileV2) error {
	manager := dirswap.Manager{Namespace: e.cfg.StateRoot, JournalDir: e.cfg.OperationsDir}
	journals, err := manager.ListOpen()
	if err != nil {
		return errors.Join(ErrPlanChanged, err)
	}
	cause := e.removalPreflight(ctx, client, declaredName, binding, receipt)
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
	if f, ok := binding.SelectedDelivery.CursorFacts(); ok {
		if binding.SelectedDelivery.ValidateClient(domain.ClientID(binding.ClientID)) != nil || binding.Scope != string(domain.ScopeUser) || f.ProfileRoot != binding.NativeProfileRoot || f.ProjectionDigest != managedPackageDigest(binding) || binding.PackageRevision == nil || f.CanonicalDigest != binding.PackageRevision.TreeDigest {
			return fmt.Errorf("%w: Cursor binding authority differs", ErrInvalidRequest)
		}
		return binding.SelectedDelivery.ValidateCursorObjects(binding.NativeObjects)
	}
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
	binding.LocalEntryObservation = binding.LocalEntryObservation.Clone()
	binding.PendingNativeIntent = binding.PendingNativeIntent.Clone()
	if binding.SelectedDelivery.IsZero() {
		return nil
	}
	adapter, ok := clients.As[selectedNativeInspection](e.cfg.Registry, client.ClientID)
	if !ok || adapter.UsesNativeRegistryExecutable() {
		return fmt.Errorf("%w: selected client has no read-only profile inspection", ErrUnsupported)
	}
	plan := domain.DeliveryPlan{ClientID: client.ClientID, Scope: e.scope(), ActivePath: binding.TargetLocator,
		LocalEntryObservation: binding.LocalEntryObservation.Clone(), PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...),
		PhysicalArtifactID: binding.PhysicalArtifact, SelectedDelivery: binding.SelectedDelivery, NativeRegistryRoot: binding.SelectedDelivery.ProfileRoot()}
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
