package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Retain the actual prior Claude directory projection and its prepared lifecycle
// in a normal SDK-owned installation. Current preparation uses the real adapter.
type legacySkillsClaudeAdapter struct{ *claude.Adapter }

func (legacySkillsClaudeAdapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	return nil, claude.ProjectClaude(in.StagingPath, in.Envelope, in.Plan, in.PluginDataPath)
}

func (legacySkillsClaudeAdapter) InstalledSkillsDirectory(domain.DeliveryPlan) string {
	return claude.ClaudeRuntimeDirectory + "/skills"
}

func (legacySkillsClaudeAdapter) Activate(context.Context, clients.Env, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	return domain.ActivationOutcome{Activation: domain.ActivationPrepared, Authentication: domain.AuthenticationNotChecked,
		Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectUnchanged}, nil
}

func legacySkillsClaudeFixture(t *testing.T) (savedJourney, SkillsBatchRequest) {
	t.Helper()
	_, journey, source, _ := managementMutableWebsite(t)
	base, err := filepath.EvalSymlinks(journey.cli.root)
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(base, "project")
	client := fixtureClient(t, domain.ClientClaude)
	client.ExecutablePath = filepath.Join(base, "inert-claude")
	dependencyWrite(t, client.ExecutablePath, "inert test artifact; never executed")
	if err := os.Chmod(client.ExecutablePath, 0700); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{project, client.ConfigRoot} {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
	}
	stateRoot := filepath.Join(project, ".uap")
	journey.cli.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json"), ProjectRoot: project}
	journey.cli.operations = filepath.Join(stateRoot, "operations-v2")
	app := &journey.cli.app
	app.Scope, app.ProjectRoot = domain.ScopeProject, project
	app.ManagedRoot, app.DependenciesRoot = filepath.Join(stateRoot, "managed"), stateRoot
	app.EnvironmentLockPath, app.EnvironmentStoreRoot = filepath.Join(stateRoot, "environment.lock"), filepath.Join(stateRoot, "store")
	app.StateStore = journey.cli.store
	app.Detector = staticDetector{clients: []domain.DetectedClient{client}}
	runner := &preparedClaudeCLIRunner{t: t, project: project, user: client.ConfigRoot, executable: client.ExecutablePath}
	setSkillsClaudeAdapter(t, &journey, runner, legacySkillsClaudeAdapter{claude.New()})
	request := SkillsBatchRequest{Selections: []SkillsSelection{{Website: source, Name: "pdf", Alias: "pdf"}}, TargetIDs: []string{"claude"}}
	plan, err := app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	result, err := plan.Apply(t.Context())
	if err != nil || len(result.Items) != 1 || result.Items[0].Client.Activation != string(domain.ActivationPrepared) {
		t.Fatalf("legacy Claude receipt fixture failed: %+v; %v", result, err)
	}
	setSkillsClaudeAdapter(t, &journey, runner, claude.New())
	return journey, request
}

func setSkillsClaudeAdapter(t *testing.T, journey *savedJourney, runner *preparedClaudeCLIRunner, adapter clients.Adapter) {
	t.Helper()
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	journey.cli.app.ClientRegistry = registry
	journey.cli.app.Installer, err = installer.New(installer.Config{ProjectRoot: journey.cli.app.ProjectRoot,
		StateRoot: journey.cli.app.DependenciesRoot, Registry: registry, Runner: runner,
		HelperExecutable: runner.executable, EnableNativeObserver: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSkillsResumeReviewsLegacyClaudeProjectionRefresh(t *testing.T) {
	journey, request := legacySkillsClaudeFixture(t)
	before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
	binding := savedJourneyBinding(t, journey.state(t), "claude")
	nativeBefore := savedJourneyTree(t, binding.TargetLocator)
	plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if actions := plan.Actions(); len(actions) != 1 || actions[0].Action != string(installer.OpRefreshProjection) || actions[0].TargetPath != binding.TargetLocator {
		t.Fatalf("legacy projection refresh was not explicitly reviewed: %+v", actions)
	}
	warnings := strings.Join(plan.Warnings(), "\n")
	if !strings.Contains(warnings, strconv.Quote(journey.cli.app.ProjectRoot)) || !strings.Contains(warnings, "trust") {
		t.Fatalf("review omitted the exact persistent project trust change: %s", warnings)
	}
	action := plan.request.prepared[0]
	if action.Authority.ExpectedAction != string(installer.OpRepair) || action.Authority.ReceiptID == "" ||
		action.Package.TreeDigest != binding.PackageRevision.TreeDigest || action.BindingID != binding.ClientBindingID {
		t.Fatalf("refresh lost the same-pin repair authority: %+v", action)
	}
	if err := plan.Check(t.Context()); err != nil {
		t.Fatal(err)
	}
	plan.Close() // Declining the visible refresh must preserve the entire state.
	if !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) || !reflect.DeepEqual(nativeBefore, savedJourneyTree(t, binding.TargetLocator)) {
		t.Fatal("preparing or declining refresh changed the legacy installation")
	}
}

func TestSkillsResumeLegacyClaudeRefusesDriftAndForeignReceipt(t *testing.T) {
	for _, scenario := range []string{"drift", "foreign", "stale"} {
		t.Run(scenario, func(t *testing.T) {
			journey, request := legacySkillsClaudeFixture(t)
			var retained *PreparedSkillsBatch
			if scenario == "stale" {
				var err error
				retained, err = journey.cli.app.PrepareSkills(t.Context(), request)
				if err != nil {
					t.Fatal(err)
				}
				defer retained.Close()
			}
			binding := savedJourneyBinding(t, journey.state(t), "claude")
			if scenario == "foreign" {
				state := journey.state(t)
				state.EnvironmentReceipts[0].EnvironmentID = "project:other-environment"
				if err := journey.cli.store.Save(state); err != nil {
					t.Fatal(err)
				}
			} else {
				dependencyWrite(t, filepath.Join(binding.TargetLocator, "user-edit.txt"), "preserve this edit")
			}
			before, nativeBefore := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot), savedJourneyTree(t, binding.TargetLocator)
			var err error
			if retained != nil {
				_, err = retained.Apply(t.Context())
				if !errors.Is(err, ErrSkillsPlanStale) {
					t.Fatalf("stale refresh accepted: %v", err)
				}
			} else {
				retained, err = journey.cli.app.PrepareSkills(t.Context(), request)
				if retained != nil {
					retained.Close()
				}
			}
			if err == nil || scenario != "stale" && !strings.Contains(err.Error(), scenario) ||
				!reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) || !reflect.DeepEqual(nativeBefore, savedJourneyTree(t, binding.TargetLocator)) {
				t.Fatalf("refused refresh changed old setup: %v", err)
			}
		})
	}
}
