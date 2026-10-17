package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	clientregistry "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/hostprep"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/plannertest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

// rulesPlanner stands in for an installer release whose command rules differ
// from the one that installed the package: while withhold is set it drops
// the ordinary command's skill from every plan.
type rulesPlanner struct {
	ports.DeliveryPlanner
	withhold *bool
}

func (planner rulesPlanner) Plan(ctx context.Context, request domain.PlanRequest) (domain.DeliveryPlan, error) {
	plan, err := planner.DeliveryPlanner.Plan(ctx, request)
	if err == nil && *planner.withhold {
		plan.Components = slices.DeleteFunc(plan.Components, func(component domain.ComponentDecision) bool {
			return component.Source == domain.ComponentSourceCommand
		})
	}
	return plan, err
}

func withRules(service Service, withhold bool) (Service, *bool) {
	rules := &withhold
	service.Planner = rulesPlanner{DeliveryPlanner: service.Planner, withhold: rules}
	return service, rules
}

// commandInput is a package with one authored skill, so tightened rules
// still leave a deliverable plan, and one ordinary command.
func commandInput(t *testing.T, client domain.DetectedClient) AddInput {
	t.Helper()
	input := addInput(t, client, "https://example.com/commands")
	skill := []byte("---\nname: guide\ndescription: Guide\n---\nRead the guide.\n")
	if err := os.MkdirAll(filepath.Join(input.Envelope.SnapshotRoot, "skills", "guide"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "skills", "guide", "SKILL.md"), skill, 0o644); err != nil {
		t.Fatal(err)
	}
	input.Envelope.Skills = map[string]domain.Skill{"guide": {Name: "guide", Description: "Guide", RelativePath: "skills/guide/SKILL.md", Raw: skill}}
	input.Envelope.Inventory.Skills = []string{"guide"}
	input.Envelope.Commands = map[string]domain.Command{"echo": {
		Name: "echo", Description: "Echo the text", Body: []byte("Reply with UAP-ECHO.\n"),
		RelativePath: "io.github.777genius.agentplugins/commands/echo.md",
	}}
	input.Confirmed = true
	return input
}

func commandSkillPath(activePath string) string {
	return filepath.Join(activePath, "skills", "demo-echo", "SKILL.md")
}

func managedState(t *testing.T, service Service) domain.ClientBinding {
	t.Helper()
	state, err := service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	return onlyBinding(state.Installations[0])
}

// A newer installer that delivers a command the installed tree lacks: repair
// refuses, update refreshes first and is then idempotent, and repair passes
// again once the projection is current.
func TestUpdateRefreshesCommandProjection(t *testing.T) {
	t.Parallel()
	base, _, cursor := serviceFixture(t)
	service, withhold := withRules(base, true)
	input := commandInput(t, cursor)
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	active := installed.Plan.ActivePath
	assertUsecaseMissing(t, commandSkillPath(active))
	before := managedState(t, service)

	*withhold = false
	input.OperationID = "repair-outdated"
	if _, err := service.Repair(context.Background(), input); !errors.Is(err, ErrProjectionRefreshRequired) {
		t.Fatalf("repair of an outdated projection = %v", err)
	}
	preview := input
	preview.DryRun, preview.Confirmed = true, false
	if _, err := service.Repair(context.Background(), preview); err != nil {
		t.Fatal(err)
	}
	// The dry run is what the installer engine prepares from; an unchanged
	// answer there would skip the update entirely.
	if result, err := service.Update(context.Background(), preview); err != nil || result.NoChange || !result.RequiresConfirmation || result.Mutated {
		t.Fatalf("dry-run update = %+v, %v", result, err)
	}
	preview.DryRun = false
	if result, err := service.Update(context.Background(), preview); err != nil || !result.RequiresConfirmation || result.Mutated {
		t.Fatalf("unconfirmed update = %+v, %v", result, err)
	}
	if after := managedState(t, service); managedDigest(after) != managedDigest(before) || len(after.Receipts) != len(before.Receipts) {
		t.Fatalf("previews changed state: %+v", after)
	}
	assertUsecaseMissing(t, commandSkillPath(active))

	// The longest id a host may pass; the refresh derives its own from it.
	input.OperationID = strings.Repeat("u", 64)
	updated, err := service.Update(context.Background(), input)
	if err != nil || !updated.Mutated || updated.NoChange {
		t.Fatalf("update = %+v, %v", updated, err)
	}
	if _, err := os.Stat(commandSkillPath(active)); err != nil {
		t.Fatalf("refreshed command skill: %v", err)
	}
	refreshed := managedState(t, service)
	input.OperationID = "update-again"
	if again, err := service.Update(context.Background(), input); err != nil || again.Mutated {
		t.Fatalf("repeated update = %+v, %v", again, err)
	}
	if after := managedState(t, service); managedDigest(after) != managedDigest(refreshed) || len(after.Receipts) != len(refreshed.Receipts) {
		t.Fatalf("repeated update changed state: %+v", after)
	}
	input.OperationID = "repair-current"
	if _, err := service.Repair(context.Background(), input); err != nil {
		t.Fatalf("repair of a current projection: %v", err)
	}
}

// Tightened rules take a delivered command skill away again, from the
// package and, for a native client, from the client's own directory.
func TestUpdateRemovesWithheldCommandSkill(t *testing.T) {
	t.Parallel()
	t.Run("claude", func(t *testing.T) {
		t.Parallel()
		base, _, _ := serviceFixture(t)
		config := filepath.Join(t.TempDir(), "claude-config")
		client := domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected, ConfigRoot: config, ExecutablePath: "/test/bin/claude"}
		planner := plannertest.NewPlanner(clientplanner.Planner{ManagedRoot: t.TempDir()})
		runner := &fakeClaudeLifecycleRunner{configRoot: config}
		stager := providerstest.NewStager(providers.Stager{})
		base.Planner, base.Targets, base.Stager = planner, planner, stager
		base.Activator = providerstest.NewActivator(providers.Activator{Runner: runner})
		base.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Runner: runner, Stager: stager})
		service, withhold := withRules(base, false)
		input := commandInput(t, client)
		input.BackendExecutable = client.ExecutablePath
		installed, err := service.Add(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		skill := filepath.Join(installed.Plan.ActivePath, ".agentplugins-runtime", "skills", "demo-echo", "SKILL.md")
		if _, err := os.Stat(skill); err != nil {
			t.Fatalf("installed command skill: %v", err)
		}
		*withhold = true
		input.OperationID = "update-tightened"
		if result, err := service.Update(context.Background(), input); err != nil || !result.Mutated {
			t.Fatalf("update = %+v, %v", result, err)
		}
		assertUsecaseMissing(t, skill)
	})
	t.Run("kiro", func(t *testing.T) {
		t.Parallel()
		base, _, _ := serviceFixture(t)
		config := filepath.Join(t.TempDir(), ".kiro")
		service, withhold := withRules(base, false)
		input := commandInput(t, domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: config})
		installed, err := service.Add(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		native := filepath.Join(config, "skills", "demo-echo")
		for _, path := range []string{commandSkillPath(installed.Plan.ActivePath), native} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("installed command skill: %v", err)
			}
		}
		*withhold = true
		input.OperationID = "update-tightened"
		if result, err := service.Update(context.Background(), input); err != nil || !result.Mutated {
			t.Fatalf("update = %+v, %v", result, err)
		}
		assertUsecaseMissing(t, commandSkillPath(installed.Plan.ActivePath))
		assertUsecaseMissing(t, native)
	})
}

// Under unchanged rules the refresh before an update stages the same tree
// and stops there. Kiro keeps a prepared activation as its settled state,
// which must not pass for an unfinished one and be activated again.
func TestUpdateUnderUnchangedRulesKeepsState(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	input := commandInput(t, domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")})
	input.InstallIntent = domain.InstallIntentPrepare
	if installed, err := service.Add(context.Background(), input); err != nil || installed.Activation.Activation != domain.ActivationPrepared {
		t.Fatalf("add = %+v, %v", installed.Activation, err)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	input.OperationID = "update-unchanged"
	if result, err := service.Update(context.Background(), input); err != nil || result.Mutated {
		t.Fatalf("update = %+v, %v", result, err)
	}
	after, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("an update under unchanged rules rewrote state\nbefore=%s\nafter=%s", before, after)
	}
}

// The refresh ahead of an update exists for command rules only. A host input
// that changed since installation, here the opencode.jsonc that OpenCode
// creates on its first run and that its projection points at, is left to
// RefreshProjection, as for a package without commands: an unchanged update
// stays a no-op, single or grouped, and repair goes ahead.
func TestUpdateIgnoresHostDriftOutsideCommands(t *testing.T) {
	t.Parallel()
	for _, group := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "group"}[group], func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			service, store, _ := serviceFixture(t)
			preparer, err := hostprep.New(clientregistry.Default(), func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
				return bridgeEvidence("TEST-injected-identity"), nil
			}, []string{"PATH="})
			if err != nil {
				t.Fatal(err)
			}
			service.OpenCodeHosts = preparer
			service.Detected = map[domain.ClientID]domain.DetectedClient{}
			service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
			client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode"), ExecutablePath: filepath.Join(t.TempDir(), "opencode-bin")}
			input := commandInput(t, client)
			if _, err := service.AddGroup(ctx, GroupInput{Targets: []AddInput{input}, OperationGroupID: "host-drift-add", Confirmed: true}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(client.ConfigRoot, "opencode.jsonc"), []byte("{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			var mutated bool
			if group {
				result, err := service.UpdateGroup(ctx, GroupInput{Targets: []AddInput{input}, OperationGroupID: "host-drift-update", Confirmed: true})
				if err != nil {
					t.Fatal(err)
				}
				mutated = result.Mutated
			} else {
				input.OperationID = "host-drift-update"
				result, err := service.Update(ctx, input)
				if err != nil {
					t.Fatal(err)
				}
				mutated = result.Mutated
			}
			after, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			if mutated || !bytes.Equal(before, after) {
				t.Fatalf("unchanged update after host drift: mutated = %v\nbefore=%s\nafter=%s", mutated, before, after)
			}
			// Repair refuses only outdated command skills, so the drift does
			// not send it to an update that has nothing to do.
			input.OperationID = "host-drift-repair"
			if group {
				_, err = service.RepairGroup(ctx, GroupInput{Targets: []AddInput{input}, OperationGroupID: "host-drift-repair", Confirmed: true})
			} else {
				_, err = service.Repair(ctx, input)
			}
			if err != nil {
				t.Fatalf("repair after host drift: %v", err)
			}
		})
	}
}

// A repeated add keeps the installed tree. Once the rules render the
// commands differently, it stops with the typed error in a preview, in the
// resumed and the settled lifecycle and in a group, and leaves state alone;
// under the rules it was installed with it still passes.
func TestRepeatedAddRefusesAnOutdatedCommandProjection(t *testing.T) {
	t.Parallel()
	for name, intent := range map[string]domain.InstallIntent{"resumed": domain.InstallIntentAutomatic, "settled": domain.InstallIntentPrepare} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			base, store, _ := serviceFixture(t)
			service, withhold := withRules(base, true)
			input := commandInput(t, domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")})
			input.InstallIntent = intent
			if _, err := service.Add(ctx, input); err != nil {
				t.Fatal(err)
			}
			input.OperationID = "add-same-rules"
			if _, err := service.Add(ctx, input); err != nil {
				t.Fatalf("repeated add under the same rules: %v", err)
			}
			before, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			*withhold = false
			preview := input
			preview.DryRun, preview.Confirmed = true, false
			input.OperationID = "add-new-rules"
			attempts := map[string]func() error{
				"preview": func() error { _, err := service.Add(ctx, preview); return err },
				"add":     func() error { _, err := service.Add(ctx, input); return err },
				"group": func() error {
					_, err := service.AddGroup(ctx, GroupInput{Targets: []AddInput{input}, OperationGroupID: "group-new-rules", Confirmed: true})
					return err
				},
			}
			for label, attempt := range attempts {
				if err := attempt(); !errors.Is(err, ErrProjectionRefreshRequired) {
					t.Errorf("%s: %v", label, err)
				}
			}
			after, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("refused adds changed state\nbefore=%s\nafter=%s", before, after)
			}
		})
	}
}

// A group repair restores recorded bytes, as Repair does. After a rules
// change it stops before any commit: an intact package with the typed error,
// a removed one, which update cannot restore either, with advice to add it
// again. Under the rules it was installed with it still passes.
func TestRepairGroupRefusesAnOutdatedCommandProjection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repairGroup := func(service Service, input AddInput, operationGroupID string) error {
		_, err := service.RepairGroup(ctx, GroupInput{Targets: []AddInput{input}, OperationGroupID: operationGroupID, Confirmed: true})
		return err
	}
	t.Run("intact", func(t *testing.T) {
		t.Parallel()
		base, store, cursor := serviceFixture(t)
		service, withhold := withRules(base, true)
		input := commandInput(t, cursor)
		installed, err := service.Add(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		if err := repairGroup(service, input, "repair-same-rules"); err != nil {
			t.Fatalf("repair under the same rules: %v", err)
		}
		before, err := os.ReadFile(store.Path)
		if err != nil {
			t.Fatal(err)
		}
		*withhold = false
		if err := repairGroup(service, input, "repair-intact"); !errors.Is(err, ErrProjectionRefreshRequired) {
			t.Fatalf("repair of an intact outdated projection = %v", err)
		}
		assertUsecaseMissing(t, commandSkillPath(installed.Plan.ActivePath))
		if after, err := os.ReadFile(store.Path); err != nil || !bytes.Equal(before, after) {
			t.Fatalf("refused repair changed state: %v\nbefore=%s\nafter=%s", err, before, after)
		}
	})
	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		base, _, _ := serviceFixture(t)
		service, withhold := withRules(base, true)
		codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: canonicalCodexProfile(t, ".codex")}
		input := commandInput(t, codex)
		added, err := service.AddGroup(ctx, GroupInput{Targets: []AddInput{input}, OperationGroupID: "missing-add", Confirmed: true})
		if err != nil {
			t.Fatal(err)
		}
		active := added.Targets[0].Plan.ActivePath
		if err := os.RemoveAll(active); err != nil {
			t.Fatal(err)
		}
		service.NativeObserver = &recoveryProbeNativeObserver{stager: service.Stager}
		input.InstallationID = added.InstallationID
		*withhold = false
		if err := repairGroup(service, input, "repair-missing"); err == nil || errors.Is(err, ErrProjectionRefreshRequired) || !strings.Contains(err.Error(), "add it again") {
			t.Fatalf("repair of a missing outdated projection = %v", err)
		}
		assertUsecaseMissing(t, active)
	})
}

// A client whose registry entry withholds commands keeps update as for a
// package without commands: no refresh, so its preview stays unchanged and
// needs no confirmation.
func TestClientsWithoutCommandsSkipTheRefresh(t *testing.T) {
	t.Parallel()
	for _, id := range []domain.ClientID{domain.ClientCopilot, domain.ClientVSCode} {
		t.Run(string(id), func(t *testing.T) {
			t.Parallel()
			service, _, _ := serviceFixture(t)
			input := commandInput(t, domain.DetectedClient{ClientID: id, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), string(id))})
			if _, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{input}, OperationGroupID: "frozen-add", Confirmed: true}); err != nil {
				t.Fatal(err)
			}
			if _, eligible, err := service.projectionRefreshTarget(input); err != nil || eligible {
				t.Fatalf("refresh eligible = %v, %v", eligible, err)
			}
			preview, err := service.UpdateGroup(context.Background(), GroupInput{Targets: []AddInput{input}, OperationGroupID: "frozen-preview", DryRun: true})
			if err != nil || !preview.Targets[0].NoChange || preview.Targets[0].RequiresConfirmation {
				t.Fatalf("update preview = %+v, %v", preview.Targets[0], err)
			}
		})
	}
}

// A saved dependency cannot take new command rules through an update, so it
// is told that they wait for its next pin change instead of being sent to an
// update that cannot help; everyone else keeps the refresh error.
func TestSavedDependencyIsNotSentToUpdate(t *testing.T) {
	t.Parallel()
	authority := &domain.EnvironmentAuthority{}
	refresh := fmt.Errorf("target kiro: %w", ErrProjectionRefreshRequired)
	if got := forSavedDependency(refresh, groupEnvironmentAuthority(GroupInput{Targets: []AddInput{{}, {EnvironmentAuthority: authority}}})); !errors.Is(got, ErrProjectionPinChangeRequired) || errors.Is(got, ErrProjectionRefreshRequired) {
		t.Fatalf("saved dependency error = %v", got)
	}
	if got := forSavedDependency(refresh, nil); !errors.Is(got, ErrProjectionRefreshRequired) {
		t.Fatalf("plain installation error = %v", got)
	}
	other := errors.New("unrelated")
	if got := forSavedDependency(other, authority); !errors.Is(got, other) {
		t.Fatalf("unrelated error = %v", got)
	}
}

// Saved-dependency sync authorizes only "update", which RefreshProjection
// does not accept, so those bindings keep the ordinary update path.
func TestEnvironmentUpdateDoesNotRefresh(t *testing.T) {
	t.Parallel()
	base, _, cursor := serviceFixture(t)
	service, withhold := withRules(base, true)
	input := commandInput(t, cursor)
	if _, err := service.Add(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	*withhold = false
	if _, eligible, err := service.projectionRefreshTarget(input); err != nil || !eligible {
		t.Fatalf("plain update eligible = %v, %v", eligible, err)
	}
	input.EnvironmentAuthority = &domain.EnvironmentAuthority{}
	if _, eligible, err := service.projectionRefreshTarget(input); err != nil || eligible {
		t.Fatalf("environment update eligible = %v, %v", eligible, err)
	}
}

func TestUpdateGroupRefreshesCommandProjection(t *testing.T) {
	t.Parallel()
	base, _, cursor := serviceFixture(t)
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	service, withhold := withRules(base, true)
	targets := []AddInput{commandInput(t, cursor), commandInput(t, kiro)}
	for index := range targets {
		targets[index].Envelope = targets[0].Envelope
	}
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: targets, OperationGroupID: "group-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range added.Targets {
		assertUsecaseMissing(t, commandSkillPath(target.Plan.ActivePath))
	}
	*withhold = false
	// Both preview forms: the CLI batch and update --all plan with DryRun and
	// skip the package when every target is unchanged, as does the engine.
	for _, preview := range []GroupInput{
		{Targets: targets, OperationGroupID: "group-preview"},
		{Targets: targets, OperationGroupID: "group-dry-run", DryRun: true},
	} {
		planned, err := service.UpdateGroup(context.Background(), preview)
		if err != nil || planned.Mutated {
			t.Fatalf("group preview = %+v, %v", planned, err)
		}
		for index, target := range planned.Targets {
			if target.NoChange || !target.RequiresConfirmation {
				t.Fatalf("group preview target %d = %+v", index, target)
			}
		}
	}
	for _, target := range added.Targets {
		assertUsecaseMissing(t, commandSkillPath(target.Plan.ActivePath))
	}
	// The longest group id the group stage accepts; each refresh derives its own from it.
	updated, err := service.UpdateGroup(context.Background(), GroupInput{Targets: targets, OperationGroupID: strings.Repeat("g", 60), Confirmed: true})
	if err != nil || !updated.Mutated || !updated.Targets[0].Mutated || !updated.Targets[1].Mutated {
		t.Fatalf("group update = %+v, %v", updated, err)
	}
	for _, target := range added.Targets {
		if _, err := os.Stat(commandSkillPath(target.Plan.ActivePath)); err != nil {
			t.Fatalf("%s command skill: %v", target.Plan.ClientID, err)
		}
	}
}

// A refresh that fails part way stops the group before its update starts. The
// result reports what the earlier refreshes already changed, the failed
// target, and every target the stop left untouched.
func TestUpdateGroupStopsAtAFailedRefresh(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]int{"first target": 0, "second target": 1} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base, _, cursor := serviceFixture(t)
			kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
			service, withhold := withRules(base, true)
			targets := []AddInput{commandInput(t, cursor), commandInput(t, kiro)}
			for index := range targets {
				targets[index].Envelope = targets[0].Envelope
			}
			added, err := service.AddGroup(context.Background(), GroupInput{Targets: targets, OperationGroupID: "group-add", Confirmed: true})
			if err != nil {
				t.Fatal(err)
			}
			*withhold = false
			if err := os.WriteFile(filepath.Join(added.Targets[broken].Plan.ActivePath, "plugin.json"), []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := service.UpdateGroup(context.Background(), GroupInput{Targets: targets, OperationGroupID: "group-partial", Confirmed: true})
			failed := result.Targets[broken]
			if err == nil || failed.Mutated || failed.GroupPhase != GroupTargetExternalFailed || failed.Failure == nil {
				t.Fatalf("partial group refresh = %+v, %v", result, err)
			}
			// Hosts match targets by client, so every one of them names it.
			for index, target := range result.Targets {
				if target.Plan.ClientID != targets[index].Client.ClientID || target.Plan.Scope != targets[index].Scope {
					t.Fatalf("target %d does not name its client: %+v", index, target.Plan)
				}
			}
			if broken == 0 {
				if result.Phase != GroupPhasePreparationFailed || result.Mutated || result.Targets[1].GroupPhase != GroupTargetExternalNotAttempted {
					t.Fatalf("group stopped at its first refresh = %+v", result)
				}
				return
			}
			if result.Phase != GroupPhaseExternalPartialFailure || !result.Mutated || !result.Targets[0].Mutated || result.Targets[0].GroupPhase != GroupTargetExternalCompleted {
				t.Fatalf("group stopped after a changing refresh = %+v", result)
			}
		})
	}
}

// When the group update fails after a refresh already changed a target, the
// group is a partial failure unless the update reports a later stage itself,
// and an update that returned no targets still shows the refreshed ones.
func TestMergeRefreshedGroupKeepsRefreshChanges(t *testing.T) {
	t.Parallel()
	refreshed := GroupResult{InstallationID: "installation", Mutated: true, Targets: []AddResult{{Mutated: true}, {}}}
	failure := errors.New("group update failed")
	for _, phase := range []GroupPhase{"", GroupPhasePreparationFailed, GroupPhaseManagedRolledBack} {
		merged := mergeRefreshedGroup(GroupResult{Phase: phase, Targets: []AddResult{{NoChange: true}, {NoChange: true}}}, refreshed, failure)
		if merged.Phase != GroupPhaseExternalPartialFailure || !merged.Mutated || !merged.Targets[0].Mutated || merged.Targets[0].NoChange || !merged.Targets[1].NoChange {
			t.Fatalf("after %q: %+v", phase, merged)
		}
	}
	if merged := mergeRefreshedGroup(GroupResult{Phase: GroupPhaseManagedActivationFailed}, refreshed, failure); merged.Phase != GroupPhaseManagedActivationFailed || merged.InstallationID != "installation" || len(merged.Targets) != 2 {
		t.Fatalf("later stage or missing targets: %+v", merged)
	}
	if merged := mergeRefreshedGroup(GroupResult{Phase: GroupPhaseCompleted, Targets: []AddResult{{}, {}}}, refreshed, nil); merged.Phase != GroupPhaseCompleted {
		t.Fatalf("successful update: %+v", merged)
	}
}

func assertUsecaseMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists: %v", path, err)
	}
}
