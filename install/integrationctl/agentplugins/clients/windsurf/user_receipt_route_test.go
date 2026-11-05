package windsurf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestRecordedCascadeMCPReplaysThroughSelectedDevinWithoutMigration(t *testing.T) {
	client, plan, _ := globalOfflineFixture(t)
	globalPath := filepath.Join(client.NativeConfigRoot, "mcp_config.json")
	globalBefore, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatal(err)
	}
	plan.NativeRegistryRoot = client.ConfigRoot
	plan.NativeRegistryExecutable = client.ExecutablePath
	plan.Components = []domain.ComponentDecision{
		{Kind: domain.ComponentMCPServer, Name: "old-cascade", Support: domain.SupportPrepared},
	}
	globalOfflineWrite(t, filepath.Join(plan.ActivePath, "mcp.json"), `{"mcpServers":{"old-cascade":{"type":"sse","url":"https://legacy.example/mcp"}}}`)
	configPath := filepath.Join(client.ConfigRoot, "mcp_config.json")
	globalOfflineWrite(t, configPath, `{"mcpServers":{"foreign":{"serverUrl":"https://foreign.example/mcp"}},"legacy":true}`)
	objects, err := BuildWindsurfNativeObjects(plan.ActivePath, plan)
	if err != nil || len(objects) != 1 {
		t.Fatalf("legacy receipt staging: %v, %+v", err, objects)
	}
	if err := ApplyWindsurfNativeMutation(client.ConfigRoot, plan.ActivePath, nil, objects); err != nil {
		t.Fatalf("seed real Cascade receipt: %v", err)
	}
	input := clients.PlanInput{
		TargetContext: plan.TargetContext, Client: client,
		PreviousBindingRecorded: true, PreviousNativeProfileRoot: client.ConfigRoot,
		PreviousNativeObjects: objects,
	}
	adapter := New()
	root, executable := adapter.NativeRegistry(input)
	if root != client.ConfigRoot || executable != client.ExecutablePath {
		t.Fatalf("legacy receipt switched to selected Devin registry: %q, %q", root, executable)
	}
	if err := adapter.CheckPlanPrecondition(input, &plan); err != nil {
		t.Fatal(err)
	}
	if err := adapter.RefinePlan(context.Background(), input, &plan); err != nil || isDevinGlobalPlan(plan) {
		t.Fatalf("legacy Cascade plan was promoted to Devin Global: %v", err)
	}
	managed := &domain.ClientBinding{
		ClientID: string(domain.ClientWindsurf), Scope: string(domain.ScopeUser),
		NativeProfileRoot: client.ConfigRoot, NativeObjects: objects,
	}
	env := clients.Env{NativeConfig: nativeconfig.New()}
	finding, err := adapter.InspectNativeRegistry(context.Background(), env, client, plan, managed)
	if err != nil || finding != clients.RegistryExpected {
		t.Fatalf("fresh-process legacy registry inspection: %v %v", finding, err)
	}
	request := domain.ActivationRequest{
		Client: client, Plan: plan, PreviousNativeObjects: objects, VerifyOnly: true,
		Delivery: domain.StagedDelivery{ClientID: domain.ClientWindsurf, ActivePath: plan.ActivePath, NativeObjects: objects},
	}
	if err := adapter.PreflightActivation(env, request); err != nil {
		t.Fatalf("legacy preflight incorrectly demanded Devin Global: %v", err)
	}
	outcome, err := adapter.Activate(context.Background(), env, request)
	if err != nil || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("legacy MCP verify replay: %+v, %v", outcome, err)
	}
	request.VerifyOnly = false
	outcome, err = adapter.Activate(context.Background(), env, request)
	if err != nil || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("legacy MCP repair replay: %+v, %v", outcome, err)
	}
	removed, err := adapter.Deactivate(context.Background(), env, domain.DeactivationRequest{
		Client: client, TargetContext: plan.TargetContext, NativeRegistryRoot: client.ConfigRoot,
		NativeObjects: objects, Confirmed: true,
	})
	if err != nil || !removed.ExternalRemovalComplete {
		t.Fatalf("legacy receipt removal: %+v, %v", removed, err)
	}
	legacyAfter, err := os.ReadFile(configPath)
	if err != nil || !strings.Contains(string(legacyAfter), "foreign") ||
		strings.Contains(string(legacyAfter), "old-cascade") {
		t.Fatalf("Cascade owned-only remove: %v, %s", err, legacyAfter)
	}
	globalAfter, err := os.ReadFile(globalPath)
	if err != nil || string(globalBefore) != string(globalAfter) {
		t.Fatal("legacy replay modified original Devin Global profile")
	}
}

func TestRecordedPackageOnlyCascadeDoesNotAdoptSelectedDevinGlobal(t *testing.T) {
	client, plan, _ := globalOfflineFixture(t)
	for _, recorded := range []string{"", client.ConfigRoot} {
		input := clients.PlanInput{
			TargetContext: plan.TargetContext, Client: client,
			PreviousBindingRecorded: true, PreviousNativeProfileRoot: recorded,
		}
		root, executable := New().NativeRegistry(input)
		if root != client.ConfigRoot || executable != client.ExecutablePath {
			t.Fatalf("package-only receipt %q was silently migrated to %q", recorded, root)
		}
		legacyPlan := plan
		legacyPlan.NativeRegistryRoot, legacyPlan.NativeRegistryExecutable = root, executable
		if err := New().CheckPlanPrecondition(input, &legacyPlan); err != nil {
			t.Fatal(err)
		}
		if err := New().RefinePlan(context.Background(), input, &legacyPlan); err != nil ||
			isDevinGlobalPlan(legacyPlan) {
			t.Fatalf("package-only receipt was upgraded: %v", err)
		}
	}
}

func TestRecordedMixedOrRetargetedUserReceiptFailsClosed(t *testing.T) {
	client, plan, _ := globalOfflineFixture(t)
	legacy := domain.NativeObjectOwnership{Kind: WindsurfMCPObjectKind}
	global := domain.NativeObjectOwnership{Kind: DevinGlobalMCPKind}
	for _, scenario := range []struct {
		root    string
		objects []domain.NativeObjectOwnership
	}{
		{client.ConfigRoot, []domain.NativeObjectOwnership{legacy, global}},
		{client.NativeConfigRoot, []domain.NativeObjectOwnership{legacy}},
		{client.ConfigRoot, []domain.NativeObjectOwnership{global}},
		{filepath.Join(filepath.Dir(client.NativeConfigRoot), "stranger"), nil},
	} {
		input := clients.PlanInput{
			TargetContext: plan.TargetContext, Client: client,
			PreviousBindingRecorded: true, PreviousNativeProfileRoot: scenario.root,
			PreviousNativeObjects: scenario.objects,
		}
		guardPlan := plan
		guardPlan.NativeRegistryRoot, guardPlan.NativeRegistryExecutable = New().NativeRegistry(input)
		if err := New().CheckPlanPrecondition(input, &guardPlan); err == nil {
			t.Fatalf("retargeted or mixed user receipt was accepted: %+v", scenario)
		}
	}
}
