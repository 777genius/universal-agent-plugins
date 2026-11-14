package grok

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ActivationPreflighter = (*Adapter)(nil)

// Project guidance must describe only components actually delivered. A
// Skills-only project remains qualified on Grok 1.0.67.
func projectPreparationAction(components []domain.ComponentDecision) string {
	if !shared.HasSupportedMCP(components) {
		return "Grok project Skills are prepared; open the selected project, review trust in Grok, then reload and verify native Skills discovery; UAP has not activated them"
	}
	label := "MCP"
	if shared.ComponentKindPresent(components, domain.ComponentSkill) {
		label = "Skills/MCP"
	}
	return "Grok project " + label + " delivery is prepared; Project MCP requires Grok 1.0.70; open the selected project, enable and trust the plugin in Grok, then reload and verify effective MCP discovery; UAP has not activated it"
}

func (*Adapter) PreflightActivation(env clients.Env, request domain.ActivationRequest) error {
	if request.Plan.Scope != domain.ScopeProject && request.Plan.TargetContext.Scope != domain.ScopeProject && request.Plan.TargetContext.ProjectRoot == "" {
		return nil
	}
	if request.ConfirmPluginEnable {
		return errEnablementUnqualified
	}
	if _, err := grokPlanRoot(request.Client, request.Plan); err != nil {
		return err
	}
	if err := preflightRetainedEnablement(env, request); err != nil {
		return err
	}
	return preflightProjectDelivery(request)
}

func preflightProjectDelivery(request domain.ActivationRequest) error {
	if request.Plan.Status == domain.PlanUnsupported {
		return fmt.Errorf("grok project package is unsupported")
	}
	if request.Delivery.ClientID != "" && request.Delivery.ClientID != domain.ClientGrok {
		return fmt.Errorf("grok project delivery client differs from plan")
	}
	if request.ActivationComplete {
		return fmt.Errorf("grok project preparation cannot be promoted by activation attestation")
	}
	if (request.Delivery.ActivePath != "" && request.Delivery.ActivePath != request.Plan.ActivePath) || (request.Delivery.OwnedBase != "" && request.Delivery.OwnedBase != request.Plan.TargetRoot) || (request.DeclaredName != "" && request.DeclaredName != request.Plan.DeclaredName) {
		return fmt.Errorf("grok project delivered identity differs from plan")
	}
	if !shared.OnlyNativeComponents(request.Plan.Components) {
		return fmt.Errorf("grok project preparation requires supported Skills or MCP")
	}
	for _, component := range request.Plan.Components {
		if component.Support != domain.SupportUnsupported && component.Kind != domain.ComponentSkill && component.Kind != domain.ComponentMCPServer {
			return fmt.Errorf("grok project component is unqualified")
		}
	}
	return nil
}
func activateProject(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	outcome := shared.StartedActivation(request)
	outcome.NativeEffect = domain.NativeEffectUnchanged
	outcome.Activation = domain.ActivationPrepared
	outcome.UserActions = []string{projectPreparationAction(request.Plan.Components)}
	if err := New().PreflightActivation(env, request); err != nil {
		return outcome, err
	}
	if request.Delivery.ClientID != domain.ClientGrok || request.Delivery.ActivePath != request.Plan.ActivePath || request.Delivery.OwnedBase != request.Plan.TargetRoot || request.DeclaredName != request.Plan.DeclaredName {
		return outcome, fmt.Errorf("grok project delivered identity differs from plan")
	}
	finding, err := inspectProject(ctx, env, request.Client, request.Plan, true, true)
	if err != nil {
		return outcome, err
	}
	if finding != clients.RegistryExpected {
		return outcome, fmt.Errorf("grok prepared project plugin source is absent, foreign or indeterminate")
	}
	if request.ConfirmPluginEnable && !request.VerifyOnly {
		return enableProject(ctx, env, request, outcome)
	}
	return outcome, nil
}
func deactivateProject(_ context.Context, _ clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	outcome.NativeEffect = domain.NativeEffectUnchanged
	root, err := grokProjectRoot(request.Client, request.TargetContext, request.NativeRegistryRoot)
	if err != nil {
		return outcome, err
	}
	binding := domain.ClientBinding{ClientID: "grok", Scope: "project", NativeProfileRoot: root, PhysicalArtifact: request.PhysicalArtifactID, TargetLocator: request.ManagedArtifactPath, NativeObjects: request.NativeObjects}
	if err := New().ValidateBindingProfile(root, binding); err != nil {
		return outcome, err
	}
	if filepath.Dir(request.ManagedArtifactPath) != filepath.Join(root, "plugins") {
		return outcome, fmt.Errorf("grok project removal path differs from owned directory")
	}
	if hasEnabledOwnership(binding) {
		return outcome, errEnablementUnqualified
	}
	if !request.Confirmed {
		outcome.UserActions = []string{"agentplugins will remove only its exact prepared Grok project plugin directory"}
		return outcome, nil
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
