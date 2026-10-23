package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// This uses the production loader, planner, renderer, stager and activator.
// It proves an inert contract journey, not OpenCode session/model discovery.
func TestProjectOpenCodePlannerStagerRendersCommandsAndOwnsExactNativeComponents(t *testing.T) {
	request, projectB, user := projectOpenCodeRequest(t)
	beforeUser, beforeB := projectGeminiInventory(t, user), projectGeminiInventory(t, projectB)
	source := filepath.Join(t.TempDir(), "source")
	projectGeminiWrite(t, filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"1.0.0"}`)
	projectGeminiWrite(t, filepath.Join(source, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Project docs\n---\nInert docs.\n")
	projectGeminiWrite(t, filepath.Join(source, filepath.FromSlash(commandext.RelativeDirectory), "echo.md"), "---\ndescription: Echo the marker\ndisable-model-invocation: false\n---\nUAP-PROJECT-INERT\n")
	projectGeminiWrite(t, filepath.Join(source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://managed.test/mcp"}}}`)
	schemas, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := (loader.Loader{Registry: schemas}).Load(context.Background(), domain.LoadInput{SnapshotRoot: source})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	project := request.Plan.TargetContext.ProjectRoot
	planning := planner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(project, ".uap", "managed"), Paths: pathpolicy.Policy{}, Registry: registry}
	plan, err := planning.Plan(context.Background(), domain.PlanRequest{Envelope: envelope, Client: request.Client, Scope: domain.ScopeProject, ProjectRoot: project, PhysicalArtifactID: "demo-0123456789ab"})
	if err != nil || plan.Status != domain.PlanReady || plan.Activation != domain.ActivationPrepared || plan.NativeRegistryRoot != project {
		t.Fatalf("scoped OpenCode plan = %+v, %v", plan, err)
	}
	stager := providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}, PluginDataRoot: filepath.Join(project, ".uap", "data")}
	delivery, err := stager.Stage(context.Background(), envelope, plan, "project-opencode-command", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if len(opencode.OpenCodeObjects(delivery.NativeObjects)) != 3 {
		t.Fatalf("staged skill/command/MCP objects = %+v", delivery.NativeObjects)
	}
	if err := os.MkdirAll(filepath.Dir(plan.ActivePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	delivery.StagingPath = ""
	request.Plan, request.Delivery, request.DeclaredName = plan, delivery, "demo"
	activator := projectOpenCodeActivator(t, nativeconfig.New())
	outcome, err := activator.Activate(context.Background(), request)
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("staged project activation = %+v, %v", outcome, err)
	}
	body, err := os.ReadFile(filepath.Join(project, ".opencode", "skills", "demo-echo", "SKILL.md"))
	if err != nil || !strings.Contains(string(body), "UAP-PROJECT-INERT") {
		t.Fatalf("rendered command skill = %q, %v", body, err)
	}
	activeObjects, err := stager.ProjectActiveNative(context.Background(), envelope, plan, delivery.ArtifactDigest, "")
	if err != nil || !reflect.DeepEqual(activeObjects, opencode.OpenCodeObjects(delivery.NativeObjects)) {
		t.Fatalf("read-only native objects changed = %+v, %v", activeObjects, err)
	}
	remove := domain.DeactivationRequest{Client: request.Client, TargetContext: plan.TargetContext, NativeRegistryRoot: plan.NativeRegistryRoot, Confirmed: true, NativeObjects: outcome.NativeObjects}
	if out, err := activator.Deactivate(context.Background(), remove); err != nil || !out.ExternalRemovalComplete {
		t.Fatalf("project cleanup = %+v, %v", out, err)
	}
	if !reflect.DeepEqual(beforeUser, projectGeminiInventory(t, user)) || !reflect.DeepEqual(beforeB, projectGeminiInventory(t, projectB)) {
		t.Fatal("stager journey changed another scope")
	}
}

func projectOpenCodeActivator(t *testing.T, kernel nativeconfig.Kernel) providers.Activator {
	t.Helper()
	registry, err := clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	return providers.Activator{Registry: registry, NativeConfig: &kernel}
}
