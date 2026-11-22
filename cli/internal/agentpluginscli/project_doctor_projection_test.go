package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/plannertest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type refusingClientRunner struct{ calls int }

func (runner *refusingClientRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	return legacyports.CommandResult{}, errors.New("inert fixture never runs a client")
}

// Doctor once reported every healthy Gemini project installation as a changed
// projection and sent users to repair, which could never clear the finding.
// The real adapters verify the exact owned objects here; native discovery
// stays unobserved and is reported separately, also by clients that answer
// the read-only check with a prepared activation.
func TestProjectDoctorRecognizesIntactOwnedProjection(t *testing.T) {
	for _, tc := range []struct {
		client domain.ClientID
		skills string
	}{
		{domain.ClientGemini, ".gemini/skills"},
		{domain.ClientOpenCode, ".opencode/skills"},
		{domain.ClientCursor, ".cursor/skills"},
		{domain.ClientCodex, ".agents/skills"},
	} {
		t.Run(string(tc.client), func(t *testing.T) { testProjectDoctorRecognizesIntactOwnedProjection(t, tc.client, tc.skills) })
	}
}

func testProjectDoctorRecognizesIntactOwnedProjection(t *testing.T, clientID domain.ClientID, skills string) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client := fixtureClient(t, clientID)
	f := newCLIFixtureAt(t, []domain.DetectedClient{client}, root)
	project := filepath.Join(root, "project")
	for _, path := range []string{project, client.ConfigRoot} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	stateRoot := filepath.Join(project, ".uap")
	f.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json"), ProjectRoot: project}
	f.app.Scope, f.app.ProjectRoot, f.app.ManagedRoot, f.app.StateStore = domain.ScopeProject, project, filepath.Join(stateRoot, "managed"), f.store
	f.app.EnvironmentLockPath = filepath.Join(stateRoot, "environment.lock")
	planner := plannertest.NewPlanner(clientplanner.Planner{ProjectRoot: project, ManagedRoot: f.app.ManagedRoot})
	f.app.Planner, f.app.Targets = planner, planner
	runner := &refusingClientRunner{}
	f.app.Lifecycle.ProjectRoot, f.app.Lifecycle.StateStore = project, f.store
	f.app.Lifecycle.Planner, f.app.Lifecycle.Targets = planner, planner
	f.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: runner})
	f.app.Lifecycle.Lock = processlock.Lock{Path: filepath.Join(stateRoot, "mutation.lock")}
	f.app.Lifecycle.Kernel = transaction.Kernel{StateStore: f.store, Directory: dirswap.Manager{JournalDir: filepath.Join(stateRoot, "operations")}}
	f.app.Lifecycle.PluginData = providers.PluginDataManager{Base: filepath.Join(stateRoot, "plugin-data")}
	f.app.Installer, err = installer.New(installer.Config{ProjectRoot: project, StateRoot: stateRoot, Registry: f.app.ClientRegistry, Runner: runner, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	f.app.Lifecycle.CheckNamespace = f.app.Installer.CheckProjectNamespace

	source := writeCLIPlugin(t)
	if err := os.MkdirAll(filepath.Join(source, "skills", "guide"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "skills", "guide", "SKILL.md"), []byte("---\nname: guide\ndescription: Inert doctor fixture\n---\nBody.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scope := []string{"--scope", "project", "--project-root", project, "--target", publicCLITargetName(clientID), "--format", "json"}
	if stdout, stderr, err := f.execute(false, append([]string{"add", source, "--no-save"}, scope...)...); err != nil {
		t.Fatalf("project add: %v\n%s\n%s", err, stdout, stderr)
	}
	runner.calls = 0
	projectionFindings := func() []string {
		t.Helper()
		stdout, stderr, err := f.execute(false, append([]string{"doctor", "demo"}, scope...)...)
		if err != nil {
			t.Fatalf("doctor: %v\n%s\n%s", err, stdout, stderr)
		}
		var output struct {
			Data struct {
				Findings []doctorFinding `json:"findings"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatal(err)
		}
		codes := []string{}
		for _, finding := range output.Data.Findings {
			if finding.Code == "native_projection_changed" || finding.Code == "native_projection_not_checked" {
				codes = append(codes, finding.Status+":"+finding.Code)
			}
		}
		return codes
	}
	if codes := projectionFindings(); len(codes) != 0 {
		t.Fatalf("intact %s project projection was diagnosed as %v", clientID, codes)
	}
	if runner.calls != 0 {
		t.Fatalf("read-only doctor started the %s client %d times", clientID, runner.calls)
	}
	skill := filepath.Join(project, filepath.FromSlash(skills), "guide", "SKILL.md")
	if err := os.WriteFile(skill, []byte("---\nname: guide\ndescription: Foreign edit\n---\nChanged.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if codes := projectionFindings(); len(codes) != 1 || codes[0] != "degraded:native_projection_changed" {
		t.Fatalf("changed %s project skill was diagnosed as %v", clientID, codes)
	}
}
