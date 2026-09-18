package contracttest

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// RunPlanRefiner asserts the planning half of the contract. A refiner speaks
// last, on a plan the generic pipeline has already shaped, and it may only add
// to it:
//
//   - the plan identity is the planner's: ClientID, Scope, PhysicalArtifactID,
//     TargetRoot and ActivePath come back unchanged;
//   - an unsupported plan is never promoted, because readiness is an upgrade
//     over a usable plan and not a way to overrule the generic verdict;
//   - refining an already refined plan changes nothing, because a caller that
//     revisits a target refines the same plan again.
//
// What each client adds is frozen by the planner's golden files instead.
func RunPlanRefiner(t *testing.T, adapter clients.Adapter) {
	t.Helper()
	RunAdapter(t, adapter)
	for _, violation := range planRefinerViolations(adapter) {
		t.Error(violation)
	}
}

func planRefinerViolations(adapter clients.Adapter) []string {
	refiner, ok := adapter.(clients.PlanRefiner)
	if !ok {
		return []string{"adapter does not implement clients.PlanRefiner"}
	}
	violations := []string{}
	for _, fixture := range refinerFixtures(adapter.ID()) {
		violations = append(violations, fixtureViolations(refiner, fixture)...)
	}
	return violations
}

// refinerPlan is one plan the harness hands to a refiner, named for the state
// it represents.
type refinerPlan struct {
	name  string
	input clients.PlanInput
	plan  domain.DeliveryPlan
}

func refinerFixtures(id domain.ClientID) []refinerPlan {
	input := clients.PlanInput{
		Envelope: refinerEnvelope(),
		Client:   refinerClient(id),
		Detected: refinerDetection(),
	}
	viable := domain.DeliveryPlan{
		ClientID: id, Scope: domain.ScopeUser,
		Status: domain.PlanManualActivationRequired, Activation: domain.ActivationManual,
		Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed,
		Verification:       domain.VerificationPackageValid,
		PhysicalArtifactID: "demo-0123456789ab",
		DeclaredName:       "demo", DeclaredVersion: "1.0.0",
		NativeRegistryRoot: "/agentplugins-contract/config",
		Components: []domain.ComponentDecision{
			{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative},
			{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportNative},
		},
		TargetAnchor: "/agentplugins-contract/anchor",
		TargetRoot:   "/agentplugins-contract/anchor/root",
		ActivePath:   "/agentplugins-contract/anchor/root/demo-0123456789ab",
	}
	unsupported := clonePlan(viable)
	unsupported.Status, unsupported.Activation = domain.PlanUnsupported, domain.ActivationFailed
	return []refinerPlan{
		{name: "viable plan", input: input, plan: viable},
		{name: "unsupported plan", input: input, plan: unsupported},
	}
}

func fixtureViolations(refiner clients.PlanRefiner, fixture refinerPlan) []string {
	violations := []string{}
	refined := clonePlan(fixture.plan)
	if err := refiner.RefinePlan(context.Background(), fixture.input, &refined); err != nil {
		return []string{fmt.Sprintf("%s: refining failed: %v", fixture.name, err)}
	}
	violations = append(violations, identityViolations(fixture, refined)...)
	if fixture.plan.Status == domain.PlanUnsupported && refined.Status != domain.PlanUnsupported {
		violations = append(violations, fmt.Sprintf("%s: refined to status %q; a refiner may not overrule the generic verdict", fixture.name, refined.Status))
	}
	again := clonePlan(refined)
	if err := refiner.RefinePlan(context.Background(), fixture.input, &again); err != nil {
		return append(violations, fmt.Sprintf("%s: refining an already refined plan failed: %v", fixture.name, err))
	}
	if !reflect.DeepEqual(refined, again) {
		violations = append(violations, fmt.Sprintf("%s: refining twice differs from refining once; a refiner has to be idempotent", fixture.name))
	}
	return violations
}

func identityViolations(fixture refinerPlan, refined domain.DeliveryPlan) []string {
	violations := []string{}
	for _, field := range []struct {
		name          string
		before, after string
	}{
		{"ClientID", string(fixture.plan.ClientID), string(refined.ClientID)},
		{"Scope", string(fixture.plan.Scope), string(refined.Scope)},
		{"PhysicalArtifactID", fixture.plan.PhysicalArtifactID, refined.PhysicalArtifactID},
		{"TargetRoot", fixture.plan.TargetRoot, refined.TargetRoot},
		{"ActivePath", fixture.plan.ActivePath, refined.ActivePath},
	} {
		if field.before != field.after {
			violations = append(violations, fmt.Sprintf("%s: changed %s from %q to %q; the plan identity belongs to the planner", fixture.name, field.name, field.before, field.after))
		}
	}
	return violations
}

// refinerEnvelope is a package with one of everything a refiner might read.
func refinerEnvelope() domain.PackageEnvelope {
	return domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "demo", Version: "1.0.0"},
		Skills:   map[string]domain.Skill{"docs": {Name: "docs"}},
		MCP: domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
			"local": {Name: "local", Type: "stdio"},
		}},
	}
}

// refinerDetection reports every client as present, so an adapter that reads a
// backend sibling finds one.
func refinerDetection() map[domain.ClientID]domain.DetectedClient {
	detected := make(map[domain.ClientID]domain.DetectedClient)
	for _, id := range domain.SupportedClientIDs() {
		detected[id] = refinerClient(id)
	}
	return detected
}

func refinerClient(id domain.ClientID) domain.DetectedClient {
	definition, _ := domain.ClientDefinitionFor(id)
	return domain.DetectedClient{
		ClientID: id, DisplayName: definition.DisplayName, Status: domain.DetectionDetected,
		ConfigRoot:     "/agentplugins-contract/config/" + string(id),
		ExecutablePath: "/agentplugins-contract/bin/" + string(id),
	}
}

// clonePlan copies the slices a refiner appends to, so one fixture run cannot
// observe another's additions through shared backing arrays.
func clonePlan(plan domain.DeliveryPlan) domain.DeliveryPlan {
	plan.Components = append([]domain.ComponentDecision(nil), plan.Components...)
	plan.UserActions = append([]string(nil), plan.UserActions...)
	plan.LocalActions = append([]string(nil), plan.LocalActions...)
	plan.Warnings = append([]string(nil), plan.Warnings...)
	plan.Diagnostics = append([]domain.Diagnostic(nil), plan.Diagnostics...)
	return plan
}
