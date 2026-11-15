package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type unchangedSkillsRunner struct {
	inner                   *savedJourneyRunner
	readOnly, unavailable   bool
	observations, mutations int
}

func (runner *unchangedSkillsRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if strings.Join(command.Argv[1:], " ") != "plugin list --json" {
		runner.mutations++
		if runner.readOnly {
			return legacyports.CommandResult{}, errors.New("unchanged Skill attempted a native mutation")
		}
	}
	if runner.readOnly {
		runner.observations++
		if runner.unavailable {
			return legacyports.CommandResult{}, errors.New("native inventory is unavailable")
		}
	}
	return runner.inner.Run(ctx, command)
}

func unchangedSkillsFixture(t *testing.T) (savedJourney, SkillsBatchRequest, *unchangedSkillsRunner) {
	t.Helper()
	_, journey, source, _ := managementMutableWebsite(t)
	runner := &unchangedSkillsRunner{inner: journey.runner}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := journey.cli.app.DependenciesRoot
	journey.cli.app.Installer, err = installer.New(installer.Config{
		StateRoot: root, StateFile: journey.cli.store.Path, LockFile: filepath.Join(root, "mutation.lock"),
		OperationsDir: journey.cli.operations, PluginDataBase: filepath.Join(root, "plugin-data"),
		ManagedRoot: journey.cli.app.ManagedRoot, TempRoot: filepath.Join(root, "installer-tmp"),
		Registry: journey.cli.app.ClientRegistry, HelperExecutable: helper, Runner: runner,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := SkillsBatchRequest{Selections: []SkillsSelection{{Website: source, Name: "pdf", Alias: "pdf"}}, TargetIDs: []string{"codex"}}
	plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	result, err := plan.Apply(t.Context())
	if err != nil || len(result.Items) != 1 || !skillsNativeReady(result.Items) {
		t.Fatalf("fixture did not install an observed native Skill: %+v; %v", result, err)
	}
	runner.readOnly = true
	runner.mutations = 0
	return journey, request, runner
}

// Uses real HTTPS acquisition, normalization, ownership and installer state.
// The existing native protocol fixture refuses every mutation during replay.
func TestSkillsUnchangedNativeObservationPreservesDurableBytes(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(map[bool]string{false: "observed", true: "unavailable"}[unavailable], func(t *testing.T) {
			journey, request, runner := unchangedSkillsFixture(t)
			runner.unavailable = unavailable
			binding := savedJourneyBinding(t, journey.state(t), "codex")
			before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
			nativeBefore := savedJourneyTree(t, binding.TargetLocator)
			plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			if len(plan.Actions()) != 1 || plan.Actions()[0].Action != string(environment.ActionUnchanged) {
				t.Fatalf("repeat scheduled a mutation: %+v", plan.Actions())
			}
			result, err := plan.Apply(t.Context())
			plan.Close()
			if err != nil || result.Status != "succeeded" || len(result.Items) != 1 || result.Items[0].Status != "unchanged" {
				t.Fatalf("repeat lost its no-op outcome: %+v; %v", result, err)
			}
			if skillsNativeReady(result.Items) == unavailable || runner.observations == 0 || runner.mutations != 0 {
				t.Fatalf("readiness lacks read-only native evidence: %+v; runner=%+v", result, runner)
			}
			if !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) ||
				!reflect.DeepEqual(nativeBefore, savedJourneyTree(t, binding.TargetLocator)) {
				t.Fatal("native observation changed saved intent, state, receipts or installed bytes")
			}
		})
	}
}

func TestSkillsUnchangedRejectsForeignAndUnprovedSavedBindings(t *testing.T) {
	journey, request, _ := unchangedSkillsFixture(t)
	plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	view, err := journey.cli.app.Installer.Inspect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	action := plan.request.prepared[0]
	path := plan.Actions()[0].TargetPath
	for _, scenario := range []string{"exact", "foreign-owner", "other-digest", "other-binding", "other-path", "prepared", "auth-pending", "attested", "pending-recovery"} {
		t.Run(scenario, func(t *testing.T) {
			candidate := action
			inspected := view
			inspected.Installations = append([]installer.InspectedInstallation(nil), view.Installations...)
			inspected.Installations[0].Bindings = append([]installer.InspectedBinding(nil), view.Installations[0].Bindings...)
			binding := &inspected.Installations[0].Bindings[0]
			owner := *binding.Owner
			binding.Owner = &owner
			switch scenario {
			case "foreign-owner":
				owner.EnvironmentID = "another-environment"
			case "other-digest":
				binding.TreeDigest = "sha256:" + strings.Repeat("f", 64)
			case "other-binding":
				binding.BindingID = "another-binding"
			case "other-path":
				binding.TargetPath += "-other"
			case "prepared":
				binding.Activation = string(domain.ActivationPrepared)
			case "auth-pending":
				binding.Authentication = string(domain.AuthenticationPending)
			case "attested":
				candidate.ActivationComplete = true
			case "pending-recovery":
				inspected.Recovery.Required = true
			}
			_, ready := unchangedSkillsClient(inspected, candidate, journey.cli.app.environmentID(), string(journey.cli.app.scope()), path)
			if ready != (scenario == "exact") {
				t.Fatalf("saved binding qualified=%v for %s", ready, scenario)
			}
		})
	}
}
