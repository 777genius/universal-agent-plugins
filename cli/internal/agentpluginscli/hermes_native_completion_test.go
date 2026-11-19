package agentpluginscli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Acquisition, review, public Cobra, the facade and owned-state completion are
// real. Only native process results are inert; this is not native qualification.
type hermesCatalogRunner struct {
	trust          *hermesCompletionRunner
	source, python string
	catalogs       int
}

func (runner *hermesCatalogRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	result := legacyports.CommandResult{CleanupComplete: true}
	switch {
	case slices.Equal(command.Argv[1:], []string{"--version"}):
		result.Stdout = []byte("Hermes Agent v0.23.0 (2026.9.24) · stable · local 2bd378ed1b8d\nInstall directory: " + runner.source + "\n")
	case slices.Equal(command.Argv[1:], []string{"--print-runtime-command"}):
		result.Stdout, _ = json.Marshal([]string{runner.python, "-I", "-c", "official native bootstrap"})
	case len(command.Argv) == 8 && command.Argv[0] == runner.python:
		var selected map[string]string
		if json.Unmarshal([]byte(command.Argv[6]), &selected) != nil || command.Argv[4] != runner.source || command.Argv[7] != runner.source ||
			!strings.Contains(command.Argv[3], "runtime_command") || !strings.Contains(command.Argv[5], "skills_tool._skill_catalog()") {
			return result, fmt.Errorf("native completion selected another runtime protocol")
		}
		runner.catalogs++
		rows := []map[string]any{}
		tier := 1
		if runner.trust.project != "" {
			tier = 0
		}
		for name, path := range selected {
			body, err := os.ReadFile(path)
			if err != nil {
				return result, err
			}
			rows = append(rows, map[string]any{"name": name, "path": path, "digest": fmt.Sprintf("%x", sha256.Sum256(body)), "tier": tier})
		}
		result.Stdout, _ = json.Marshal(map[string]any{"profile": runner.trust.profile, "project": runner.trust.project,
			"project_trusted": runner.trust.project != "" && slices.Contains(runner.trust.trusted, runner.trust.project), "skills": rows})
	default:
		return runner.trust.Run(ctx, command)
	}
	return result, nil
}

func hermesCatalogHost(t *testing.T, journey *savedJourney, scope domain.InstallScope) *hermesCatalogRunner {
	t.Helper()
	trust := setHermesCompletionHost(t, journey)
	app := &journey.cli.app
	clients, err := app.Detector.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(clients[0].ExecutablePath))
	runner := &hermesCatalogRunner{trust: trust, source: filepath.Join(root, "hermes-agent"), python: filepath.Join(root, "tools", "python", "python")}
	stateRoot := filepath.Dir(journey.cli.store.Path)
	if scope == domain.ScopeUser {
		trust.project = ""
		app.Scope, app.ProjectRoot = domain.ScopeUser, ""
		stateRoot = filepath.Join(app.UserHome, "uap")
		journey.cli.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json")}
		journey.cli.operations = filepath.Join(stateRoot, "operations")
		app.StateStore = journey.cli.store
		app.ManagedRoot, app.DependenciesRoot = filepath.Join(stateRoot, "managed"), stateRoot
		app.EnvironmentLockPath, app.EnvironmentStoreRoot = filepath.Join(stateRoot, "environment.lock"), filepath.Join(stateRoot, "store")
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	app.Installer, err = installer.New(installer.Config{ProjectRoot: app.ProjectRoot, UserHome: app.UserHome, StateRoot: stateRoot,
		Registry: app.ClientRegistry, Runner: runner, HelperExecutable: helper, EnableNativeObserver: true,
		LookupExecutable: func(string) (string, error) { return filepath.Join(app.UserHome, "git"), nil }})
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func TestHermesPublicSkillsRepeatUsesConfirmedNativeCompletion(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			_, journey, source, _ := managementMutableWebsite(t)
			runner := hermesCatalogHost(t, &journey, scope)
			request := SkillsBatchRequest{Selections: []SkillsSelection{{Website: source, Name: "pdf", Alias: "pdf"}}, TargetIDs: []string{"hermes"}}
			args := []string{"skills", "add", source, "--skill", "pdf", "--target", "hermes", "--no-ui", "--yes", "--scope", string(scope), "--format", "json"}
			if scope == domain.ScopeProject {
				args = append(args, "--project-root", journey.cli.app.ProjectRoot)
			}
			for attempt := range 2 {
				preview, err := journey.cli.app.PrepareSkills(t.Context(), request)
				if err != nil {
					t.Fatal(err)
				}
				if err := preview.Check(t.Context()); err != nil || runner.catalogs != attempt {
					t.Fatalf("review started native completion: catalogs=%d err=%v", runner.catalogs, err)
				}
				preview.Close()
				output := journey.run(t, args...)
				var response struct {
					Readiness   string        `json:"readiness"`
					NextActions []AgentAction `json:"next_actions"`
				}
				if json.Unmarshal([]byte(output), &response) != nil || response.Readiness != "ready" || len(response.NextActions) != 0 {
					t.Fatalf("completed native setup remained a manual step: %s", output)
				}
				binding := savedJourneyBinding(t, journey.state(t), "hermes")
				if binding.Activation != domain.ActivationActive || binding.Verification != domain.VerificationInstalled || runner.catalogs != attempt+1 {
					t.Fatalf("confirmed public add did not complete exact native Skills: %+v catalogs=%d", binding, runner.catalogs)
				}
				if attempt == 0 {
					before := binding
					requirements, lock := journey.intent(t)
					t.Cleanup(func() {
						after := savedJourneyBinding(t, journey.state(t), "hermes")
						freshRequirements, freshLock := journey.intent(t)
						if before.ClientBindingID != after.ClientBindingID || !reflect.DeepEqual(before.PackageRevision, after.PackageRevision) ||
							!reflect.DeepEqual(requirements, freshRequirements) || !reflect.DeepEqual(lock, freshLock) {
							t.Error("native repeat changed binding, source pin or portable intent")
						}
					})
					runner.trust.trusted = nil // Explicit re-add may restore this native user revocation.
				}
			}
			if scope == domain.ScopeUser && runner.trust.grants != 0 || scope == domain.ScopeProject && runner.trust.grants != 2 {
				t.Fatal("native completion changed trust outside explicit Project adds", runner.trust.grants)
			}
		})
	}
}
