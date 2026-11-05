package agentpluginscli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Advisory environment.lock records a short-lived concurrency lease. Its
// timestamps and body may change during an idempotent command, without any
// change to durable manifests, pins, state or installed native artifacts.
// Assert the complete persistent tree, excluding only that lease file.
func savedSkillsDurableTree(t *testing.T, root string) map[string]savedJourneyFile {
	t.Helper()
	files := savedJourneyTree(t, root)
	delete(files, "environment.lock")
	return files
}

// A raw Agent Skill is a different source type from a plugin package. Exercise
// both user targets through the existing public dependency/installer seams
// without requiring credentials or executing downloaded Skill instructions.
func TestUserRawSkillCodexClaudePinnedLifecycle(t *testing.T) {
	j, repo, commit := skillJourney(t)
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
	// A clean manifest/lock is insufficient evidence: check the native
	// projection trees and unrelated neighbor bytes independently.
	managedBefore := make(map[string]map[string]savedJourneyFile)
	foreignNeighbors := make(map[string]string)
	for _, target := range []string{"codex", "claude"} {
		binding := savedJourneyBinding(t, state, target)
		if binding.Scope != string(domain.ScopeUser) || binding.ClientBindingID == "" {
			t.Fatalf("%s has wrong user scope/identity: %+v", target, binding)
		}
		// Claude's official plugin layout keeps the portable raw Skill below
		// a manager-owned runtime directory; Codex stages it at package root.
		skillRoot := binding.TargetLocator
		if target == "claude" {
			skillRoot = filepath.Join(skillRoot, ".agentplugins-runtime")
		}
		body, readErr := os.ReadFile(filepath.Join(skillRoot, "skills", "pdf", "references", "note.txt"))
		if readErr != nil || string(body) != "original resource" {
			t.Fatalf("%s lost pinned raw Skill resources: %q; %v", target, body, readErr)
		}
		managedBefore[target] = savedJourneyTree(t, binding.TargetLocator)
		// This file belongs to neither UAP nor the tested package; neither
		// target's removal may delete or overwrite it.
		neighbor := filepath.Join(filepath.Dir(binding.TargetLocator), "foreign-user-neighbor-"+target+".txt")
		if err := os.WriteFile(neighbor, []byte("foreign-owned-"+target), 0600); err != nil {
			t.Fatal(err)
		}
		foreignNeighbors[target] = neighbor
	}

	// In the public CLI, global is the default user scope. Replaying with an
	// explicit --scope user must preserve the same exact physical state.
	before := savedSkillsDurableTree(t, j.cli.app.DependenciesRoot)
	j.run(t, "deps", "sync", "--locked", "--yes", "--scope", "user", "--format", "json")
	if after := savedSkillsDurableTree(t, j.cli.app.DependenciesRoot); !reflect.DeepEqual(before, after) {
		t.Fatalf("explicit user replay changed locked environment: %s", savedJourneyDifferences(before, after))
	}
	for _, target := range []string{"codex", "claude"} {
		path := savedJourneyBinding(t, state, target).TargetLocator
		if after := savedJourneyTree(t, path); !reflect.DeepEqual(managedBefore[target], after) {
			t.Fatalf("explicit user replay changed %s native projection: %s", target, savedJourneyDifferences(managedBefore[target], after))
		}
	}
	if _, _, err := j.cli.execute(false, "skills", "add", "invalid.example/skills", "--scope", "user", "--project-root", t.TempDir(), "--skill", "pdf", "--target", "codex", "--yes"); err == nil {
		t.Fatal("contradictory user scope and project root passed validation")
	}
	if _, _, err := j.cli.execute(false, "skills", "add", "invalid.example/skills", "--scope", "global", "--skill", "pdf", "--target", "codex", "--yes"); err == nil {
		t.Fatal("unknown global scope silently mapped to a user install")
	}
	if after := savedSkillsDurableTree(t, j.cli.app.DependenciesRoot); !reflect.DeepEqual(before, after) {
		t.Fatal("invalid scope request changed user state")
	}

	// Upgrade one moving Git Skill for both clients in the same saved user
	// namespace. Lock and physical projections must agree on v2 while
	// binding IDs and target paths retain their established ownership.
	originalBindings := make(map[string]domain.ClientBinding, 2)
	for _, target := range []string{"codex", "claude"} {
		originalBindings[target] = savedJourneyBinding(t, state, target)
	}
	if err := os.WriteFile(filepath.Join(repo, "skills", "pdf", "references", "note.txt"), []byte("updated resource"), 0644); err != nil {
		t.Fatal(err)
	}
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "fixture: advance both user Skills")
	nextCommit := savedJourneyGit(t, repo, "rev-parse", "HEAD")
	j.run(t, "update", "pdf", "--scope", "user", "--format", "json")
	_, updatedLock := j.intent(t)
	if len(updatedLock.Packages) != 1 || updatedLock.Packages[0].Source.Git == nil ||
		updatedLock.Packages[0].Source.Git.Commit != nextCommit ||
		updatedLock.Packages[0].TreeDigest == locked.Packages[0].TreeDigest {
		t.Fatalf("two-target update did not advance immutable Skill pin: %+v", updatedLock)
	}
	updatedState := j.state(t)
	if len(updatedState.EnvironmentReceipts) != 2 {
		t.Fatalf("two-target update dropped ownership receipts: %+v", updatedState.EnvironmentReceipts)
	}
	for _, target := range []string{"codex", "claude"} {
		old := originalBindings[target]
		current := savedJourneyBinding(t, updatedState, target)
		if current.ClientBindingID != old.ClientBindingID || current.TargetLocator != old.TargetLocator ||
			current.PackageRevision == nil || current.PackageRevision.ResolvedRevision != nextCommit {
			t.Fatalf("%s update changed identity or lost pinned revision: before=%+v after=%+v", target, old, current)
		}
		skillRoot := current.TargetLocator
		if target == "claude" {
			skillRoot = filepath.Join(skillRoot, ".agentplugins-runtime")
		}
		body, err := os.ReadFile(filepath.Join(skillRoot, "skills", "pdf", "references", "note.txt"))
		if err != nil || string(body) != "updated resource" {
			t.Fatalf("%s did not install updated raw Skill resources: %q %v", target, body, err)
		}
	}
	updatedBytes := savedSkillsDurableTree(t, j.cli.app.DependenciesRoot)
	j.run(t, "deps", "sync", "--locked", "--yes", "--scope", "user", "--format", "json")
	if current := savedSkillsDurableTree(t, j.cli.app.DependenciesRoot); !reflect.DeepEqual(updatedBytes, current) {
		t.Fatalf("locked replay after two-target update rewrote user state: %s", savedJourneyDifferences(updatedBytes, current))
	}

	// A changed managed raw Skill must not be removed just because the
	// saved receipt names its directory. Prove the refusal through the public
	// CLI while the other installed user target remains byte-identical.
	claudeBefore := savedJourneyBinding(t, updatedState, "claude")
	managedBefore["claude"] = savedJourneyTree(t, claudeBefore.TargetLocator)
	codexBefore := savedJourneyBinding(t, updatedState, "codex")
	codexTree := savedJourneyTree(t, codexBefore.TargetLocator)
	foreignInside := filepath.Join(codexBefore.TargetLocator, "foreign-owned.txt")
	if err := os.WriteFile(foreignInside, []byte("owned by another user"), 0600); err != nil {
		t.Fatal(err)
	}
	guardedTree := savedJourneyTree(t, codexBefore.TargetLocator)
	guardedState := savedSkillsDurableTree(t, j.cli.app.DependenciesRoot)
	if _, _, err := j.cli.execute(false, "remove", "pdf", "--target", "codex", "--external-uninstalled", "--scope", "user", "--format", "json"); err == nil {
		t.Fatal("selected removal adopted a foreign file inside the raw Skill")
	}
	if after := savedSkillsDurableTree(t, j.cli.app.DependenciesRoot); !reflect.DeepEqual(guardedState, after) {
		t.Fatalf("refused Codex removal changed saved user state: %s", savedJourneyDifferences(guardedState, after))
	}
	if after := savedJourneyTree(t, codexBefore.TargetLocator); !reflect.DeepEqual(guardedTree, after) {
		t.Fatalf("refused Codex removal changed foreign/owned bytes: %s", savedJourneyDifferences(guardedTree, after))
	}
	if after := savedJourneyTree(t, claudeBefore.TargetLocator); !reflect.DeepEqual(managedBefore["claude"], after) {
		t.Fatal("refused Codex removal changed Claude user Skill")
	}
	if err := os.Remove(foreignInside); err != nil {
		t.Fatal(err)
	}
	if after := savedJourneyTree(t, codexBefore.TargetLocator); !reflect.DeepEqual(codexTree, after) {
		t.Fatal("removing only the injected fixture did not restore the managed Codex Skill")
	}
	// Removing one target may not remove the other one's exact owned bytes.
	j.run(t, "remove", "pdf", "--target", "codex", "--external-uninstalled", "--scope", "user", "--format", "json")
	afterCodex := j.state(t)
	if !reflect.DeepEqual(claudeBefore, savedJourneyBinding(t, afterCodex, "claude")) {
		t.Fatal("removing Codex changed Claude's untouched binding")
	}
	if len(afterCodex.EnvironmentReceipts) != 1 {
		t.Fatal("removing Codex damaged Claude's receipt")
	}
	if _, err := os.Lstat(savedJourneyBinding(t, state, "codex").TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("Codex managed raw Skill survived selected removal: %v", err)
	}
	if after := savedJourneyTree(t, claudeBefore.TargetLocator); !reflect.DeepEqual(managedBefore["claude"], after) {
		t.Fatalf("removing Codex changed Claude native bytes: %s", savedJourneyDifferences(managedBefore["claude"], after))
	}
	j.run(t, "remove", "pdf", "--target", "claude", "--external-uninstalled", "--scope", "user", "--format", "json")
	if remaining := j.state(t).EnvironmentReceipts; len(remaining) != 0 {
		t.Fatalf("raw Skill removal left live ownership receipts: %+v", remaining)
	}
	if _, err := os.Lstat(claudeBefore.TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("Claude managed raw Skill survived selected removal: %v", err)
	}
	for target, path := range foreignNeighbors {
		body, err := os.ReadFile(path)
		if err != nil || string(body) != "foreign-owned-"+target {
			t.Fatalf("%s removal modified an unowned native neighbor %s: %q; %v", target, path, body, err)
		}
	}
}
