package hermes

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.Lifecycle = (*Adapter)(nil)
	_ clients.AutomaticActivator = (*Adapter)(nil)
	_ clients.ActivationPreflighter = (*Adapter)(nil)
	_ clients.RegistryInspector = (*Adapter)(nil)
)

func (*Adapter) AutomaticallyActivates(_ clients.Env, req domain.ActivationRequest) bool {
	return req.Plan.InstallIntent == domain.InstallIntentAutomatic && req.Plan.NativeRegistryRoot != ""
}

func (*Adapter) PreflightActivation(env clients.Env, req domain.ActivationRequest) error {
	// Group preflight runs before staging, so Delivery has no identity or path
	// yet. Validate the selected client and plan now; demand exact delivery
	// identity only once a staged delivery actually exists.
	if req.Plan.ClientID != domain.ClientHermes || req.Client.ClientID != domain.ClientHermes {
		return fmt.Errorf("hermes selected client identity differs")
	}
	if req.Delivery.ClientID != "" || req.Delivery.ActivePath != "" {
		if err := shared.ActivationIdentityMismatch(req); err != nil {
			return err
		}
		if req.Delivery.ActivePath != req.Plan.ActivePath {
			return fmt.Errorf("hermes staged delivery path differs")
		}
	}
	if req.ActivationComplete {
		return fmt.Errorf("hermes runtime activation cannot be attested by installer")
	}
	layout, err := components(req.Plan, req.Delivery.ActivePath)
	if err != nil {
		return err
	}
	desired, err := ownedObjects(req.Plan.Scope, req.Delivery.NativeObjects)
	if err != nil { return err }
	previous, err := ownedObjects(req.Plan.Scope, req.PreviousNativeObjects)
	if err != nil { return err }
	if err := layout.ValidateObjects(previous); err != nil { return err }
	if err := layout.ValidateObjects(desired); err != nil {
		return err
	}
	return env.NativeConfig.RequireFileIO()
}

func (*Adapter) UsesNativeRegistryExecutable() bool { return false }

func (*Adapter) InspectNativeRegistry(ctx context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, binding *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil { return clients.RegistryIndeterminate, err }
	if client.ClientID != domain.ClientHermes || plan.ClientID != domain.ClientHermes {
		return clients.RegistryIndeterminate, fmt.Errorf("hermes native observation client identity differs")
	}
	if plan.Scope == domain.ScopeUser && client.ConfigRoot != plan.NativeRegistryRoot {
		return clients.RegistryIndeterminate, fmt.Errorf("hermes User profile authority differs from native registry")
	}
	if plan.Scope == domain.ScopeProject && plan.TargetContext.UserConfigRoot != "" &&
		client.ConfigRoot != plan.TargetContext.UserConfigRoot {
		return clients.RegistryIndeterminate, fmt.Errorf("hermes Project request changed selected User profile")
	}
	layout, err := components(plan, "")
	if err != nil { return clients.RegistryIndeterminate, err }
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentSkill { continue }
		if component.Kind == domain.ComponentMCPServer && plan.Scope == domain.ScopeUser && component.Support != domain.SupportUnsupported { continue }
		return clients.RegistryIndeterminate, fmt.Errorf("hermes native observation cannot inspect an unsupported component")
	}
	if binding != nil {
		owned, err := ownedObjects(plan.Scope, binding.NativeObjects)
		if err != nil { return clients.RegistryIndeterminate, err }
		snapshot := *binding
		snapshot.NativeObjects = owned
		binding = &snapshot
	}
	return layout.Inspect(plan.Components, binding, env.NativeConfig)
}

func (*Adapter) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := shared.ActivationIdentityMismatch(req); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if req.Delivery.ActivePath != req.Plan.ActivePath {
		return domain.ActivationOutcome{}, fmt.Errorf("hermes staged delivery path differs")
	}
	if err := New().PreflightActivation(env, req); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if req.ActivationComplete {
		return domain.ActivationOutcome{}, fmt.Errorf("hermes runtime activation cannot be attested by installer")
	}
	layout, err := components(req.Plan, req.Delivery.ActivePath)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	desired, err := ownedObjects(req.Plan.Scope, req.Delivery.NativeObjects)
	if err != nil { return domain.ActivationOutcome{}, err }
	previous, err := ownedObjects(req.Plan.Scope, req.PreviousNativeObjects)
	if err != nil { return domain.ActivationOutcome{}, err }
	outcome, err := shared.CompleteNativeConfigActivation(ctx, req, shared.NativeConfigActivation{
		Automatic: req.Plan.InstallIntent == domain.InstallIntentAutomatic,
		UnavailableAction: "Select automatic installation for owned Hermes Skills",
		RepairAction: "Repair UAP-managed Hermes Skills in the selected scope",
		RetryAction: "Retry Hermes Skills installation after restoring ownership",
		CompletedAction: "Restart Hermes and review project trust before using installed Skills",
		Verify: func() error {
			return layout.Verify(desired, false, env.NativeConfig)
		},
		Activate: func(ctx context.Context, _ domain.ActivationRequest) error {
			servers, err := hermesDesiredServers(req.Delivery.ActivePath, desired)
			if err != nil { return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err} }
			return (shared.ProjectComponentMutation{
				Layout: layout, ActivePath: req.Delivery.ActivePath,
				Previous: previous, Desired: desired,
				Servers: servers, Kernel: env.NativeConfig,
			}).Apply(ctx)
		},
	})
	if err == nil && outcome.Verification == domain.VerificationInstalled {
		outcome.Activation = domain.ActivationPrepared
		outcome.ActivationAttested = false
	}
	return outcome, err
}

func (*Adapter) Deactivate(ctx context.Context, env clients.Env, req domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if err := ctx.Err(); err != nil {
		return outcome, err
	}
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientHermes,
		Scope: req.TargetContext.Scope, TargetContext: req.TargetContext,
		NativeRegistryRoot: req.NativeRegistryRoot,
	}
	layout, err := components(plan, "")
	if err != nil {
		return outcome, err
	}
	owned, err := ownedObjects(plan.Scope, req.NativeObjects)
	if err != nil { return outcome, err }
	if err := layout.ValidateObjects(owned); err != nil {
		return outcome, err
	}
	if !req.Confirmed {
		outcome.UserActions = append(outcome.UserActions, "Only exact-owned Hermes Skills will be removed; foreign Skills and project trust remain unchanged")
		return outcome, nil
	}
	err = (shared.ProjectComponentMutation{
		Layout: layout, Previous: owned,
		Kernel: env.NativeConfig,
	}).Apply(ctx)
	if err != nil {
		var observed *shared.NativeEffectError
		if errors.As(err, &observed) && observed.Effect == domain.NativeEffectUnchanged {
			outcome.NativeEffect = domain.NativeEffectUnchanged
		}
		if !shared.CommittedNativeDeactivationCleanup(&outcome, err) {
			return outcome, err
		}
	}
	outcome.NativeEffect = domain.NativeEffectCommitted
	outcome.ExternalRemovalComplete = true
	outcome.UserActions = append(outcome.UserActions, "Restart Hermes to clear previously loaded Skills")
	return outcome, nil
}
