package installer

import (
	"context"
	"fmt"
	"reflect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

type seamActivator struct {
	independent  bool
	profileCheck func(context.Context) error
	inner        providers.Activator
	onCommitted  func(context.Context, BindingFacts) error
	store        interface {
		Load() (domain.StateFileV2, error)
	}
	facts BindingFacts
}

func (a seamActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	prepared, err := a.prepareActivation(ctx, request)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	return a.inner.Activate(ctx, prepared)
}

// Native config adapters may write the shared kernel while activating. Effect
// isolation must be explicitly attested by the registered client adapter.
func (a seamActivator) SupportsIndependentActivation(request domain.ActivationRequest) bool {
	return a.independent && a.inner.SupportsIndependentActivation(request)
}

func (a seamActivator) PrepareIndependentActivation(ctx context.Context, request domain.ActivationRequest) (func(context.Context) (domain.ActivationOutcome, error), error) {
	if !a.SupportsIndependentActivation(request) {
		return nil, fmt.Errorf("client activation cannot run independently")
	}
	prepared, err := a.prepareActivation(ctx, request)
	if err != nil {
		return nil, err
	}
	return func(effectContext context.Context) (domain.ActivationOutcome, error) {
		return a.inner.Activate(effectContext, prepared)
	}, nil
}

func (a seamActivator) prepareActivation(ctx context.Context, request domain.ActivationRequest) (domain.ActivationRequest, error) {
	if !request.Plan.SelectedDelivery.IsZero() {
		if _, err := a.committedFacts(request); err != nil {
			return request, err
		}
	}
	// The engine reconciles a pending host handoff before resuming an existing
	// package. CLI activation may still need effects, independently of that
	// completed callback. A staged replacement must run its own post-commit hook.
	// Selected deliveries retain their separate confirmation contract.
	resume := request.PackageUnchanged && request.Plan.SelectedDelivery.IsZero() && a.facts.BindingID != ""
	if resume {
		if a.hostHandoffPending(request) && !keepProjectVerifyOnly(request) {
			request.VerifyOnly = false
		}
	} else if a.onCommitted != nil && !request.VerifyOnly {
		facts, err := a.committedFacts(request)
		if err != nil {
			return request, err
		}
		if err := a.onCommitted(ctx, facts); err != nil {
			return request, err
		}
	}
	if a.profileCheck != nil {
		if err := a.profileCheck(ctx); err != nil {
			return request, err
		}
	}
	return request, nil
}

// keepProjectVerifyOnly preserves the core's existing read-only decision for
// verified project component files. Prepared session activation alone must not
// turn that verification back into a write. It never invents VerifyOnly.
func keepProjectVerifyOnly(request domain.ActivationRequest) bool {
	if !request.VerifyOnly || !request.PackageUnchanged || !request.Plan.SelectedDelivery.IsZero() ||
		request.Plan.Scope != domain.ScopeProject || request.Plan.ClientID != request.Client.ClientID {
		return false
	}
	target, err := domain.NewTargetContext(domain.ScopeProject, request.Plan.TargetContext.ProjectRoot, request.Client.ConfigRoot)
	return err == nil && target == request.Plan.TargetContext &&
		request.Plan.SelectedDelivery.EffectiveTraitsForScope(request.Client.ClientID, request.Plan.Scope).LifecycleKind == domain.LifecycleNativeConfig
}

func hostHandoffPending(binding domain.ClientBinding) bool {
	switch binding.Materialization {
	case domain.MaterializationMaterialized, domain.MaterializationDegraded:
	default:
		return false
	}
	if binding.InstallIntent == domain.InstallIntentPrepare {
		return binding.Activation != domain.ActivationPrepared
	}
	switch binding.Activation {
	case "", domain.ActivationFailed, domain.ActivationPrepared:
		return true
	default:
		return false
	}
}

func (a seamActivator) hostHandoffPending(request domain.ActivationRequest) bool {
	if a.store == nil {
		return false
	}
	state, err := a.store.Load()
	if err != nil {
		return false
	}
	installation, ok := findInstall(state, a.facts.InstallationID)
	if !ok {
		return false
	}
	for _, binding := range installation.Clients {
		if binding.TargetLocator != request.Plan.ActivePath || binding.ClientID != string(request.Plan.ClientID) {
			continue
		}
		return hostHandoffPending(binding)
	}
	return false
}

func (a seamActivator) AutomaticallyActivates(request domain.ActivationRequest) bool {
	return a.inner.AutomaticallyActivates(request)
}

func (a seamActivator) VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, executable string) bool {
	return a.inner.VerifierAvailable(client, plan, executable)
}

func (a seamActivator) PreflightActivation(request domain.ActivationRequest) error {
	return a.inner.PreflightActivation(request)
}

func (a seamActivator) Deactivate(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return a.inner.Deactivate(ctx, request)
}

func (a seamActivator) committedFacts(request domain.ActivationRequest) (BindingFacts, error) {
	facts := a.facts
	facts.TargetContext = request.Plan.TargetContext
	if a.store == nil {
		return uncommittedFacts(facts, request.Plan.SelectedDelivery)
	}
	state, err := a.store.Load()
	if err != nil {
		return BindingFacts{}, err
	}
	installation, ok := findInstall(state, facts.InstallationID)
	if !ok {
		return uncommittedFacts(facts, request.Plan.SelectedDelivery)
	}
	for _, binding := range installation.Clients {
		if binding.TargetLocator != request.Plan.ActivePath || binding.ClientID != string(request.Plan.ClientID) {
			continue
		}
		if !reflect.DeepEqual(binding.SelectedDelivery, request.Plan.SelectedDelivery) {
			return BindingFacts{}, fmt.Errorf("%w: committed delivery differs from activation plan", ErrPlanChanged)
		}
		facts.ProfileAuthority = domain.CloneProfileAuthority(binding.ProfileAuthority)
		facts.SelectedDelivery = binding.SelectedDelivery
		receipt := installation.DataReceipts[binding.DataReceiptID]
		facts.BindingID = binding.ClientBindingID
		facts.Scope = binding.Scope
		facts.TargetPath = binding.TargetLocator
		facts.DataRoot = receipt.Locator
		facts.DataReceiptID = binding.DataReceiptID
		facts.ClientID = binding.ClientID
		facts.TreeDigest = recordedBindingDigest(binding, facts.TreeDigest)
		return facts, nil
	}
	return uncommittedFacts(facts, request.Plan.SelectedDelivery)
}

func uncommittedFacts(facts BindingFacts, selected domain.SelectedDelivery) (BindingFacts, error) {
	if !selected.IsZero() {
		return BindingFacts{}, fmt.Errorf("%w: selected delivery has no committed binding", ErrPlanChanged)
	}
	return facts, nil
}
