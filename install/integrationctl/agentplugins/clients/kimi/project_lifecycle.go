package kimi

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const KimiProjectAction = "Kimi Code project components are configured. Review folder trust and the selected working directory, start a fresh session and inspect skills/MCP. Effective extra skill roots, disabled/session selection, native discovery, MCP initialization and authentication remain unobserved; existing sessions retain removed MCP tools until restarted."

func preflightKimiProject(env clients.Env, r domain.ActivationRequest) error {
	root, err := kimiPlanRoot(r.Client, r.Plan)
	if err != nil {
		return err
	}
	if r.Client.ClientID != domain.ClientKimi || (r.Delivery.ClientID != "" && r.Delivery.ClientID != domain.ClientKimi) {
		return fmt.Errorf("kimi activation client mismatch")
	}
	if r.Delivery.ActivePath != "" && r.Delivery.ActivePath != r.Plan.ActivePath {
		return fmt.Errorf("kimi delivery differs from selected artifact")
	}
	if r.ActivationComplete {
		return fmt.Errorf("kimi project trust and native discovery cannot be established by activation attestation")
	}
	if err := currentKimi(r.Client); err != nil {
		return err
	}
	if shared.ComponentKindPresent(r.Plan.Components, domain.ComponentSkill) {
		if err := kimiSkillRoot(r.Plan.TargetContext.ProjectRoot); err != nil {
			return err
		}
	}
	if !shared.OnlyNativeComponents(r.Plan.Components) {
		return fmt.Errorf("kimi project delivery requires supported skills and MCP only")
	}
	layout := kimiComponents(root)
	if err := layout.ValidateObjects(kimiNativeObjects(r.PreviousNativeObjects)); err != nil {
		return err
	}
	if err := layout.ValidateObjects(kimiNativeObjects(r.Delivery.NativeObjects)); err != nil {
		return err
	}
	if err := env.NativeConfig.RequireFileIO(); err != nil {
		return err
	}
	if shared.ComponentKindPresent(r.Plan.Components, domain.ComponentMCPServer) {
		_, err := kimiConfigNames(filepath.Join(root, "mcp.json"))
		return err
	}
	return nil
}

func activateKimiProject(ctx context.Context, env clients.Env, r domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := shared.ActivationIdentityMismatch(r); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if r.Delivery.ActivePath != r.Plan.ActivePath {
		return domain.ActivationOutcome{}, fmt.Errorf("kimi delivery differs from selected artifact")
	}
	if err := preflightKimiProject(env, r); err != nil {
		return domain.ActivationOutcome{}, err
	}
	layout := kimiComponents(r.Plan.NativeRegistryRoot)
	out, err := shared.CompleteNativeConfigActivation(ctx, r, shared.NativeConfigActivation{
		Automatic:         r.Plan.InstallIntent == domain.InstallIntentAutomatic,
		UnavailableAction: "select supported Kimi project components", RepairAction: "repair exact Kimi project entries", RetryAction: "retry Kimi project component installation", CompletedAction: KimiProjectAction,
		Verify: func() error {
			return layout.Verify(kimiNativeObjects(r.Delivery.NativeObjects), false, env.NativeConfig)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			servers, err := kimiDesiredServers(request.Delivery.ActivePath, kimiNativeObjects(request.Delivery.NativeObjects))
			if err != nil {
				return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
			}
			return (shared.ProjectComponentMutation{Layout: layout, ActivePath: request.Delivery.ActivePath, Previous: kimiNativeObjects(request.PreviousNativeObjects), Desired: kimiNativeObjects(request.Delivery.NativeObjects), Servers: servers, Kernel: env.NativeConfig}).Apply(ctx)
		},
	})
	if err == nil && out.Verification == domain.VerificationInstalled {
		out.Activation, out.ActivationAttested = domain.ActivationPrepared, false
	}
	return out, err
}

func deactivateKimiProject(ctx context.Context, env clients.Env, r domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	out := shared.StartedDeactivation()
	if err := ctx.Err(); err != nil {
		return out, err
	}
	plan := domain.DeliveryPlan{Scope: domain.ScopeProject, TargetContext: r.TargetContext, ClientID: domain.ClientKimi, PhysicalArtifactID: r.PhysicalArtifactID, ActivePath: r.ManagedArtifactPath, NativeRegistryRoot: r.NativeRegistryRoot}
	root, err := kimiPlanRoot(r.Client, plan)
	if err != nil {
		return out, err
	}
	if r.Client.ClientID != domain.ClientKimi {
		return out, fmt.Errorf("kimi deactivation client mismatch")
	}
	layout := kimiComponents(root)
	if err := layout.ValidateObjects(kimiNativeObjects(r.NativeObjects)); err != nil {
		return out, err
	}
	if !r.Confirmed {
		out.UserActions = []string{"agentplugins will remove only exact owned Kimi project skill directories and MCP members"}
		return out, nil
	}
	if err := (shared.ProjectComponentMutation{Layout: layout, Previous: kimiNativeObjects(r.NativeObjects), Kernel: env.NativeConfig}).Apply(ctx); err != nil && !shared.CommittedNativeDeactivationCleanup(&out, err) {
		return out, err
	}
	out.ExternalRemovalComplete = true
	out.UserActions = []string{"Start a fresh Kimi session to observe removal; already-open sessions can retain removed MCP tools"}
	return out, nil
}
