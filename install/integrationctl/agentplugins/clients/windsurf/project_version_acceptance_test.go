package windsurf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestDevinProjectExactObservedVersionsKeepOriginalProfiles(t *testing.T) {
	selected, _, _ := globalOfflineFixture(t)
	projectRoot := filepath.Join(filepath.Dir(filepath.Dir(selected.NativeConfigRoot)), "workspace")
	nativeRoot := filepath.Join(projectRoot, ".devin")
	globalOfflineWrite(t, filepath.Join(nativeRoot, "mcp_config.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.example/mcp"}},"foreign":true}`)
	target := domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: projectRoot, UserConfigRoot: selected.ConfigRoot}
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientWindsurf, Scope: domain.ScopeProject,
		TargetContext: target, NativeRegistryRoot: nativeRoot,
		PhysicalArtifactID: "project-version-proof",
		ActivePath:         filepath.Join(projectRoot, ".uap", "managed", "clients", "windsurf", "project-version-proof"),
		Components: []domain.ComponentDecision{
			{Kind: domain.ComponentSkill, Name: "owned-skill", Support: domain.SupportPrepared},
			{Kind: domain.ComponentMCPServer, Name: "owned-mcp", Support: domain.SupportPrepared},
		},
	}
	watched := []string{
		filepath.Join(nativeRoot, "mcp_config.json"),
		filepath.Join(selected.NativeConfigRoot, "mcp_config.json"),
		filepath.Join(selected.ConfigRoot, "mcp_config.json"),
	}
	before := make([]string, len(watched))
	for i, name := range watched {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		before[i] = string(body)
	}
	for _, version := range []string{"3000.11.205", "3000.12.57"} {
		t.Run("supported-"+version, func(t *testing.T) {
			candidate := selected
			candidate.Version = version
			if err := New().CheckPlanPrecondition(clients.PlanInput{
				Client: candidate, TargetContext: target,
			}, &plan); err != nil {
				t.Fatalf("observed Devin Project version refused: %v", err)
			}
		})
	}
	for _, version := range []string{"", "3000.11.204", "3000.12.56", "3000.12.58", "3000.12.57-beta", "3000.12.57\n"} {
		t.Run("unsupported-"+version, func(t *testing.T) {
			candidate := selected
			candidate.Version = version
			if err := New().CheckPlanPrecondition(clients.PlanInput{
				Client: candidate, TargetContext: target,
			}, &plan); err == nil {
				t.Fatalf("unsupported project version %q admitted", version)
			}
		})
	}
	sibling := selected
	sibling.Version = "3000.12.57"
	sibling.ExecutablePath = filepath.Join(filepath.Dir(selected.ExecutablePath), "windsurf")
	sibling.Surfaces = []domain.ClientSurface{{ID: "devin_cli", Detected: true}}
	if err := New().CheckPlanPrecondition(clients.PlanInput{
		Client: sibling, TargetContext: target,
	}, &plan); err == nil {
		t.Fatal("secondary Devin executable authorized legacy Windsurf primary")
	}
	wrongTarget := target
	wrongTarget.UserConfigRoot = nativeRoot
	if err := New().CheckPlanPrecondition(clients.PlanInput{
		Client: selected, TargetContext: wrongTarget,
	}, &plan); err == nil {
		t.Fatal("different original user profile authorized Project mutation")
	}
	for i, name := range watched {
		after, err := os.ReadFile(name)
		if err != nil || string(after) != before[i] {
			t.Fatalf("version precondition mutated original authority %q: %v", name, err)
		}
	}
}
