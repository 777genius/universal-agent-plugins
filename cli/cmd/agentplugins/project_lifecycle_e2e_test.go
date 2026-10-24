package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// All composition, CLI selection, package loading, immutable storage, planner,
// installer and native filesystem transactions are production implementations.
// Git/ZIP use real TLS; only host detection and security assessment are inert
// fixture boundaries. No Gemini session or model process is certified here.
func TestProjectComposedSourceLifecycleSeparatesUserAndProjects(t *testing.T) {
	for _, kind := range []string{"git", "zip"} {
		t.Run(kind, func(t *testing.T) {
			transport := newProjectJourneyTransport(t, kind)
			journey := newProjectJourney(t, transport)
			for _, root := range []string{"", journey.a, journey.b} {
				journey.run(t, root, "add", transport.endpoint, "--alias", "docs", "--target", "gemini")
				journey.assertOwnedContext(t, root)
			}
			projectJourneySharedIdentity(t, journey)
			projectJourneyRepairAndRemove(t, journey)
		})
	}
}

func projectJourneySharedIdentity(t *testing.T, journey projectJourney) {
	t.Helper()
	user, _ := journey.owned(t, "")
	for _, root := range []string{journey.a, journey.b} {
		installation := journey.assertOwnedContext(t, root)
		if installation.Source.SourceBindingID != user.Source.SourceBindingID || installation.Source.CanonicalSource != user.Source.CanonicalSource || installation.Source.TreeDigest != user.Source.TreeDigest {
			t.Fatal("scope changed portable source/package identity")
		}
		projectJourneyDataMarker(t, journey, root, installation)
	}
	projectJourneyDataMarker(t, journey, "", user)
	userPair := journey.assertPortablePair(t, journey.cache)
	if userPair != journey.assertPortablePair(t, journey.a) || userPair != journey.assertPortablePair(t, journey.b) {
		t.Fatal("same selection produced a machine-specific agreement")
	}
	store := environmentstore.Store{Root: filepath.Join(journey.cache, "store")}
	cached, err := store.Get(context.Background(), domain.TreeDigestAlgorithm, user.Source.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	if projectJourneyRead(t, filepath.Join(cached.Root, "skills", "docs", "SKILL.md")) != projectJourneySkill {
		t.Fatal("shared immutable cache does not contain the installed pin")
	}
	for _, root := range []string{journey.a, journey.b} {
		projectJourneyAbsent(t, filepath.Join(root, ".uap", "store"))
	}
}

func projectJourneyDataMarker(t *testing.T, journey projectJourney, root string, installation domain.Installation) {
	t.Helper()
	if len(installation.DataReceipts) != 1 {
		t.Fatalf("expected scoped persistent data: %+v", installation.DataReceipts)
	}
	for _, receipt := range installation.DataReceipts {
		if err := pathpolicy.RequireContainedChild(filepath.Join(journey.namespace(root), "plugin-data"), receipt.Locator); err != nil {
			t.Fatal(err)
		}
		projectJourneyWrite(t, filepath.Join(receipt.Locator, "keep.txt"), "retained environment data\n")
	}
}

func projectJourneyRepairAndRemove(t *testing.T, journey projectJourney) {
	t.Helper()
	userNative, bNative := securityFixtureTree(t, journey.nativeRoot("")), securityFixtureTree(t, journey.nativeRoot(journey.b))
	userState := projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json"))
	bState := projectJourneyRead(t, filepath.Join(journey.namespace(journey.b), "state-v2.json"))
	statePath := filepath.Join(journey.namespace(journey.a), "state-v2.json")
	before := projectJourneyRead(t, statePath)
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	if before != projectJourneyRead(t, statePath) {
		t.Fatal("unchanged locked reconciliation rewrote project receipts")
	}
	installation, binding := journey.owned(t, journey.a)
	managedSkill := filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")
	projectJourneyWrite(t, managedSkill, "drifted managed package\n")
	projectJourneyWrite(t, filepath.Join(journey.transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	projectJourneyGit(t, journey.transport.repo, "add", ".")
	projectJourneyGit(t, journey.transport.repo, "commit", "-m", "advance disposable project source")
	projectJourneyGit(t, journey.transport.repo, "push", "--quiet", journey.transport.remote, "main")
	journey.transport.mu.Lock()
	journey.transport.archive = projectJourneyArchive(t, journey.transport.repo)
	journey.transport.mu.Unlock()
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "gemini")
	if projectJourneyRead(t, managedSkill) != projectJourneySkill {
		t.Fatal("repair followed moving source instead of installed pin")
	}
	journey.assertOwnedContext(t, journey.a)
	projectJourneyRecoverInterruptedJournal(t, journey)
	projectJourneyUpdateSelectedProject(t, journey, installation, binding)
	journey.run(t, journey.a, "remove", "docs", "--target", "gemini")
	projectJourneyAssertRemoval(t, journey, installation, binding)
	journey.assertOtherScopeUnchanged(t, "", userNative, userState)
	journey.assertOtherScopeUnchanged(t, journey.b, bNative, bState)
}

func projectJourneyUpdateSelectedProject(t *testing.T, journey projectJourney, previous domain.Installation, binding domain.ClientBinding) {
	t.Helper()
	journey.run(t, journey.a, "update", "docs", "--target", "gemini")
	updated, selected := journey.owned(t, journey.a)
	if updated.Source.SourceBindingID != previous.Source.SourceBindingID || updated.Source.TreeDigest == previous.Source.TreeDigest || updated.Source.ResolvedRevision == previous.Source.ResolvedRevision || selected.ClientBindingID != binding.ClientBindingID || selected.TargetLocator != binding.TargetLocator {
		t.Fatal("project update did not advance within its existing source/binding authority")
	}
	if selected.Activation != domain.ActivationPrepared || selected.Verification != domain.VerificationInstalled || selected.NativeProfileRoot != journey.nativeRoot(journey.a) {
		t.Fatalf("project update lost conservative scoped lifecycle: %+v", selected)
	}
	for _, root := range []string{selected.TargetLocator, journey.nativeRoot(journey.a)} {
		if projectJourneyRead(t, filepath.Join(root, "skills", "docs", "SKILL.md")) != projectJourneyAdvancedSkill {
			t.Fatal("selected project did not receive updated pinned bytes")
		}
	}
	for key, receipt := range previous.DataReceipts {
		if updated.DataReceipts[key].Locator != receipt.Locator || projectJourneyRead(t, filepath.Join(receipt.Locator, "keep.txt")) != "retained environment data\n" {
			t.Fatal("project update replaced persistent data authority")
		}
	}
}

func projectJourneyAssertRemoval(t *testing.T, journey projectJourney, installation domain.Installation, binding domain.ClientBinding) {
	t.Helper()
	projectJourneyAbsent(t, binding.TargetLocator, filepath.Join(journey.nativeRoot(journey.a), "skills", "docs"))
	settings := projectJourneyRead(t, filepath.Join(journey.nativeRoot(journey.a), "settings.json"))
	if strings.Contains(settings, `"docs"`) || !strings.Contains(settings, "foreign.test") || !strings.Contains(settings, "retained foreign settings") {
		t.Fatalf("exact removal changed foreign MCP configuration: %s", settings)
	}
	for _, receipt := range installation.DataReceipts {
		if projectJourneyRead(t, filepath.Join(receipt.Locator, "keep.txt")) != "retained environment data\n" {
			t.Fatal("removal consumed retained project data")
		}
	}
	state := journey.state(t, journey.a)
	if len(state.EnvironmentReceipts) != 0 {
		t.Fatal("removed project retained an environment ownership receipt")
	}
	if projectJourneyRead(t, filepath.Join(journey.nativeRoot(journey.a), "skills", "foreign", "SKILL.md")) != "foreign skill\n" {
		t.Fatal("foreign native skill changed")
	}
}
