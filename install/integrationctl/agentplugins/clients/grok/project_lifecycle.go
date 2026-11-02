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

const projectPreparationAction = "Grok project plugin is prepared; review the selected project and plugin in Grok, make any trust decision there, and separately verify native discovery; skills execution and bundled MCP remain unqualified"

func (*Adapter) PreflightActivation(_ clients.Env, request domain.ActivationRequest) error {
	if request.Plan.Scope != domain.ScopeProject && request.Plan.TargetContext.Scope != domain.ScopeProject && request.Plan.TargetContext.ProjectRoot == "" {
		return nil
	}
	if _, err := grokPlanRoot(request.Client, request.Plan); err != nil {
		return err
	}
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
	if !shared.ComponentKindPresent(request.Plan.Components, domain.ComponentSkill) {
		return fmt.Errorf("grok project preparation requires a supported skill")
	}
	for _, component := range request.Plan.Components {
		if component.Support != domain.SupportUnsupported && component.Kind != domain.ComponentSkill {
			return fmt.Errorf("grok project component is unqualified")
		}
	}
	return nil
}
func activateProject(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := New().PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if request.Delivery.ClientID != domain.ClientGrok || request.Delivery.ActivePath != request.Plan.ActivePath || request.Delivery.OwnedBase != request.Plan.TargetRoot || request.DeclaredName != request.Plan.DeclaredName {
		return domain.ActivationOutcome{}, fmt.Errorf("grok project delivered identity differs from plan")
	}
	finding, err := inspectProject(ctx, env, request.Client, request.Plan, true)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	if finding != clients.RegistryExpected {
		return domain.ActivationOutcome{}, fmt.Errorf("grok prepared project plugin source is absent, foreign or indeterminate")
	}
	outcome := shared.StartedActivation(request)
	outcome.Activation = domain.ActivationPrepared
	outcome.UserActions = []string{projectPreparationAction}
	return outcome, nil
}
func deactivateProject(_ context.Context, _ clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
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
	if !request.Confirmed {
		outcome.UserActions = []string{"agentplugins will remove only its exact prepared Grok project plugin directory"}
		return outcome, nil
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
