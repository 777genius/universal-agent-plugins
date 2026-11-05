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

func globalOfflineWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func globalOfflineFixture(t *testing.T) (domain.DetectedClient, domain.DeliveryPlan, domain.PackageEnvelope) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	native := filepath.Join(base, "original-xdg", "devin")
	legacy := filepath.Join(base, ".codeium", "windsurf")
	active := filepath.Join(base, "independent-state", "managed", "clients", "windsurf", "global-artifact")
	globalOfflineWrite(t, filepath.Join(active, "skills", "global-marker", "SKILL.md"), "---\nname: global-marker\ndescription: original source\n---\nGlobal source marker\n")
	globalOfflineWrite(t, filepath.Join(active, "mcp.json"), `{"mcpServers":{"global-stdio":{"type":"stdio","command":"/bin/echo","args":["original-stdio"]},"global-http":{"type":"streamable-http","url":"https://localhost.example/mcp"}}}`)
	globalOfflineWrite(t, filepath.Join(native, "skills", "foreign-skill", "SKILL.md"), "foreign-skill unchanged\n")
	globalOfflineWrite(t, filepath.Join(native, "mcp_config.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.example/mcp","disabled":true}},"policy":"keep"}`)
	globalOfflineWrite(t, filepath.Join(legacy, "mcp_config.json"), `{"mcpServers":{"old-cascade":{"serverUrl":"https://legacy.example/mcp"}},"legacy":true}`)
	client := domain.DetectedClient{
		ClientID: domain.ClientWindsurf, Status: domain.DetectionDetected,
		ExecutablePath: filepath.Join(base, "bin", "devin"), Version: "3000.11.205",
		ConfigRoot: legacy, NativeConfigRoot: native,
	}
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientWindsurf, Scope: domain.ScopeUser,
		TargetContext: domain.TargetContext{Scope: domain.ScopeUser, UserConfigRoot: legacy},
		NativeRegistryRoot: native, NativeRegistryExecutable: "",
		TargetRoot: filepath.Dir(active), ActivePath: active, PhysicalArtifactID: "global-artifact",
		Status: domain.PlanReady, InstallIntent: domain.InstallIntentAutomatic,
		Components: []domain.ComponentDecision{
			{Kind: domain.ComponentSkill, Name: "global-marker", Support: domain.SupportPrepared},
			{Kind: domain.ComponentMCPServer, Name: "global-stdio", Support: domain.SupportPrepared},
			{Kind: domain.ComponentMCPServer, Name: "global-http", Support: domain.SupportPrepared},
		},
	}
	envelope := domain.PackageEnvelope{Skills: map[string]domain.Skill{
		"global-marker": {Name: "global-marker", RelativePath: "skills/global-marker/SKILL.md"},
	}}
	return client, plan, envelope
}

func TestDevinGlobalOfflineNativeLifecycleRelocatedStateAndLegacyParity(t *testing.T) {
	client, plan, envelope := globalOfflineFixture(t)
	objects, err := devinGlobalObjects(plan.ActivePath, envelope, plan)
	if err != nil || len(objects) != 3 {
		t.Fatalf("global projection failed: %d objects, %v", len(objects), err)
	}
	legacyFile := filepath.Join(client.ConfigRoot, "mcp_config.json")
	legacyOriginal, err := os.ReadFile(legacyFile)
	if err != nil {
		t.Fatal(err)
	}
	nativeFile := filepath.Join(plan.NativeRegistryRoot, "mcp_config.json")
	nativeOriginal, err := os.ReadFile(nativeFile)
	if err != nil {
		t.Fatal(err)
	}
	request := domain.ActivationRequest{
		Client: client, Plan: plan,
		Delivery: domain.StagedDelivery{ActivePath: plan.ActivePath, NativeObjects: objects},
	}
	env := clients.Env{NativeConfig: nativeconfig.New()}
	outcome, err := New().Activate(context.Background(), env, request)
	if err != nil || outcome.Verification != domain.VerificationInstalled || outcome.Activation != domain.ActivationPrepared {
		t.Fatalf("Devin Global install must report installed files, not real model invocation: %+v, %v", outcome, err)
	}
	layout := devinGlobalComponents(plan.NativeRegistryRoot, plan.ActivePath)
	if err := layout.Verify(objects, false, env.NativeConfig); err != nil {
		t.Fatal(err)
	}
	installed, err := os.ReadFile(nativeFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, literal := range []string{"global-stdio", "global-http", "foreign", "policy", "keep"} {
		if !strings.Contains(string(installed), literal) {
			t.Fatalf("installed config lost a native or foreign entry %q: %s", literal, installed)
		}
	}
	legacyAfter, err := os.ReadFile(legacyFile)
	if err != nil || string(legacyAfter) != string(legacyOriginal) {
		t.Fatalf("Legacy Cascade changed during selected Devin install: %v", err)
	}
	request.PreviousNativeObjects = objects
	if err := os.RemoveAll(filepath.Join(plan.NativeRegistryRoot, "skills", "global-marker")); err != nil {
		t.Fatal(err)
	}
	repaired, err := New().Activate(context.Background(), env, request)
	if err != nil || repaired.Verification != domain.VerificationInstalled {
		t.Fatalf("missing owned Global Skill was not repaired: %+v, %v", repaired, err)
	}
	remove := domain.DeactivationRequest{
		TargetContext: domain.TargetContext{Scope: domain.ScopeUser, UserConfigRoot: client.ConfigRoot},
		Client: client, NativeRegistryRoot: plan.NativeRegistryRoot,
		ManagedArtifactPath: plan.ActivePath, PhysicalArtifactID: plan.PhysicalArtifactID,
		NativeObjects: objects, Confirmed: true,
	}
	removed, err := New().Deactivate(context.Background(), env, remove)
	if err != nil || !removed.ExternalRemovalComplete {
		t.Fatalf("Global receipt-based removal failed: %+v, %v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(plan.NativeRegistryRoot, "skills", "global-marker")); !os.IsNotExist(err) {
		t.Fatalf("managed Global Skill survived removal: %v", err)
	}
	finalNative, err := os.ReadFile(nativeFile)
	if err != nil || !strings.Contains(string(finalNative), "foreign") ||
		!strings.Contains(string(finalNative), "keep") ||
		strings.Contains(string(finalNative), "global-stdio") ||
		strings.Contains(string(finalNative), "global-http") {
		t.Fatalf("Global removal touched foreign objects or retained owned: %v, %s", err, finalNative)
	}
	legacyAfter, err = os.ReadFile(legacyFile)
	if err != nil || string(legacyAfter) != string(legacyOriginal) {
		t.Fatal("Global removal mutated legacy Cascade configuration")
	}
	if !strings.Contains(string(nativeOriginal), "foreign") {
		t.Fatal("baseline native foreign object was not present")
	}
}

func TestDevinGlobalOfflineRejectsForeignSkillAndWrongReceipts(t *testing.T) {
	client, plan, envelope := globalOfflineFixture(t)
	objects, err := devinGlobalObjects(plan.ActivePath, envelope, plan)
	if err != nil {
		t.Fatal(err)
	}
	globalOfflineWrite(t, filepath.Join(plan.NativeRegistryRoot, "skills", "global-marker", "SKILL.md"), "foreign collision")
	env := clients.Env{NativeConfig: nativeconfig.New()}
	_, err = New().Activate(context.Background(), env, domain.ActivationRequest{
		Client: client, Plan: plan, Delivery: domain.StagedDelivery{ActivePath: plan.ActivePath, NativeObjects: objects},
	})
	if err == nil {
		t.Fatal("unowned Global Skill destination must fail without modification")
	}
	config, readErr := os.ReadFile(filepath.Join(plan.NativeRegistryRoot, "mcp_config.json"))
	if readErr != nil || strings.Contains(string(config), "global-http") {
		t.Fatal("foreign Skill collision mutated Global MCP settings")
	}
	wrong := append([]domain.NativeObjectOwnership(nil), objects...)
	wrong[0].Kind = WindsurfMCPObjectKind
	if _, err := New().Deactivate(context.Background(), env, domain.DeactivationRequest{
		TargetContext: domain.TargetContext{Scope: domain.ScopeUser},
		Client: client, NativeRegistryRoot: plan.NativeRegistryRoot,
		PhysicalArtifactID: plan.PhysicalArtifactID, ManagedArtifactPath: plan.ActivePath,
		NativeObjects: wrong, Confirmed: true,
	}); err == nil {
		t.Fatal("legacy Cascade ownership was accepted as Devin Global ownership")
	}
}

func TestDevinGlobalSelectedUserProfileIsNotLegacyOrProject(t *testing.T) {
	client, plan, _ := globalOfflineFixture(t)
	adapter := New()
	if got := adapter.UserNativeRegistry(client); got != client.NativeConfigRoot {
		t.Fatalf("selected global registry differs from original native root: %q", got)
	}
	if root, executable := adapter.NativeRegistry(clients.PlanInput{
		TargetContext: plan.TargetContext, Client: client,
	}); root != client.NativeConfigRoot || executable != "" {
		t.Fatalf("selected user registry fell back to Cascade: %q, %q", root, executable)
	}
	if err := adapter.CheckPlanPrecondition(clients.PlanInput{
		TargetContext: plan.TargetContext, Client: client,
	}, &plan); err != nil {
		t.Fatalf("exact genuine selected profile was refused: %v", err)
	}
	for _, mutate := range []func(*domain.DetectedClient, *domain.DeliveryPlan){
		func(c *domain.DetectedClient, _ *domain.DeliveryPlan) { c.Version = "" },
		func(c *domain.DetectedClient, _ *domain.DeliveryPlan) { c.Version = "3000.11.204" },
		func(c *domain.DetectedClient, _ *domain.DeliveryPlan) { c.NativeConfigRoot += "-other" },
		func(_ *domain.DetectedClient, p *domain.DeliveryPlan) { p.NativeRegistryRoot = client.ConfigRoot },
		func(_ *domain.DetectedClient, p *domain.DeliveryPlan) { p.NativeRegistryExecutable = client.ExecutablePath },
	} {
		changed, changedPlan := client, plan
		mutate(&changed, &changedPlan)
		if err := adapter.CheckPlanPrecondition(clients.PlanInput{TargetContext: changedPlan.TargetContext, Client: changed}, &changedPlan); err == nil {
			t.Fatalf("unqualified selected Devin Global identity was admitted: %+v", changed)
		}
	}
	legacy := client
	legacy.ExecutablePath = filepath.Join(filepath.Dir(client.ExecutablePath), "windsurf")
	if got := adapter.UserNativeRegistry(legacy); got != client.ConfigRoot {
		t.Fatalf("legacy Cascade root changed: %s", got)
	}
	if root, executable := adapter.NativeRegistry(clients.PlanInput{TargetContext: plan.TargetContext, Client: legacy}); root != legacy.ConfigRoot || executable != legacy.ExecutablePath {
		t.Fatal("legacy user native registry changed")
	}
	if got := adapter.UserNativeRegistry(domain.DetectedClient{ClientID: domain.ClientWindsurf}); got != "" {
		t.Fatal("missing selected host fabricated a global native root")
	}
}
