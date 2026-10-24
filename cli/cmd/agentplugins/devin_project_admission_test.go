package main

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectDevinNoSaveUsesSelectedHostAndAbsentLegacyProfile(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	devinProjectCommitSource(t, transport, projectJourneySkill, "https://managed.test/v1")
	f := newDevinProjectJourney(t, transport)
	user, b := securityFixtureTree(t, f.devinRoot("")), securityFixtureTree(t, f.devinRoot(f.b))
	f.run(t, f.a, "add", transport.repo, "--no-save", "--target", "devin")
	state := f.state(t, f.a)
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("no-save changed ownership route: %+v", state)
	}
	installation := state.Installations[0]
	var binding domain.ClientBinding
	for _, owned := range installation.Clients {
		binding = owned
	}
	if binding.ClientID != "windsurf" || binding.NativeProfileRoot != f.devinRoot(f.a) || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("no-save project host evidence missing: %+v", binding)
	}
	if !strings.Contains(projectJourneyRead(t, filepath.Join(f.devinRoot(f.a), "mcp_config.json")), "managed.test/v1") {
		t.Fatal("no-save current Devin dedicated MCP missing")
	}
	f.run(t, f.a, "remove", installation.InstallationID, "--target", "windsurf")
	projectJourneyAbsent(t, binding.TargetLocator, filepath.Join(f.devinRoot(f.a), "skills", "docs"))
	if !reflect.DeepEqual(user, securityFixtureTree(t, f.devinRoot(""))) || !reflect.DeepEqual(b, securityFixtureTree(t, f.devinRoot(f.b))) {
		t.Fatal("no-save changed another native scope")
	}
}

type devinUnqualifiedDetector struct{ client domain.DetectedClient }

func (detector devinUnqualifiedDetector) Detect(_ context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{detector.client}, nil
}

func TestProjectDevinCLIRefusesMissingOlderAndWrongHarnessBeforeNativeEffects(t *testing.T) {
	transport := newProjectJourneyTransport(t, "zip")
	devinProjectCommitSource(t, transport, projectJourneySkill, "https://managed.test/v1")
	for _, version := range []string{"", "3000.3.0", "3000.11.205-alpha", "wrong-harness"} {
		t.Run("version="+version, func(t *testing.T) {
			f := newDevinProjectJourney(t, transport)
			client := domain.DetectedClient{ClientID: domain.ClientWindsurf, Status: domain.DetectionDetected, ExecutablePath: filepath.Join(f.home, "devin"), Version: version}
			if version == "wrong-harness" {
				client.Version = "3000.11.205"
				client.ExecutablePath = filepath.Join(f.home, "windsurf")
			}
			compose := f.app.ForScope
			f.app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
				selected, err := compose(scope, root)
				if err != nil {
					return selected, err
				}
				selected.Detector = devinUnqualifiedDetector{client: client}
				return selected, nil
			}
			before, user, b := securityFixtureTree(t, f.devinRoot(f.a)), securityFixtureTree(t, f.devinRoot("")), securityFixtureTree(t, f.devinRoot(f.b))
			_, err := f.execute(f.a, "add", transport.repo, "--no-save", "--target", "windsurf")
			if err == nil || !strings.Contains(err.Error(), "explicitly selected primary") {
				t.Fatalf("unqualified CLI host accepted: %v", err)
			}
			if !reflect.DeepEqual(before, securityFixtureTree(t, f.devinRoot(f.a))) || !reflect.DeepEqual(user, securityFixtureTree(t, f.devinRoot(""))) || !reflect.DeepEqual(b, securityFixtureTree(t, f.devinRoot(f.b))) {
				t.Fatal("unqualified CLI host changed native scope")
			}
		})
	}
}
