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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/grok"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type grokProjectProviderRunner struct {
	t       *testing.T
	request domain.ActivationRequest
	calls   int
}

func (r *grokProjectProviderRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	plan := r.request.Plan
	if !reflect.DeepEqual(command.Argv, []string{r.request.Client.ExecutablePath, "inspect", "--json"}) || command.Dir != plan.TargetContext.ProjectRoot {
		r.t.Fatal("non-readonly or wrong project command", command)
	}
	body, _ := json.Marshal(map[string]any{"grokVersion": "1.0.67", "cwd": command.Dir, "projectRoot": command.Dir + string(filepath.Separator), "projectTrusted": false, "skills": []any{}, "plugins": []any{map[string]any{"name": plan.DeclaredName, "scope": "project", "path": plan.ActivePath, "enabled": true}}})
	return legacyports.CommandResult{Stdout: body}, nil
}
func TestProjectGrokComposedPlannerStagerActivationAttestation(t *testing.T) {
	for _, mode := range []struct {
		name       string
		withSkills bool
	}{{"skills-and-mcp", true}, {"mcp-only", false}} {
		t.Run(mode.name, func(t *testing.T) { testProjectGrokPreparedMCP(t, mode.withSkills) })
	}
}

func testProjectGrokPreparedMCP(t *testing.T, withSkills bool) {
	base := projectGeminiTemp(t)
	project, user, source := filepath.Join(base, "A"), filepath.Join(base, "home", ".grok"), filepath.Join(base, "source")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	projectGeminiWrite(t, filepath.Join(user, "keep"), "original")
	projectGeminiWrite(t, filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"1.0.0"}`)
	if withSkills {
		projectGeminiWrite(t, filepath.Join(source, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Inert Grok project fixture\n---\nText only.\n")
	}
	projectGeminiWrite(t, filepath.Join(source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://inert.test/mcp"}}}`)
	schemas, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := (loader.Loader{Registry: schemas}).Load(t.Context(), domain.LoadInput{SnapshotRoot: source})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(grok.New())
	if err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientGrok, ConfigRoot: user, ExecutablePath: "/inert/grok-not-executed", Status: domain.DetectionDetected}
	planning := planner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(project, ".uap", "managed"), Paths: pathpolicy.Policy{}, Registry: registry}
	plan, err := planning.Plan(t.Context(), domain.PlanRequest{Envelope: envelope, Client: client, Scope: domain.ScopeProject, ProjectRoot: project, PhysicalArtifactID: "demo-0123456789ab"})
	if err != nil || plan.Status != domain.PlanReady || plan.TargetRoot != filepath.Join(project, ".grok", "plugins") {
		t.Fatalf("Grok plan %+v %v", plan, err)
	}
	mcpFound := false
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentMCPServer {
			if component.Support != domain.SupportNative || component.Name != "docs" {
				t.Fatalf("project MCP lost native support: %+v", component)
			}
			mcpFound = true
		}
	}
	if !mcpFound {
		t.Fatal("project MCP selection missing")
	}
	stager := providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}, PluginDataRoot: filepath.Join(project, ".uap", "data")}
	delivery, err := stager.Stage(t.Context(), envelope, plan, "inert", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(delivery.StagingPath) != plan.TargetAnchor {
		t.Fatal("staging entered watched plugin root")
	}
	if _, err := os.Stat(filepath.Join(delivery.StagingPath, "mcp.json")); err != nil {
		t.Fatal("Grok Project lost its portable MCP source", err)
	}
	body, err := os.ReadFile(filepath.Join(delivery.StagingPath, ".mcp.json"))
	if err != nil {
		t.Fatal("project MCP sidecar missing", err)
	}
	var document struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &document); err != nil || len(document.Servers) != 1 ||
		document.Servers["docs"]["type"] != "http" || document.Servers["docs"]["url"] != "https://inert.test/mcp" {
		t.Fatalf("wrong project Grok MCP sidecar: %s %v", body, err)
	}
	manifest, err := os.ReadFile(filepath.Join(delivery.StagingPath, ".grok-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var attributes map[string]any
	if err := json.Unmarshal(manifest, &attributes); err != nil || attributes["mcpServers"] != "./.mcp.json" {
		t.Fatalf("wrong Grok project manifest MCP path: %s %v", manifest, err)
	}
	if _, ok := attributes["skills"]; ok != withSkills {
		t.Fatalf("Grok project skill projection disagrees with package: %s", manifest)
	}
	if err := os.MkdirAll(plan.TargetRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	delivery.StagingPath = ""
	request := domain.ActivationRequest{Client: client, Plan: plan, Delivery: delivery, DeclaredName: "demo", BackendExecutable: client.ExecutablePath}
	runner := &grokProjectProviderRunner{t: t, request: request}
	actor := providers.Activator{Registry: registry, Runner: runner}
	outcome, err := actor.Activate(t.Context(), request)
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationPackageValid || outcome.AuthoritativeObservation {
		t.Fatalf("composed source promoted: %+v %v", outcome, err)
	}
	request.ActivationComplete = true
	before := runner.calls
	projectBefore, userBefore := projectGeminiInventory(t, project), projectGeminiInventory(t, user)
	if err := actor.PreflightActivation(request); err == nil {
		t.Fatal("provider preflight accepted attestation")
	}
	if _, err := actor.Activate(t.Context(), request); err == nil {
		t.Fatal("provider accepted attestation")
	}
	if before != runner.calls || !reflect.DeepEqual(projectBefore, projectGeminiInventory(t, project)) || !reflect.DeepEqual(userBefore, projectGeminiInventory(t, user)) {
		t.Fatal("refused attestation made effects")
	}
}
