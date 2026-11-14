package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

type kiroProjectFixture struct {
	request             domain.ActivationRequest
	user, other, source string
	envelope            domain.PackageEnvelope
	registry            *clients.Registry
	stager              providers.Stager
}

func newKiroProjectFixture(t *testing.T, stdio bool) *kiroProjectFixture {
	t.Helper()
	base := projectGeminiTemp(t)
	project, user, other := filepath.Join(base, "A"), filepath.Join(base, "user", ".kiro"), filepath.Join(base, "B")
	for _, root := range []string{user, filepath.Join(project, ".kiro"), filepath.Join(other, ".kiro")} {
		projectGeminiWrite(t, filepath.Join(root, "settings", "mcp.json"), `{ // preserve comment
"mcpServers":{"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"policy":"preserve"}`)
		projectGeminiWrite(t, filepath.Join(root, "hooks.json"), "foreign hooks\n")
		projectGeminiWrite(t, filepath.Join(root, "skills", "foreign", "SKILL.md"), "foreign skill\n")
	}
	registry, err := clients.NewRegistry(kiro.New())
	if err != nil {
		t.Fatal(err)
	}
	fixture := &kiroProjectFixture{source: filepath.Join(base, "source"), user: user, other: other, registry: registry}
	fixture.stager = providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}, PluginDataRoot: filepath.Join(project, ".uap", "data")}
	fixture.request.Client = domain.DetectedClient{ClientID: domain.ClientKiro, ConfigRoot: user, ExecutablePath: "fixture-editor-never-executed", Status: domain.DetectionDetected}
	fixture.request.Plan.TargetContext, err = domain.NewTargetContext(domain.ScopeProject, project, user)
	if err != nil {
		t.Fatal(err)
	}
	projectGeminiWrite(t, filepath.Join(fixture.source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"1.0.0"}`)
	projectGeminiWrite(t, filepath.Join(fixture.source, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Inert project fixture\n---\nVersion one.\n")
	mcp := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://managed.test/v1"}}}`
	if stdio {
		mcp = `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/server.js","${PLUGIN_DATA}"],"cwd":"${PLUGIN_ROOT}"}}}`
		projectGeminiWrite(t, filepath.Join(fixture.source, "server.js"), "// Inert fixture; never executed.\n")
		fixture.stager.LauncherSource = kiroFixtureLauncher(t, base)
	}
	projectGeminiWrite(t, filepath.Join(fixture.source, "mcp.json"), mcp)
	fixture.stage(t, "initial")
	return fixture
}

func kiroFixtureLauncher(t *testing.T, base string) *managedstdio.Source {
	t.Helper()
	if !managedstdio.Supported() {
		t.Skip("managed stdio platform unsupported")
	}
	path := filepath.Join(base, "inert-launcher")
	if err := os.WriteFile(path, []byte("trusted fixture bytes; never executed"), 0700); err != nil {
		t.Fatal(err)
	}
	source, err := managedstdio.NewSource(path, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func (fixture *kiroProjectFixture) stage(t *testing.T, operation string) {
	t.Helper()
	schemas, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	fixture.envelope, err = (loader.Loader{Registry: schemas}).Load(context.Background(), domain.LoadInput{SnapshotRoot: fixture.source})
	if err != nil {
		t.Fatal(err)
	}
	project := fixture.request.Plan.TargetContext.ProjectRoot
	planning := planner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(project, ".uap", "managed"), Paths: pathpolicy.Policy{}, Registry: fixture.registry}
	plan, err := planning.Plan(context.Background(), domain.PlanRequest{Envelope: fixture.envelope, Client: fixture.request.Client, Scope: domain.ScopeProject, ProjectRoot: project, PhysicalArtifactID: "demo-0123456789ab"})
	if err != nil || plan.Status != domain.PlanReady || plan.Activation != domain.ActivationPrepared {
		t.Fatalf("Kiro project plan = %+v, %v", plan, err)
	}
	delivery, err := fixture.stager.Stage(context.Background(), fixture.envelope, plan, operation, domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(plan.ActivePath); err == nil {
		if err := os.Rename(plan.ActivePath, plan.ActivePath+"-"+operation+"-backup"); err != nil {
			t.Fatal(err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	delivery.StagingPath = ""
	fixture.request.Plan, fixture.request.Delivery, fixture.request.DeclaredName = plan, delivery, "demo"
}

func (fixture *kiroProjectFixture) activator(kernel nativeconfig.Kernel) providers.Activator {
	return providers.Activator{Registry: fixture.registry, NativeConfig: &kernel}
}

func (fixture *kiroProjectFixture) removal(objects []domain.NativeObjectOwnership) domain.DeactivationRequest {
	return domain.DeactivationRequest{Client: fixture.request.Client, TargetContext: fixture.request.Plan.TargetContext,
		NativeRegistryRoot: fixture.request.Plan.NativeRegistryRoot, Confirmed: true, NativeObjects: objects}
}
