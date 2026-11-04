package pi

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const piProjectComponentAction = "Review project trust in Pi and reload or restart the session to load project Skills and MCP; authentication, active connections and model calls are not implied by file installation."
const piUserComponentAction = "Reload resources or restart Pi to discover user Skills and MCP; installed files do not prove an authenticated connection or model tool call."

// piSelectedAuthority checks the selected real client separately from the
// planned target. validatePlan alone checks self-consistency and cannot prove
// that a project plan kept the originally detected user profile.
func piSelectedAuthority(client domain.DetectedClient, plan domain.DeliveryPlan) error {
	if client.ClientID != domain.ClientPi {
		return fmt.Errorf("pi selected client identity differs")
	}
	switch plan.Scope {
	case domain.ScopeProject:
		_, err := projectPiRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
		return err
	case domain.ScopeUser:
		if client.ConfigRoot != plan.TargetAnchor || client.ConfigRoot != plan.NativeRegistryRoot {
			return fmt.Errorf("pi user target differs from selected original profile")
		}
		return validateProfile(client.ConfigRoot)
	default:
		return fmt.Errorf("unsupported Pi component scope")
	}
}

// RefinePlan makes only the explicitly selected project-native component
// selection ready. An extension-only project remains valid; an unsupported
// portable component never becomes an implied native capability.
func (adapter *ProjectAdapter) RefinePlan(ctx context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if err := validatePlan(*plan); err != nil {
		return err
	}
	if err := piSelectedAuthority(in.Client, *plan); err != nil {
		return err
	}
	found := false
	components := false
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		switch component.Kind {
		case domain.ComponentSkill, domain.ComponentMCPServer:
			found = true
			components = true
		case domain.ComponentExtension:
			if component.Name != Namespace {
				return fmt.Errorf("unknown Pi extension")
			}
			found = true
		default:
			return fmt.Errorf("unqualified Pi component")
		}
	}
	if plan.Scope == domain.ScopeUser && !components {
		// Preserve historical user extension-only activation semantics.
		return adapter.Adapter.RefinePlan(ctx, in, plan)
	}
	shared.PromoteNativeReady(plan, plan.NativeRegistryRoot, found)
	action := piUserComponentAction
	if plan.Scope == domain.ScopeProject {
		action = piProjectComponentAction
	}
	plan.UserActions = shared.AppendUnique(plan.UserActions, action)
	return nil
}

func (adapter *ProjectAdapter) InspectNativeRegistry(ctx context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if err := validatePlan(plan); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if err := piSelectedAuthority(client, plan); err != nil {
		return clients.RegistryIndeterminate, err
	}
	found, err := inspectPackage(plan, managed != nil && managed.TargetLocator == plan.ActivePath)
	if err != nil || found == clients.RegistryCollision || found == clients.RegistryIndeterminate {
		return found, err
	}
	var prior *domain.ClientBinding
	if managed != nil {
		binding := *managed
		binding.NativeObjects = piScopedNative(managed.NativeObjects, plan.Scope)
		prior = &binding
	}
	selected := make([]domain.ComponentDecision, 0)
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentSkill || component.Kind == domain.ComponentMCPServer {
			selected = append(selected, component)
		}
	}
	if len(selected) == 0 && (prior == nil || len(prior.NativeObjects) == 0) {
		return found, nil
	}
	layout, err := piComponents(plan.NativeRegistryRoot, plan.Scope)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	native, err := layout.Inspect(selected, prior, env.NativeConfig)
	if err != nil || native == clients.RegistryCollision || native == clients.RegistryIndeterminate {
		return native, err
	}
	if native == clients.RegistryExpected || found == clients.RegistryExpected {
		return clients.RegistryExpected, nil
	}
	return clients.RegistryClear, nil
}

// Existing extension-only Pi packages retain their prior file lifecycle.
// Skill/MCP packages use the existing shared native-project mutation kernel.
func (adapter *ProjectAdapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := adapter.PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if request.Delivery.ClientID != domain.ClientPi || request.Delivery.ActivePath != request.Plan.ActivePath {
		return domain.ActivationOutcome{}, fmt.Errorf("pi activation differs from selected delivery")
	}
	objects := piScopedNative(request.Delivery.NativeObjects, request.Plan.Scope)
	previous := piScopedNative(request.PreviousNativeObjects, request.Plan.Scope)
	if len(objects) == 0 && len(previous) == 0 {
		return adapter.Adapter.Activate(ctx, env, request)
	}
	if request.ActivationComplete {
		return domain.ActivationOutcome{}, fmt.Errorf("pi runtime activation cannot be attested by the installer")
	}
	if _, err := adapter.ProjectActiveNative(ctx, request.Delivery.ActivePath, domain.PackageEnvelope{}, request.Plan, ""); err != nil {
		return domain.ActivationOutcome{}, err
	}
	layout, err := piComponents(request.Plan.NativeRegistryRoot, request.Plan.Scope)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	action := piUserComponentAction
	if request.Plan.Scope == domain.ScopeProject {
		action = piProjectComponentAction
	}
	out, err := shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic:         request.Plan.InstallIntent == domain.InstallIntentAutomatic,
		UnavailableAction: action,
		RepairAction:      "repair exact owned Pi components",
		RetryAction:       "retry exact Pi Skills/MCP installation",
		CompletedAction:   action,
		Verify: func() error {
			return layout.Verify(objects, false, env.NativeConfig)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			servers, err := piDesiredServers(request.Delivery.ActivePath, objects, layout.MCPKind)
			if err != nil {
				return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
			}
			return (shared.ProjectComponentMutation{
				Layout: layout, ActivePath: request.Delivery.ActivePath,
				Previous: previous, Desired: objects,
				Servers: servers, Kernel: env.NativeConfig,
			}).Apply(ctx)
		},
	})
	if err == nil && out.Verification == domain.VerificationInstalled {
		// File ownership is not a live Pi session, including user scope.
		out.Activation, out.ActivationAttested = domain.ActivationPrepared, false
	}
	return out, err
}

func (adapter *ProjectAdapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	out := shared.StartedDeactivation()
	if err := ctx.Err(); err != nil {
		return out, err
	}
	switch request.TargetContext.Scope {
	case domain.ScopeProject:
		if _, err := projectPiRoot(request.Client, request.TargetContext, request.NativeRegistryRoot); err != nil {
			return out, err
		}
	case domain.ScopeUser:
		if request.NativeRegistryRoot != request.Client.ConfigRoot {
			return out, fmt.Errorf("pi user profile differs from original authority")
		}
		if err := validateProfile(request.NativeRegistryRoot); err != nil {
			return out, err
		}
	default:
		return out, fmt.Errorf("unsupported Pi removal scope")
	}
	objects := piScopedNative(request.NativeObjects, request.TargetContext.Scope)
	if len(objects) == 0 {
		// Old extension-only user/project receipts retain their exact legacy
		// file-removal path and no new native-effects or user actions.
		return adapter.Adapter.Deactivate(ctx, env, request)
	}
	layout, err := piComponents(request.NativeRegistryRoot, request.TargetContext.Scope)
	if err != nil {
		return out, err
	}
	if err := layout.ValidateObjects(objects); err != nil {
		return out, err
	}
	if !request.Confirmed {
		out.UserActions = []string{"agentplugins will remove only its owned Pi Skills/MCP members and managed package"}
		return out, nil
	}
	if len(objects) > 0 {
		if err := (shared.ProjectComponentMutation{
			Layout:   layout,
			Previous: objects, Kernel: env.NativeConfig,
		}).Apply(ctx); err != nil {
			// A rejected foreign edit is proven to have caused no native
			// mutation. Let the common removal attempt roll back its pending
			// intent so the owner can restore the original bytes and retry.
			// Unknown or partly committed effects still require recovery.
			var observed *shared.NativeEffectError
			if errors.As(err, &observed) && observed.Effect == domain.NativeEffectUnchanged {
				out.NativeEffect = domain.NativeEffectUnchanged
				return out, err
			}
			if !shared.CommittedNativeDeactivationCleanup(&out, err) {
				return out, err
			}
			out.NativeEffect = domain.NativeEffectCommitted
		} else {
			out.NativeEffect = domain.NativeEffectCommitted
		}
	} else {
		out.NativeEffect = domain.NativeEffectUnchanged
	}
	legacy, err := adapter.Adapter.Deactivate(ctx, env, request)
	if err != nil {
		return out, err
	}
	// The legacy extension path proves only directory removal authority.
	// Preserve the independent Skills/MCP effect and any degraded-cleanup
	// action so the common removal kernel can safely complete or reconcile.
	legacy.NativeEffect = out.NativeEffect
	legacy.UserActions = append(out.UserActions, legacy.UserActions...)
	legacy.UserActions = append(legacy.UserActions, "Restart Pi to clear previously loaded Skills and MCP connections.")
	return legacy, nil
}

func piDesiredServers(root string, objects []domain.NativeObjectOwnership, mcpKind string) (map[string]nativeconfig.Server, error) {
	servers := make(map[string]nativeconfig.Server)
	for _, object := range objects {
		if object.Kind != mcpKind {
			continue
		}
		server, err := piProjectServer(root, object.LogicalName)
		if err != nil {
			return nil, err
		}
		servers[object.LogicalName] = server
	}
	return servers, nil
}
