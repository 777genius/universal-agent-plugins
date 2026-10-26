package qoder

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ActivationPreflighter     = (*Adapter)(nil)
	_ clients.PreparedRegistryInspector = (*Adapter)(nil)
)

func projectActions(plan domain.DeliveryPlan) []string {
	view := filepath.Join(plan.ActivePath, projectNativeView)
	command, _ := json.Marshal(struct {
		WorkingDirectory string   `json:"working_directory"`
		Argv             []string `json:"argv"`
	}{WorkingDirectory: plan.TargetContext.ProjectRoot, Argv: []string{"qoder", "plugins", "install", view, "--scope", "project", "--json"}})
	return []string{
		"Qoder project skill bundle is prepared at " + view + "; native discovery, loaded selection, trust, authentication and MCP are not established.",
		"Optional separate native import (UAP does not execute this command): " + string(command),
		"Review existing native registrations and cache first: native import writes the user's shared native registry/cache, and same-name/same-version replacement remains unqualified.",
		"A manually imported native cached copy is independent of this UAP bundle. UAP removal removes only its project bundle; it does not unregister or delete that native copy.",
	}
}

func (*Adapter) PreflightActivation(_ clients.Env, request domain.ActivationRequest) error {
	if !projectRequested(request.Plan) {
		return nil
	}
	if err := validateProjectPlan(request.Client, request.Plan); err != nil {
		return err
	}
	if request.Plan.Status == domain.PlanUnsupported {
		return fmt.Errorf("qoder project package is unsupported")
	}
	if request.ActivationComplete {
		return fmt.Errorf("qoder project preparation cannot be promoted by activation attestation")
	}
	if request.Delivery.ClientID != "" && request.Delivery.ClientID != domain.ClientQoder {
		return fmt.Errorf("qoder project delivery client differs from plan")
	}
	if (request.Delivery.ActivePath != "" && request.Delivery.ActivePath != request.Plan.ActivePath) || (request.Delivery.OwnedBase != "" && request.Delivery.OwnedBase != request.Plan.TargetRoot) || (request.DeclaredName != "" && request.DeclaredName != request.Plan.DeclaredName) {
		return fmt.Errorf("qoder project delivered identity differs from plan")
	}
	_, err := selectedProjectSkills(request.Plan)
	return err
}

func activateProject(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := New().PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if request.Delivery.ClientID != domain.ClientQoder || request.Delivery.ActivePath != request.Plan.ActivePath || request.Delivery.OwnedBase != request.Plan.TargetRoot || request.DeclaredName != request.Plan.DeclaredName {
		return domain.ActivationOutcome{}, fmt.Errorf("qoder project delivered identity differs from plan")
	}
	if err := verifyProjectBundle(ctx, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	outcome.Activation, outcome.Verification = domain.ActivationPrepared, domain.VerificationInstalled
	outcome.NativeEffect = domain.NativeEffectUnchanged
	outcome.ActivationAttested = false
	outcome.UserActions = projectActions(request.Plan)
	return outcome, nil
}

func deactivateProject(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if err := ctx.Err(); err != nil {
		return outcome, err
	}
	root, err := projectRoot(request.Client, request.TargetContext)
	if err != nil {
		return outcome, err
	}
	if request.NativeRegistryRoot != root || !request.SelectedDelivery.IsZero() {
		return outcome, fmt.Errorf("qoder project removal profile differs from selected namespace")
	}
	binding := domain.ClientBinding{ClientID: "qoder", Scope: "project", NativeProfileRoot: root, PhysicalArtifact: request.PhysicalArtifactID, TargetLocator: request.ManagedArtifactPath, NativeObjects: request.NativeObjects}
	if err := New().ValidateBindingProfile(root, binding); err != nil {
		return outcome, err
	}
	if !request.Confirmed {
		outcome.UserActions = []string{"agentplugins will remove only its exact prepared Qoder project bundle"}
		return outcome, nil
	}
	outcome.ExternalRemovalComplete = true
	outcome.UserActions = []string{"Any separately imported Qoder cached copy remains native state; UAP did not unregister or delete it."}
	return outcome, nil
}

func (*Adapter) InspectPreparedRegistry(plan domain.DeliveryPlan, name string, owned bool) (clients.RegistryFinding, error) {
	if projectRequested(plan) {
		if err := validateProjectPlan(domain.DetectedClient{ClientID: domain.ClientQoder, ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
			return clients.RegistryIndeterminate, err
		}
		if name != plan.DeclaredName {
			return clients.RegistryIndeterminate, fmt.Errorf("qoder prepared name differs from plan")
		}
	}
	return shared.InspectUnqualifiedPluginRoot(plan.TargetRoot, name, plan.ActivePath, owned)
}
