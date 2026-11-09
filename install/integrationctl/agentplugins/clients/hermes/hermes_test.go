package hermes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestHermesClientIdentityAndDetection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	profile := filepath.Join(root, ".hermes")
	probes := clients.HostProbes{
		HomeDir: root, WorkingDir: root,
		Environment: map[string]string{"HERMES_HOME": profile},
		Lstat: os.Lstat, EvalSymlinks: filepath.EvalSymlinks,
		LookPath: func(binary string) (string, error) {
			if binary != "hermes" {
				t.Fatalf("unexpected binary %q", binary)
			}
			return filepath.Join(root, "hermes"), nil
		},
	}
	adapter := New()
	if adapter.ID() != domain.ClientHermes {
		t.Fatal("Hermes identity mismatch")
	}
	result := adapter.DetectSurfaces(clients.NewHost(probes))
	if result.Err != nil || result.ConfigRoot != profile || result.ExecutablePath == "" {
		t.Fatalf("Hermes detection: %+v", result)
	}
	if len(result.SelectionSurfaceIDs) != 1 || result.SelectionSurfaceIDs[0] != "hermes_cli" {
		t.Fatalf("directory presence must not imply executable detection: %+v", result)
	}
	probes.Environment["HERMES_HOME"] = "../other"
	if r := adapter.DetectSurfaces(clients.NewHost(probes)); r.Err == nil {
		t.Fatal("relative profile must be rejected before filesystem mutation")
	}
}

func TestHermesProjectScopeCannotInstallUserMCP(t *testing.T) {
	t.Parallel()
	adapter := New()
	root := t.TempDir()
	client := domain.DetectedClient{ClientID: domain.ClientHermes}
	target := domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: root}
	anchor, destination, err := adapter.ProjectTargetRoot(client, domain.PackagePrepared,
		filepath.Join(root, ".uap"), target)
	if err != nil || !strings.HasPrefix(destination, anchor) {
		t.Fatalf("project managed root refused: %v %q", err, destination)
	}
	plan := domain.DeliveryPlan{
		Scope: domain.ScopeProject, ClientID: domain.ClientHermes,
		TargetContext: target,
		NativeRegistryRoot: filepath.Join(root, ".hermes"),
		Status: domain.PlanReady,
		Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "foreign", Support: domain.SupportUnsupported}},
	}
	if err := adapter.QualifyPlan(clients.PlanInput{}, &plan); err != nil || plan.Status != domain.PlanUnsupported {
		t.Fatalf("project MCP must fail closed, not fall back to user: %+v, %v", plan, err)
	}
	plan.Status = domain.PlanReady
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "managed", Support: domain.SupportNative}}
	if err := adapter.RefinePlan(context.Background(), clients.PlanInput{}, &plan); err != nil || plan.Activation != domain.ActivationPrepared {
		t.Fatalf("skills must remain Prepared until native trust: %+v, %v", plan, err)
	}
}
