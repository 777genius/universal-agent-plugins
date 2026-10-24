package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/codextoml"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

type projectCodexFixture struct {
	user, a, b, source string
	registry           *clients.Registry
	kernel             nativeconfig.Kernel
	activator          providers.Activator
}

func projectCodexWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func newProjectCodexFixture(t *testing.T) projectCodexFixture {
	t.Helper()
	base, err := os.MkdirTemp("", "uap-project-codex-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	f := projectCodexFixture{user: filepath.Join(base, "user", ".codex"), a: filepath.Join(base, "A"), b: filepath.Join(base, "B"), source: filepath.Join(base, "source")}
	for _, config := range []string{filepath.Join(f.user, "config.toml"), filepath.Join(f.a, ".codex", "config.toml"), filepath.Join(f.b, ".codex", "config.toml")} {
		projectCodexWrite(t, config, "# foreign native policy\nmodel = 'foreign-model'\n[mcp_servers.foreign]\nurl='https://foreign.test/mcp' # keep server\n")
	}
	projectCodexWrite(t, filepath.Join(f.user, "plugins", "cache", "foreign", "keep.txt"), "user plugin cache\n")
	for _, root := range []string{f.a, f.b} {
		projectCodexWrite(t, filepath.Join(root, ".agents", "skills", "foreign", "SKILL.md"), "foreign repository skill\n")
	}
	projectCodexWrite(t, filepath.Join(f.source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"1.0.0"}`)
	projectCodexWrite(t, filepath.Join(f.source, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Inert project fixture\n---\nUAP-CODEX-PROJECT\n")
	projectCodexWrite(t, filepath.Join(f.source, filepath.FromSlash(commandext.RelativeDirectory), "echo.md"), "---\ndescription: Inert command\ndisable-model-invocation: false\n---\nUAP-CODEX-COMMAND\n")
	projectCodexWrite(t, filepath.Join(f.source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://managed.test/mcp","headers":{"X-Fixture":"inert"}},"local":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
	f.registry, err = clients.NewRegistry(codex.New())
	if err != nil {
		t.Fatal(err)
	}
	f.kernel = nativeconfig.New().WithCodexDocumentParser(codextoml.Parse)
	f.activator = providers.Activator{Registry: f.registry, NativeConfig: &f.kernel}
	return f
}

func (f projectCodexFixture) stage(t *testing.T, hints domain.CompatibilityHints) domain.ActivationRequest {
	t.Helper()
	envelope := f.load(t)
	client := domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: f.user, Version: "0.153.4", Status: domain.DetectionDetected}
	plan, err := (planner.Planner{ProjectRoot: f.a, ManagedRoot: filepath.Join(f.a, ".uap", "managed"), Paths: pathpolicy.Policy{}, Registry: f.registry}).Plan(context.Background(), domain.PlanRequest{
		Envelope: envelope, Client: client, Scope: domain.ScopeProject, ProjectRoot: f.a, PhysicalArtifactID: "demo-0123456789ab"})
	if err != nil || plan.Status != domain.PlanReady || plan.Activation != domain.ActivationPrepared || plan.NativeRegistryRoot != f.a {
		t.Fatalf("project Codex plan = %+v, %v", plan, err)
	}
	delivery, err := (providers.Stager{Registry: f.registry, Paths: pathpolicy.Policy{}, PluginDataRoot: filepath.Join(f.a, ".uap", "data")}).StageWithPluginData(t.Context(), envelope, plan, "project-codex-fixture", hints, filepath.Join(f.a, ".uap", "data", "owned-slot"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(plan.ActivePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	delivery.StagingPath = ""
	return domain.ActivationRequest{Client: client, Plan: plan, Delivery: delivery, DeclaredName: "demo"}
}

func (f projectCodexFixture) load(t *testing.T) domain.PackageEnvelope {
	t.Helper()
	schemas, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := (loader.Loader{Registry: schemas}).Load(t.Context(), domain.LoadInput{SnapshotRoot: f.source})
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func projectCodexRead(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
