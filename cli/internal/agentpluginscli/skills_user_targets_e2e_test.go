package agentpluginscli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// A raw Agent Skill is a different source type from a plugin package. Exercise
// both user targets through the existing public dependency/installer seams
// without requiring credentials or executing downloaded Skill instructions.
func TestUserRawSkillCodexClaudePinnedLifecycle(t *testing.T) {
	j, _, commit := skillJourney(t)
	ctx := t.Context()
	input := SkillsBatchRequest{
		Selections: []SkillsSelection{{
			Repository: "acme/tools", Commit: commit, Path: "skills/pdf",
			Name: "pdf", Selector: environment.Selector{Kind: environment.SelectorRef, Value: "main"},
		}},
		TargetIDs: []string{"codex", "claude"},
	}
	plan, err := j.cli.app.PrepareSkills(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if err := plan.Check(ctx); err != nil {
		t.Fatalf("fresh user plan rejected: %v", err)
	}
	actions := plan.Actions()
	if len(actions) != 2 || actions[0].ClientID == actions[1].ClientID {
		t.Fatalf("raw Skill did not plan exactly two independent targets: %+v", actions)
	}
	for _, action := range actions {
		if action.TargetPath == "" || action.Alias != "pdf" {
			t.Fatalf("raw Skill planned an invalid target: %+v", action)
		}
	}
	manifest, lockFile := j.cli.app.dependencyPaths()
	for _, path := range []string{manifest, lockFile} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("prepare wrote intent before confirmation at %s: %v", path, statErr)
		}
	}
	if len(j.state(t).EnvironmentReceipts) != 0 {
		t.Fatal("prepare wrote persistent ownership")
	}
	result, err := plan.Apply(ctx)
	if err != nil || result.Status != "succeeded" || len(result.Items) != 2 {
		t.Fatalf("two-target user install failed: %+v; %v", result, err)
	}
	if err = plan.Check(ctx); err == nil {
		t.Fatal("applied plan was accepted again")
	}

	requirements, locked := j.intent(t)
	if len(locked.Packages) != 1 || locked.Packages[0].Source.Git == nil ||
		locked.Packages[0].Source.Git.Kind != environment.SkillKind ||
		locked.Packages[0].Source.Git.Commit != commit ||
		len(requirements.Targets) != 2 {
		t.Fatalf("user raw Skill identity/targets were not pinned: %+v; %+v", requirements, locked)
	}
	state := j.state(t)
	if len(state.EnvironmentReceipts) != 2 {
		t.Fatalf("expected two owned receipts: %+v", state.EnvironmentReceipts)
	}
	for _, target := range []string{"codex", "claude"} {
		binding := savedJourneyBinding(t, state, target)
		if binding.Scope != string(domain.ScopeUser) || binding.ClientBindingID == "" {
			t.Fatalf("%s has wrong user scope/identity: %+v", target, binding)
		}
		body, readErr := os.ReadFile(filepath.Join(binding.TargetLocator, "skills", "pdf", "references", "note.txt"))
		if readErr != nil || string(body) != "original resource" {
			t.Fatalf("%s lost pinned raw Skill resources: %q; %v", target, body, readErr)
		}
	}

	// In the public CLI, global is the default user scope. Replaying with an
	// explicit --scope user must preserve the same exact physical state.
	before := savedJourneyTree(t, j.cli.app.DependenciesRoot)
	j.run(t, "deps", "sync", "--locked", "--yes", "--scope", "user", "--format", "json")
	if after := savedJourneyTree(t, j.cli.app.DependenciesRoot); !reflect.DeepEqual(before, after) {
		t.Fatalf("explicit user replay changed locked environment: %s", savedJourneyDifferences(before, after))
	}
	if _, _, err := j.cli.execute(false, "skills", "add", "invalid.example/skills", "--scope", "user", "--project-root", t.TempDir(), "--skill", "pdf", "--target", "codex", "--yes"); err == nil {
		t.Fatal("contradictory user scope and project root passed validation")
	}
	if _, _, err := j.cli.execute(false, "skills", "add", "invalid.example/skills", "--scope", "global", "--skill", "pdf", "--target", "codex", "--yes"); err == nil {
		t.Fatal("unknown global scope silently mapped to a user install")
	}
	if after := savedJourneyTree(t, j.cli.app.DependenciesRoot); !reflect.DeepEqual(before, after) {
		t.Fatal("invalid scope request changed user state")
	}

	// Removing one target may not remove the other one's exact owned bytes.
	claudeBefore := savedJourneyBinding(t, state, "claude")
	j.run(t, "remove", "pdf", "--target", "codex", "--external-uninstalled", "--scope", "user", "--format", "json")
	afterCodex := j.state(t)
	if !reflect.DeepEqual(claudeBefore, savedJourneyBinding(t, afterCodex, "claude")) {
		t.Fatal("removing Codex changed Claude's untouched binding")
	}
	if len(afterCodex.EnvironmentReceipts) != 1 {
		t.Fatal("removing Codex damaged Claude's receipt")
	}
	j.run(t, "remove", "pdf", "--target", "claude", "--external-uninstalled", "--scope", "user", "--format", "json")
	if remaining := j.state(t).EnvironmentReceipts; len(remaining) != 0 {
		t.Fatalf("raw Skill removal left live ownership receipts: %+v", remaining)
	}
}
