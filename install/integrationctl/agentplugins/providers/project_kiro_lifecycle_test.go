package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func kiroAssertPrepared(t *testing.T, outcome domain.ActivationOutcome, err error) {
	t.Helper()
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || outcome.ActivationAttested {
		t.Fatalf("Kiro configured state = %+v, %v", outcome, err)
	}
}

// Inert production loader/planner/renderer/stager/provider evidence; this does
// not execute Kiro, the injected launcher, a model or an MCP server.
func TestProjectKiroComposedLifecycleAndScopeIsolation(t *testing.T) {
	fixture := newKiroProjectFixture(t, false)
	userBefore, bBefore := projectGeminiInventory(t, fixture.user), projectGeminiInventory(t, fixture.other)
	activator := fixture.activator(nativeconfig.New())
	outcome, err := activator.Activate(context.Background(), fixture.request)
	kiroAssertPrepared(t, outcome, err)
	root := fixture.request.Plan.NativeRegistryRoot
	activeBefore := projectGeminiInventory(t, fixture.request.Plan.ActivePath)
	objects, err := fixture.stager.ProjectActiveNative(context.Background(), fixture.envelope, fixture.request.Plan, fixture.request.Delivery.ArtifactDigest, "")
	if err != nil || len(objects) != 2 || !reflect.DeepEqual(activeBefore, projectGeminiInventory(t, fixture.request.Plan.ActivePath)) {
		t.Fatalf("read-only projection = %+v, %v", objects, err)
	}
	binding := domain.ClientBinding{Scope: "project", PhysicalArtifact: fixture.request.Plan.PhysicalArtifactID,
		TargetLocator: fixture.request.Plan.ActivePath, NativeProfileRoot: root, NativeObjects: outcome.NativeObjects}
	if err := kiro.New().ValidateBindingProfile(root, binding); err != nil {
		t.Fatal(err)
	}
	fixture.request.PreviousNativeObjects = outcome.NativeObjects
	for _, verify := range []bool{false, true} {
		fixture.request.VerifyOnly = verify
		outcome, err = activator.Activate(context.Background(), fixture.request)
		kiroAssertPrepared(t, outcome, err)
	}
	data := filepath.Join(fixture.request.Plan.TargetContext.ProjectRoot, ".uap", "data", "keep.txt")
	projectGeminiWrite(t, data, "persistent data\n")
	for range 2 {
		removed, err := activator.Deactivate(context.Background(), fixture.removal(outcome.NativeObjects))
		if err != nil || !removed.ExternalRemovalComplete {
			t.Fatalf("repeat exact remove = %+v, %v", removed, err)
		}
	}
	projectGeminiAssert(t, data, "persistent data\n")
	if _, err := os.Stat(filepath.Join(root, "skills", "docs")); !os.IsNotExist(err) {
		t.Fatalf("managed skill survived remove: %v", err)
	}
	if !reflect.DeepEqual(userBefore, projectGeminiInventory(t, fixture.user)) || !reflect.DeepEqual(bBefore, projectGeminiInventory(t, fixture.other)) {
		t.Fatal("Kiro project lifecycle changed user or project B")
	}
	projectGeminiAssert(t, filepath.Join(root, "hooks.json"), "foreign hooks\n")
}

func TestProjectKiroUpdateRepairAndRemovedComponent(t *testing.T) {
	fixture := newKiroProjectFixture(t, false)
	activator := fixture.activator(nativeconfig.New())
	outcome, err := activator.Activate(context.Background(), fixture.request)
	kiroAssertPrepared(t, outcome, err)
	fixture.request.PreviousNativeObjects = outcome.NativeObjects
	projectGeminiWrite(t, filepath.Join(fixture.source, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Updated inert fixture\n---\nVersion two.\n")
	projectGeminiWrite(t, filepath.Join(fixture.source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://managed.test/v2"}}}`)
	fixture.stage(t, "update")
	outcome, err = activator.Activate(context.Background(), fixture.request)
	kiroAssertPrepared(t, outcome, err)
	skill := filepath.Join(fixture.request.Plan.NativeRegistryRoot, "skills", "docs")
	if err := os.RemoveAll(skill); err != nil {
		t.Fatal(err)
	}
	fixture.request.PreviousNativeObjects = outcome.NativeObjects
	outcome, err = activator.Activate(context.Background(), fixture.request)
	kiroAssertPrepared(t, outcome, err)
	projectGeminiAssert(t, filepath.Join(skill, "SKILL.md"), "---\nname: docs\ndescription: Updated inert fixture\n---\nVersion two.\n")
	projectGeminiWrite(t, filepath.Join(fixture.source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{}}`)
	fixture.request.PreviousNativeObjects = outcome.NativeObjects
	fixture.stage(t, "drop-mcp")
	outcome, err = activator.Activate(context.Background(), fixture.request)
	kiroAssertPrepared(t, outcome, err)
	config, err := os.ReadFile(filepath.Join(fixture.request.Plan.NativeRegistryRoot, "settings", "mcp.json"))
	if err != nil || strings.Contains(string(config), "managed.test") || !strings.Contains(string(config), `"disabled":true`) {
		t.Fatalf("removed component/foreign data = %s, %v", config, err)
	}
}
