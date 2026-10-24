package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectKimiImperativeLocalLifecyclePreservesUserAndB(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".kimi-code")
	journey.app = configureProjectKimiJourney(journey.app, user, filepath.Join(journey.home, "kimi"))
	for _, root := range []string{journey.a, journey.b} {
		if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	projectJourneyWrite(t, filepath.Join(user, "mcp.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.test","enabled":false}}}`)
	projectJourneyWrite(t, filepath.Join(journey.b, ".kimi-code", "skills", "foreign", "SKILL.md"), "other project\n")
	beforeUser, beforeB := securityFixtureTree(t, user), securityFixtureTree(t, journey.b)
	journey.run(t, journey.a, "add", transport.repo, "--target", "kimi", "--no-save")
	state := journey.state(t, journey.a)
	if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("imperative route changed saved ownership: %+v", state)
	}
	installation := state.Installations[0]
	for _, binding := range installation.Clients {
		if binding.ClientID != "kimi" || binding.Scope != "project" || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
			t.Fatalf("imperative Kimi overclaimed or lost scope: %+v", binding)
		}
		if projectJourneyRead(t, filepath.Join(journey.a, ".kimi-code", "skills", "docs", "SKILL.md")) != projectJourneySkill {
			t.Fatal("imperative skill projection missing")
		}
	}
	journey.run(t, journey.a, "remove", installation.InstallationID, "--target", "kimi")
	projectJourneyAbsent(t, filepath.Join(journey.a, ".kimi-code", "skills", "docs"))
	if !reflect.DeepEqual(beforeUser, securityFixtureTree(t, user)) || !reflect.DeepEqual(beforeB, securityFixtureTree(t, journey.b)) {
		t.Fatal("imperative project lifecycle changed user or B")
	}
}
