package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Public saved CLI, real TLS Git and the production source/store/planner,
// installer/native transactions. Detection/security are inert host boundaries.
// The trusted test executable is copied as launcher bytes but never executed;
// this test certifies no Copilot, MCP, model, trust or native discovery behavior.
func TestProjectCopilotComposedSavedLifecycle(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".copilot")
	journey.app = configureProjectCopilotJourney(journey.app, user)
	foreign := `{"mcpServers":{"foreign":{"type":"http","url":"https://foreign.test/mcp","enabled":false}},"policy":"keep"}`
	for _, project := range []string{user, journey.a, journey.b} {
		projectJourneyWrite(t, filepath.Join(project, ".github", "mcp.json"), foreign)
		projectJourneyWrite(t, filepath.Join(project, ".github", "skills", "foreign", "SKILL.md"), "foreign Copilot skill\n")
	}
	projectJourneyWrite(t, filepath.Join(journey.a, ".mcp.json"), foreign)
	userBefore := securityFixtureTree(t, user)
	for _, project := range []string{journey.a, journey.b} {
		journey.run(t, project, "add", transport.endpoint, "--alias", "docs", "--target", "copilot")
		projectCopilotOwned(t, journey, project, projectJourneySkill)
	}
	projectJourneyWrite(t, filepath.Join(journey.b, "B-retained-marker"), "other project\n")
	bBefore := securityFixtureTree(t, journey.b)
	statePath := filepath.Join(journey.namespace(journey.a), "state-v2.json")
	stateBefore := projectJourneyRead(t, statePath)
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	if stateBefore != projectJourneyRead(t, statePath) {
		t.Fatal("unchanged Copilot reconciliation rewrote ownership")
	}
	installation, binding := projectCopilotOwned(t, journey, journey.a, projectJourneySkill)
	projectJourneyDataMarker(t, journey, journey.a, installation)
	projectJourneyWrite(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"), "TEST immutable drift\n")
	if err := os.RemoveAll(filepath.Join(journey.a, ".github", "skills", "docs")); err != nil {
		t.Fatal(err)
	}
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "advance disposable Copilot source")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "copilot")
	projectCopilotOwned(t, journey, journey.a, projectJourneySkill)
	projectCopilotRecover(t, journey)
	projectCopilotUpdateRemove(t, journey, installation, binding)
	if !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) {
		t.Fatal("Copilot A lifecycle mutated the user profile or B")
	}
}

func projectCopilotUpdateRemove(t *testing.T, journey projectJourney, installation domain.Installation, binding domain.ClientBinding) {
	t.Helper()
	journey.run(t, journey.a, "update", "docs", "--target", "copilot")
	updated, current := projectCopilotOwned(t, journey, journey.a, projectJourneyAdvancedSkill)
	if updated.Source.TreeDigest == installation.Source.TreeDigest || current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator || updated.Source.SourceBindingID != installation.Source.SourceBindingID {
		t.Fatal("Copilot update lost exact source/binding authority or did not advance")
	}
	status := journey.run(t, journey.a, "deps", "status")
	for _, expected := range []string{`"copilot"`, `"project"`, `"activation_prepared"`} {
		if !strings.Contains(status, expected) {
			t.Fatalf("Copilot status omitted %s: %s", expected, status)
		}
	}
	journey.run(t, journey.a, "remove", "docs", "--target", "copilot")
	projectJourneyAbsent(t, binding.TargetLocator, filepath.Join(journey.a, ".github", "skills", "docs"))
	config := projectJourneyRead(t, filepath.Join(journey.a, ".mcp.json"))
	if strings.Contains(config, `"docs"`) || !strings.Contains(config, "foreign.test") || !strings.Contains(config, `"enabled":false`) {
		t.Fatalf("Copilot exact removal changed foreign settings: %s", config)
	}
	if projectJourneyRead(t, filepath.Join(journey.a, ".github", "skills", "foreign", "SKILL.md")) != "foreign Copilot skill\n" {
		t.Fatal("Copilot removal changed an unrelated project skill")
	}
	for key, receipt := range installation.DataReceipts {
		if updated.DataReceipts[key].Locator != receipt.Locator || projectJourneyRead(t, filepath.Join(receipt.Locator, "keep.txt")) != "retained environment data\n" {
			t.Fatal("Copilot update/remove changed retained data authority")
		}
	}
	if len(journey.state(t, journey.a).EnvironmentReceipts) != 0 {
		t.Fatal("Copilot remove retained saved environment ownership")
	}
	after := securityFixtureTree(t, journey.a)
	_, _ = journey.execute(journey.a, "remove", "docs", "--target", "copilot")
	if !reflect.DeepEqual(after, securityFixtureTree(t, journey.a)) {
		t.Fatal("repeat absent Copilot removal changed bytes")
	}
}

func projectCopilotRecover(t *testing.T, journey projectJourney) {
	t.Helper()
	github := securityFixtureTree(t, filepath.Join(journey.a, ".github"))
	config := projectJourneyRead(t, filepath.Join(journey.a, ".mcp.json"))
	staging, active := projectJourneyInterruptDirectoryJournal(t, journey)
	for range 2 {
		journey.run(t, journey.a, "deps", "recover", "--yes")
	}
	journey.inspectNoRecovery(t, journey.a)
	projectJourneyAbsent(t, active)
	if !reflect.DeepEqual(github, securityFixtureTree(t, filepath.Join(journey.a, ".github"))) || config != projectJourneyRead(t, filepath.Join(journey.a, ".mcp.json")) || projectJourneyRead(t, filepath.Join(staging, "unpublished.txt")) != "caller-owned unpublished bytes\n" {
		t.Fatal("Copilot bounded pending-journal recovery changed native/caller bytes")
	}
}
