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

// RefinePlan makes only the explicitly selected project-native component
// selection ready. An extension-only project remains valid; an unsupported
// portable component never becomes an implied native capability.
func (adapter *ProjectAdapter) RefinePlan(ctx context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if plan.Scope != domain.ScopeProject {
		return adapter.Adapter.RefinePlan(ctx, in, plan)
	}
	if _, err := projectPiRoot(in.Client, plan.TargetContext, plan.NativeRegistryRoot); err != nil {
		return err
	}
	found := false
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		switch component.Kind {
		case domain.ComponentSkill, domain.ComponentMCPServer:
			found = true
		case domain.ComponentExtension:
			if component.Name != Namespace {
				return fmt.Errorf("unknown Pi project extension")
			}
			found = true
		default:
			return fmt.Errorf("unqualified Pi project component")
		}
	}
	shared.PromoteNativeReady(plan, plan.NativeRegistryRoot, found)
	plan.UserActions = shared.AppendUnique(plan.UserActions, piProjectComponentAction)
	return nil
}

func (adapter *ProjectAdapter) InspectNativeRegistry(ctx context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if plan.Scope != domain.ScopeProject {
		return adapter.Adapter.InspectNativeRegistry(ctx, env, client, plan, managed)
	}
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if _, err := projectPiRoot(client, plan.TargetContext, plan.NativeRegistryRoot); err != nil {
		return clients.RegistryIndeterminate, err
	}
	found, err := inspectPackage(plan, managed != nil && managed.TargetLocator == plan.ActivePath)
	if err != nil || found == clients.RegistryCollision || found == clients.RegistryIndeterminate {
		return found, err
	}
	var prior *domain.ClientBinding
	if managed != nil {
		binding := *managed
		binding.NativeObjects = piProjectOnlyNative(managed.NativeObjects)
		prior = &binding
	}
	selected := make([]domain.ComponentDecision, 0)
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentSkill || component.Kind == domain.ComponentMCPServer {
			selected = append(selected, component)
		}
	}
	if len(selected) == 0 && prior == nil {
		return found, nil
	}
	native, err := piProjectComponents(plan.NativeRegistryRoot).Inspect(selected, prior, env.NativeConfig)
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
	if request.Plan.Scope != domain.ScopeProject {
		return adapter.Adapter.Activate(ctx, env, request)
	}
	if err := ctx.Err(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := adapter.Adapter.PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if request.ActivationComplete {
		return domain.ActivationOutcome{}, fmt.Errorf("pi trust or runtime activation cannot be attested by the installer")
	}
	if request.Delivery.ClientID != domain.ClientPi || request.Delivery.ActivePath != request.Plan.ActivePath {
		return domain.ActivationOutcome{}, fmt.Errorf("pi project activation differs from selected delivery")
	}
	objects := piProjectOnlyNative(request.Delivery.NativeObjects)
	previous := piProjectOnlyNative(request.PreviousNativeObjects)
	if len(objects) == 0 && len(previous) == 0 {
		return adapter.Adapter.Activate(ctx, env, request)
	}
	if _, err := adapter.ProjectActiveNative(ctx, request.Delivery.ActivePath, domain.PackageEnvelope{}, request.Plan, ""); err != nil {
		return domain.ActivationOutcome{}, err
	}
	layout := piProjectComponents(request.Plan.NativeRegistryRoot)
	out, err := shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic: request.Plan.InstallIntent == domain.InstallIntentAutomatic,
		UnavailableAction: piProjectComponentAction,
		RepairAction: "repair exact owned Pi project components",
		RetryAction: "retry exact Pi project Skills/MCP installation",
		CompletedAction: piProjectComponentAction,
		Verify: func() error {
			return layout.Verify(objects, false, env.NativeConfig)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			servers, err := piDesiredServers(request.Delivery.ActivePath, objects)
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
		out.Activation, out.ActivationAttested = domain.ActivationPrepared, false
	}
	return out, err
}

func (adapter *ProjectAdapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if request.TargetContext.Scope != domain.ScopeProject {
		return adapter.Adapter.Deactivate(ctx, env, request)
	}
	out := shared.StartedDeactivation()
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if _, err := projectPiRoot(request.Client, request.TargetContext, request.NativeRegistryRoot); err != nil {
		return out, err
	}
	objects := piProjectOnlyNative(request.NativeObjects)
	if err := piProjectComponents(request.NativeRegistryRoot).ValidateObjects(objects); err != nil {
		return out, err
	}
	if !request.Confirmed {
		out.UserActions = []string{"agentplugins will remove only its own Pi project Skills/MCP members and managed package"}
		return out, nil
	}
	if len(objects) > 0 {
		if err := (shared.ProjectComponentMutation{
			Layout: piProjectComponents(request.NativeRegistryRoot),
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
	legacy.UserActions = append(legacy.UserActions, "Restart Pi to clear previously loaded project Skills and MCP connections.")
	return legacy, nil
}

func piDesiredServers(root string, objects []domain.NativeObjectOwnership) (map[string]nativeconfig.Server, error) {
	servers := make(map[string]nativeconfig.Server)
	for _, object := range objects {
		if object.Kind != piProjectMCPKind {
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
