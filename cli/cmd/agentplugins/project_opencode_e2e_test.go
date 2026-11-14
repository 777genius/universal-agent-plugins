package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Public saved CLI and production composition, filesystem/native transactions,
// source acquisition and immutable store. Only the detected native host version
// comes from an inert TEST process; no OpenCode/MCP/model runtime is certified.
func TestProjectOpenCodeComposedSavedLifecycle(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, "opencode-user")
	journey.app = configureProjectOpenCodeJourney(journey.app, projectOpenCodeDetector{root: user, executable: projectOpenCodeProbeFixture(t)})
	foreign := "{ // retained OpenCode settings\n\"mcp\":{\"foreign\":{\"type\":\"remote\",\"url\":\"https://foreign.test/mcp\",\"enabled\":false}}}\n"
	for _, root := range []string{user, journey.a, journey.b} {
		projectJourneyWrite(t, filepath.Join(root, "opencode.jsonc"), foreign)
		projectJourneyWrite(t, filepath.Join(root, ".opencode", "skills", "foreign", "SKILL.md"), "foreign OpenCode skill\n")
	}
	userBefore := securityFixtureTree(t, user)
	for _, project := range []string{journey.a, journey.b} {
		journey.run(t, project, "add", transport.endpoint, "--alias", "docs", "--target", "opencode")
		projectOpenCodeOwned(t, journey, project, projectJourneySkill)
	}
	bBefore := securityFixtureTree(t, journey.b)
	statePath := filepath.Join(journey.namespace(journey.a), "state-v2.json")
	stateBefore := projectJourneyRead(t, statePath)
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	if stateBefore != projectJourneyRead(t, statePath) {
		t.Fatal("unchanged OpenCode sync rewrote saved ownership")
	}
	installation, binding := projectOpenCodeOwned(t, journey, journey.a, projectJourneySkill)
	projectJourneyDataMarker(t, journey, journey.a, installation)
	projectJourneyWrite(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"), "drifted immutable bytes\n")
	if err := os.RemoveAll(filepath.Dir(projectOpenCodeNativeSkill(journey.a))); err != nil {
		t.Fatal(err)
	}
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "advance disposable OpenCode source")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "opencode")
	projectOpenCodeOwned(t, journey, journey.a, projectJourneySkill)
	projectOpenCodeRecover(t, journey)
	projectOpenCodeUpdateAndRemove(t, journey, installation, binding)
	if len(journey.state(t, journey.a).EnvironmentReceipts) != 0 || !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) {
		t.Fatal("OpenCode lifecycle retained removed ownership or changed user/project B")
	}
}

func projectOpenCodeUpdateAndRemove(t *testing.T, journey projectJourney, installation domain.Installation, binding domain.ClientBinding) {
	t.Helper()
	journey.run(t, journey.a, "update", "docs", "--target", "opencode")
	updated, selected := projectOpenCodeOwned(t, journey, journey.a, projectJourneyAdvancedSkill)
	if updated.Source.TreeDigest == installation.Source.TreeDigest || updated.Source.SourceBindingID != installation.Source.SourceBindingID || selected.ClientBindingID != binding.ClientBindingID {
		t.Fatal("OpenCode update lost saved source/binding authority or failed to advance")
	}
	status := journey.run(t, journey.a, "deps", "status")
	for _, fact := range []string{`"project"`, `"opencode"`, `"activation_prepared"`} {
		if !strings.Contains(status, fact) {
			t.Fatalf("saved OpenCode status lost %s: %s", fact, status)
		}
	}
	journey.run(t, journey.a, "remove", "docs", "--target", "opencode")
	projectJourneyAbsent(t, binding.TargetLocator, filepath.Dir(projectOpenCodeNativeSkill(journey.a)))
	config := projectJourneyRead(t, filepath.Join(journey.a, "opencode.jsonc"))
	if strings.Contains(config, `"docs"`) || !strings.Contains(config, "foreign.test") || !strings.Contains(config, "retained OpenCode settings") {
		t.Fatalf("OpenCode remove changed foreign native configuration: %s", config)
	}
	if projectJourneyRead(t, filepath.Join(journey.a, ".opencode", "skills", "foreign", "SKILL.md")) != "foreign OpenCode skill\n" {
		t.Fatal("OpenCode remove changed foreign project skill")
	}
	for key, receipt := range installation.DataReceipts {
		if updated.DataReceipts[key].Locator != receipt.Locator || projectJourneyRead(t, filepath.Join(receipt.Locator, "keep.txt")) != "retained environment data\n" {
			t.Fatal("OpenCode update/remove lost retained project data authority")
		}
	}
	after := securityFixtureTree(t, journey.a)
	_, _ = journey.execute(journey.a, "remove", "docs", "--target", "opencode")
	if !reflect.DeepEqual(after, securityFixtureTree(t, journey.a)) {
		t.Fatal("repeat OpenCode removal changed an absent managed target")
	}
}

func projectOpenCodeRecover(t *testing.T, journey projectJourney) {
	t.Helper()
	before := securityFixtureTree(t, filepath.Join(journey.a, ".opencode"))
	config := projectJourneyRead(t, filepath.Join(journey.a, "opencode.jsonc"))
	staging, active := projectJourneyInterruptDirectoryJournal(t, journey)
	journey.run(t, journey.a, "deps", "recover", "--yes")
	journey.inspectNoRecovery(t, journey.a)
	projectJourneyAbsent(t, active)
	if projectJourneyRead(t, filepath.Join(staging, "unpublished.txt")) != "caller-owned unpublished bytes\n" || !reflect.DeepEqual(before, securityFixtureTree(t, filepath.Join(journey.a, ".opencode"))) || config != projectJourneyRead(t, filepath.Join(journey.a, "opencode.jsonc")) {
		t.Fatal("OpenCode project recovery changed native configuration or caller staging")
	}
	journey.run(t, journey.a, "deps", "recover", "--yes")
}
