package usecase

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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

	input.OperationID = "update-rules"
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
	updated, err := service.UpdateGroup(context.Background(), GroupInput{Targets: targets, OperationGroupID: "group-update", Confirmed: true})
	if err != nil || !updated.Mutated || !updated.Targets[0].Mutated || !updated.Targets[1].Mutated {
		t.Fatalf("group update = %+v, %v", updated, err)
	}
	for _, target := range added.Targets {
		if _, err := os.Stat(commandSkillPath(target.Plan.ActivePath)); err != nil {
			t.Fatalf("%s command skill: %v", target.Plan.ClientID, err)
		}
	}
}

// A refresh that fails part way stops the group before its update starts
// and reports what the earlier refreshes already changed.
func TestUpdateGroupStopsAtAFailedRefresh(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(added.Targets[1].Plan.ActivePath, "plugin.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := service.UpdateGroup(context.Background(), GroupInput{Targets: targets, OperationGroupID: "group-partial", Confirmed: true})
	if err == nil || result.Phase != GroupPhaseExternalPartialFailure || !result.Mutated || !result.Targets[0].Mutated || result.Targets[1].Mutated {
		t.Fatalf("partial group refresh = %+v, %v", result, err)
	}
}

func assertUsecaseMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists: %v", path, err)
	}
}
