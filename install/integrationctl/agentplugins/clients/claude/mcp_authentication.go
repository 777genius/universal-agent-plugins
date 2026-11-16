package claude

import (
	"context"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

// Exact enabled plugin ownership and the verified delivered Skill bytes prove
// installation setup. They do not establish per-Skill session discovery or the
// credentials a future model/tool request may need.
func qualifyClaudeNativeSkillAuthentication(plan domain.DeliveryPlan, outcome domain.ActivationOutcome) domain.ActivationOutcome {
	if outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled ||
		outcome.Policy != domain.PolicyAllowed || outcome.Authentication != domain.AuthenticationNotChecked || plan.Authentication != domain.AuthenticationNotChecked ||
		outcome.ActivationAttested || outcome.AuthenticationAttested {
		return outcome
	}
	outcome.Authentication = shared.ObservedSkillInstallationAuthentication(plan)
	return outcome
}

// Older native health output omits Type. Its connected status remains useful,
// but local authentication qualification then requires the exact selected entry
// in the unchanged, verified delivery; absence of Type alone proves nothing.
func claudeMCPIsLocal(ctx context.Context, request domain.ActivationRequest, name string, observation claudeMCPObservation) bool {
	if observation.transport != "" {
		return observation.transport == "stdio"
	}
	root, expected := request.Delivery.ActivePath, request.Delivery.ArtifactDigest
	if !exactAbsolute(root) || root != request.Plan.ActivePath || expected == "" {
		return false
	}
	digest, err := (packagesnapshot.Builder{}).Digest(ctx, root)
	if err != nil || digest != expected {
		return false
	}
	layout, err := New().ManagedMCPSelectionForArtifact(request.Plan.Scope, root)
	if err != nil {
		return false
	}
	document, err := readClaudeJSONObject(root, layout.File)
	if err != nil {
		return false
	}
	server, ok := document[name].(map[string]any)
	if !ok {
		return false
	}
	command, ok := server["command"].(string)
	if !ok || strings.TrimSpace(command) == "" {
		return false
	}
	if transport, exists := server["type"]; exists && transport != "stdio" {
		return false
	}
	if _, remote := server["url"]; remote {
		return false
	}
	digest, err = (packagesnapshot.Builder{}).Digest(ctx, root)
	return err == nil && digest == expected
}

// A connected local MCP transport proves local installation setup, separately
// from any credentials its future tools or a model session might require.
// The planner's warning is emitted only when catalog evidence is absent.
func qualifyClaudeNativeAuthentication(plan domain.DeliveryPlan, outcome domain.ActivationOutcome, allLocal bool) domain.ActivationOutcome {
	if !allLocal || outcome.Activation != domain.ActivationActive || outcome.Policy != domain.PolicyAllowed || (outcome.Authentication != "" && outcome.Authentication != domain.AuthenticationNotChecked) || !slices.Contains(plan.Warnings, "authentication_not_catalog_verified") {
		return outcome
	}
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if component.Support != domain.SupportNative && component.Support != domain.SupportProjected {
			return outcome
		}
		if component.Kind != domain.ComponentSkill && component.Kind != domain.ComponentMCPServer {
			return outcome
		}
	}
	outcome.Authentication = domain.AuthenticationNotRequired
	return outcome
}
