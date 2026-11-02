package main

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProjectKimiEmptyVersionLifecycleRequestsOnlySelectedProbe(t *testing.T) {
	for _, saved := range []bool{false, true} {
		name := "direct-local"
		if saved {
			name = "saved-git"
		}
		t.Run(name, func(t *testing.T) { projectKimiEmptyVersionLifecycle(t, saved) })
	}
}

func projectKimiEmptyVersionLifecycle(t *testing.T, saved bool) {
	t.Helper()
	journey, detector := newProjectKimiVersionJourney(t)
	userBefore := securityFixtureTree(t, detector.user)
	add := []string{"add", journey.transport.repo, "--target", "kimi", "--no-save"}
	selector := "project-demo"
	if saved {
		add = []string{"add", journey.transport.endpoint, "--alias", "docs", "--target", "kimi"}
		selector = "docs"
	}
	for _, project := range []string{journey.a, journey.b} {
		before := len(detector.targeted)
		journey.run(t, project, add...)
		detector.assertCalls(t, before, 1)
	}
	installation, binding := projectKimiVersionOwned(t, journey, projectJourneySkill, saved)
	if !saved {
		selector = installation.InstallationID
	}
	projectJourneyDataMarker(t, journey, journey.a, installation)
	bBefore := securityFixtureTree(t, journey.b)
	preserved := func() {
		t.Helper()
		if !reflect.DeepEqual(userBefore, securityFixtureTree(t, detector.user)) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) {
			t.Fatal("A version-assisted lifecycle changed original-user or B authority")
		}
	}
	if saved {
		before := len(detector.targeted)
		state := projectJourneyRead(t, filepath.Join(journey.namespace(journey.a), "state-v2.json"))
		journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
		detector.assertCalls(t, before, 0)
		if state != projectJourneyRead(t, filepath.Join(journey.namespace(journey.a), "state-v2.json")) {
			t.Fatal("unchanged saved Kimi reconciliation rewrote state")
		}
	}
	for _, args := range [][]string{{"list"}, {"info", installation.InstallationID}, {"doctor"}} {
		before := len(detector.targeted)
		journey.run(t, journey.a, args...)
		detector.assertCalls(t, before, 0)
	}
	if saved {
		before := len(detector.targeted)
		journey.run(t, journey.a, "deps", "status")
		detector.assertCalls(t, before, 0)
	}
	// Explicit info reconciliation already permits the selected version probe.
	before := len(detector.targeted)
	journey.run(t, journey.a, "info", installation.InstallationID, "--target", "kimi")
	detector.assertCalls(t, before, 1)
	for _, args := range [][]string{
		append(append([]string{}, add...), "--dry-run"),
		{"update", selector, "--target", "kimi", "--dry-run"},
		{"repair", installation.InstallationID, "--target", "kimi", "--dry-run"},
	} {
		before := len(detector.targeted)
		snapshot := securityFixtureTree(t, journey.a)
		_, err := journey.execute(journey.a, args...)
		if err != nil && !strings.Contains(err.Error(), "kimi project components require") {
			t.Fatalf("unexpected dry-run failure: %v", err)
		}
		detector.assertCalls(t, before, 0)
		if !reflect.DeepEqual(snapshot, securityFixtureTree(t, journey.a)) {
			t.Fatal("dry-run changed project authority")
		}
	}
	// Intact repair exercises public restaging and the missing-version gate;
	// the existing Kimi lifecycle retains its separate corrupted-pin repair case.
	before = len(detector.targeted)
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "kimi")
	detector.assertCalls(t, before, 1)
	projectKimiVersionOwned(t, journey, projectJourneySkill, saved)
	preserved()
	projectJourneyWrite(t, filepath.Join(journey.transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	if saved {
		projectJourneyGit(t, journey.transport.repo, "add", ".")
		projectJourneyGit(t, journey.transport.repo, "commit", "-m", "advance Kimi selected-version fixture")
		projectJourneyGit(t, journey.transport.repo, "push", "--quiet", journey.transport.remote, "main")
	}
	before = len(detector.targeted)
	journey.run(t, journey.a, "update", selector, "--target", "kimi")
	detector.assertCalls(t, before, 1)
	updated, current := projectKimiVersionOwned(t, journey, projectJourneyAdvancedSkill, saved)
	if updated.Source.TreeDigest == installation.Source.TreeDigest || current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator {
		t.Fatal("version-assisted update lost selected source/binding identity")
	}
	for key, receipt := range installation.DataReceipts {
		if updated.DataReceipts[key].Locator != receipt.Locator || projectJourneyRead(t, filepath.Join(receipt.Locator, "keep.txt")) != "retained environment data\n" {
			t.Fatal("version-assisted update changed retained data")
		}
	}
	preserved()
	before = len(detector.targeted)
	journey.run(t, journey.a, "remove", selector, "--target", "kimi")
	detector.assertCalls(t, before, 0)
	projectJourneyAbsent(t, binding.TargetLocator, filepath.Join(journey.a, ".kimi-code", "skills", "docs"))
	preserved()
	afterRemoval := securityFixtureTree(t, journey.a)
	_, _ = journey.execute(journey.a, "remove", selector, "--target", "kimi")
	detector.assertCalls(t, before, 0)
	if !reflect.DeepEqual(afterRemoval, securityFixtureTree(t, journey.a)) {
		t.Fatal("repeated Kimi removal changed bytes")
	}
}

func TestProjectKimiEmptyVersionProbeErrorAndOtherSelectedFailurePreserveProject(t *testing.T) {
	for _, kind := range []string{"probe-error", "other-selected-error"} {
		t.Run(kind, func(t *testing.T) {
			journey, detector := newProjectKimiVersionJourney(t)
			target, calls := "kimi", 1
			if kind == "probe-error" {
				detector.probeError = errors.New("selected Kimi fixture probe failed")
			} else {
				detector.geminiError = errors.New("selected Gemini fixture detection failed")
				target, calls = "kimi,gemini", 0
			}
			beforeA, beforeB, beforeUser := securityFixtureTree(t, journey.a), securityFixtureTree(t, journey.b), securityFixtureTree(t, detector.user)
			_, err := journey.execute(journey.a, "add", journey.transport.repo, "--target", target, "--no-save")
			if err == nil {
				t.Fatal("failed selected detection unexpectedly installed")
			}
			detector.assertCalls(t, 0, calls)
			if !reflect.DeepEqual(beforeA, securityFixtureTree(t, journey.a)) || !reflect.DeepEqual(beforeB, securityFixtureTree(t, journey.b)) || !reflect.DeepEqual(beforeUser, securityFixtureTree(t, detector.user)) {
				t.Fatal("failed selected detection changed native/project authority")
			}
		})
	}
}

func TestProjectKimiEmptyVersionUserAndUnselectedOperationsDoNotProbe(t *testing.T) {
	journey, detector := newProjectKimiVersionJourney(t)
	journey.run(t, "", "add", journey.transport.repo, "--target", "kimi", "--no-save")
	detector.assertCalls(t, 0, 0)
	journey.run(t, journey.a, "add", journey.transport.repo, "--target", "gemini", "--no-save")
	detector.assertCalls(t, 0, 0)
}
