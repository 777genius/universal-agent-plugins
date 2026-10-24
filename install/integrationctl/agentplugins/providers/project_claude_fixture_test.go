package providers_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type claudeProjectFixture struct {
	request             domain.ActivationRequest
	user, other, source string
	registry            *clients.Registry
	stager              providers.Stager
	runner              *claudeProjectRunner
}

func newClaudeProjectFixture(t *testing.T) *claudeProjectFixture {
	t.Helper()
	base := projectGeminiTemp(t)
	project, user, other := filepath.Join(base, "A"), filepath.Join(base, "user", ".claude"), filepath.Join(base, "B")
	for _, root := range []string{user, filepath.Join(project, ".claude"), filepath.Join(other, ".claude")} {
		projectGeminiWrite(t, filepath.Join(root, "settings.json"), `{"enabledPlugins":{"foreign@skills-dir":false}}`)
		projectGeminiWrite(t, filepath.Join(root, "skills", "foreign", "SKILL.md"), "foreign skill\n")
	}
	registry, err := clients.NewRegistry(claude.New())
	if err != nil {
		t.Fatal(err)
	}
	f := &claudeProjectFixture{user: user, other: other, source: filepath.Join(base, "source"), registry: registry}
	f.request.Client = domain.DetectedClient{ClientID: domain.ClientClaude, ConfigRoot: user, ExecutablePath: "/inert/claude-not-executed", Status: domain.DetectionDetected}
	f.request.Plan.TargetContext, err = domain.NewTargetContext(domain.ScopeProject, project, user)
	if err != nil {
		t.Fatal(err)
	}
	f.stager = providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}, PluginDataRoot: filepath.Join(project, ".uap", "data")}
	f.runner = &claudeProjectRunner{t: t, fixture: f, scope: "project", enabled: true}
	projectGeminiWrite(t, filepath.Join(f.source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"1.0.0"}`)
	projectGeminiWrite(t, filepath.Join(f.source, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Inert Claude fixture\n---\nVersion one.\n")
	projectGeminiWrite(t, filepath.Join(f.source, filepath.FromSlash(commandext.RelativeDirectory), "echo.md"), "---\ndescription: Echo inert marker\ndisable-model-invocation: false\n---\nUAP-CLAUDE-PROJECT-MARKER\n")
	projectGeminiWrite(t, filepath.Join(f.source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://managed.test/v1"}}}`)
	f.stage(t, "initial")
	return f
}

func (f *claudeProjectFixture) stage(t *testing.T, operation string) {
	t.Helper()
	schemas, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := (loader.Loader{Registry: schemas}).Load(t.Context(), domain.LoadInput{SnapshotRoot: f.source})
	if err != nil {
		t.Fatal(err)
	}
	project := f.request.Plan.TargetContext.ProjectRoot
	planning := planner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(project, ".uap", "managed"), Paths: pathpolicy.Policy{}, Registry: f.registry}
	plan, err := planning.Plan(t.Context(), domain.PlanRequest{Envelope: envelope, Client: f.request.Client, Scope: domain.ScopeProject, ProjectRoot: project, PhysicalArtifactID: "demo-0123456789ab"})
	if err != nil || plan.Status != domain.PlanReady || plan.NativeRegistryRoot != filepath.Join(project, ".claude") {
		t.Fatalf("Claude project plan=%+v, %v", plan, err)
	}
	delivery, err := f.stager.Stage(t.Context(), envelope, plan, operation, domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(delivery.StagingPath) == plan.TargetRoot {
		t.Fatal("staging entered watched skill root")
	}
	if err := os.MkdirAll(plan.TargetRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	delivery.StagingPath = ""
	f.request.Plan, f.request.Delivery, f.request.DeclaredName, f.request.BackendExecutable = plan, delivery, "demo", f.request.Client.ExecutablePath
}

func (f *claudeProjectFixture) activator() providers.Activator {
	return providers.Activator{Registry: f.registry, Runner: f.runner}
}

func (f *claudeProjectFixture) removal() domain.DeactivationRequest {
	return domain.DeactivationRequest{Client: f.request.Client, TargetContext: f.request.Plan.TargetContext, NativeRegistryRoot: f.request.Plan.NativeRegistryRoot,
		Confirmed: true, BackendExecutable: f.request.BackendExecutable, DeclaredName: "demo", ManagedArtifactPath: f.request.Plan.ActivePath, CurrentActivation: domain.ActivationActive}
}

// A fixture runner validates production command contracts without executing a
// native client or package. Its listing is fixture evidence only.
type claudeProjectRunner struct {
	t                       *testing.T
	fixture                 *claudeProjectFixture
	scope                   string
	enabled                 bool
	wrongPath, malformed    bool
	suppressed, otherSource bool
	calls                   int
}

func (r *claudeProjectRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.t.Helper()
	f := r.fixture
	if !reflect.DeepEqual(command.Argv, []string{f.request.Client.ExecutablePath, "plugin", "list", "--json"}) || command.Dir != f.request.Plan.TargetContext.ProjectRoot {
		r.t.Fatalf("wrong scoped native command: %+v", command)
	}
	want := "CLAUDE_CONFIG_DIR=" + f.user
	found := false
	for _, value := range command.Env {
		if value == want {
			found = true
		}
	}
	if !found {
		r.t.Fatalf("original user authority lost: %q", command.Env)
	}
	r.calls++
	if r.malformed {
		return legacyports.CommandResult{Stdout: []byte(`{"unexpected":true}`)}, nil
	}
	path := f.request.Plan.ActivePath
	if r.wrongPath {
		path = filepath.Join(f.other, ".claude", "skills", filepath.Base(path))
	}
	rows := []map[string]any{{"id": "demo@skills-dir", "scope": r.scope, "enabled": r.enabled, "installPath": path}}
	if r.suppressed {
		rows = []map[string]any{{"id": "(suppressed)@skills-dir", "version": "unknown", "scope": "project", "enabled": false, "installPath": "", "notes": []string{"1 project-scope directory under ./.claude/skills/ that may load as a plugin was skipped because this workspace was not trusted when plugins were scanned. After accepting the trust dialog, run /reload-plugins (or relaunch) to load what qualifies."}}}
	}
	if r.otherSource {
		rows = append(rows, map[string]any{"id": "demo@skills-dir", "scope": "user", "enabled": true, "installPath": filepath.Join(f.user, "skills", "demo-user")})
	}
	body, err := json.Marshal(rows)
	return legacyports.CommandResult{Stdout: body}, err
}
