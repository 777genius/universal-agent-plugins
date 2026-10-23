package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

func TestProjectKimiRequiresCurrentObservedHarnessAndNearestGit(t *testing.T) {
	f := newKimiProjectFixture(t, false, false)
	p := planner.Planner{Registry: f.registry, Paths: pathpolicy.Policy{}, ProjectRoot: f.project, ManagedRoot: filepath.Join(f.project, ".uap", "managed")}
	qualified, err := p.Plan(context.Background(), domain.PlanRequest{Envelope: f.envelope, Client: f.request.Client, Scope: domain.ScopeProject, ProjectRoot: f.project, PhysicalArtifactID: f.request.Plan.PhysicalArtifactID})
	if err != nil || qualified.Status != domain.PlanReady {
		t.Fatalf("current observed Kimi harness did not qualify: %+v, %v", qualified, err)
	}
	for _, change := range []func(*domain.DetectedClient){
		func(c *domain.DetectedClient) { c.Version = "" }, func(c *domain.DetectedClient) { c.Version = "2.3.0" }, func(c *domain.DetectedClient) { c.Version = "2.3.1-alpha" }, func(c *domain.DetectedClient) { c.ExecutablePath = filepath.Join(f.base, "kimi-cli") },
	} {
		client := f.request.Client
		change(&client)
		before := projectGeminiInventory(t, f.base)
		if _, err := p.Plan(context.Background(), domain.PlanRequest{Envelope: f.envelope, Client: client, Scope: domain.ScopeProject, ProjectRoot: f.project, PhysicalArtifactID: f.request.Plan.PhysicalArtifactID}); err == nil || !strings.Contains(err.Error(), "require the explicitly selected current Kimi Code 2.3.1 executable/version") {
			t.Fatalf("unobserved current Kimi harness refusal = %v", err)
		}
		if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
			t.Fatal("admission refusal created effects")
		}
	}
	if err := os.Remove(filepath.Join(f.project, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(f.base, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	before := projectGeminiInventory(t, f.base)
	if _, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request); err == nil || !strings.Contains(err.Error(), "differs from selected project") {
		t.Fatalf("nearest Git ancestor mismatch = %v", err)
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
		t.Fatal("nearest Git refusal changed files")
	}
}

func TestProjectKimiExactRemovalDoesNotNeedCurrentVersionOrGit(t *testing.T) {
	f := newKimiProjectFixture(t, false, false)
	a := f.activator(nativeconfig.New())
	out, err := a.Activate(context.Background(), f.request)
	kimiPrepared(t, out, err)
	r := f.remove(out.NativeObjects)
	r.Client.Version, r.Client.ExecutablePath = "", ""
	if err := os.Remove(filepath.Join(f.project, ".git")); err != nil {
		t.Fatal(err)
	}
	removed, err := a.Deactivate(context.Background(), r)
	if err != nil || !removed.ExternalRemovalComplete {
		t.Fatalf("exact receipt removal depended on native harness/discovery: %+v %v", removed, err)
	}
}

func TestProjectKimiSSEIsScopedAndLowerPrioritySourcesStayForeign(t *testing.T) {
	f := newKimiProjectFixture(t, false, true)
	userConfig := filepath.Join(f.user, "mcp.json")
	const lower = `{"mcpServers":{"docs":{"transport":"http","url":"https://lower.test/mcp","enabled":false}}}`
	projectGeminiWrite(t, userConfig, lower)
	projectGeminiWrite(t, filepath.Join(f.project, ".mcp.json"), lower)
	if err := os.RemoveAll(filepath.Join(f.source, "skills")); err != nil {
		t.Fatal(err)
	}
	projectGeminiWrite(t, filepath.Join(f.source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"sse","url":"https://managed.test/events"}}}`)
	f.stage(t, "sse-only")
	if !strings.Contains(strings.Join(f.request.Plan.Warnings, " "), "overrides a same-name lower-priority") {
		t.Fatal("project-over-user/compatibility policy was not exposed")
	}
	out, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request)
	kimiPrepared(t, out, err)
	body, err := os.ReadFile(f.config)
	if err != nil || !strings.Contains(string(body), `"transport":"sse"`) {
		t.Fatalf("project-only SSE transport = %s,%v", body, err)
	}
	projectGeminiAssert(t, userConfig, lower)
	projectGeminiAssert(t, filepath.Join(f.project, ".mcp.json"), lower)
	userPlanner := planner.Planner{Registry: f.registry, Paths: pathpolicy.Policy{}, ManagedRoot: filepath.Join(f.base, "user-managed")}
	plan, err := userPlanner.Plan(context.Background(), domain.PlanRequest{Envelope: f.envelope, Client: f.request.Client, Scope: domain.ScopeUser, PhysicalArtifactID: "sse-user-0123456789ab"})
	if err != nil || len(plan.Components) != 1 || plan.Components[0].Support != domain.SupportUnsupported {
		t.Fatalf("project SSE widened preserved user policy: %+v,%v", plan, err)
	}
}
