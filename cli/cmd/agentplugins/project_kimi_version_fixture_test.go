package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This is an inert detector. A plain discovery never knows the client version;
// only the exact selected probe supplies it. No client executable is invoked.
type projectKimiVersionDetector struct {
	user, executable string
	targeted         [][]domain.ClientID
	broad            int
	probeError       error
	geminiError      error
}

func (d *projectKimiVersionDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{
		{ClientID: domain.ClientKimi, ConfigRoot: d.user, ExecutablePath: d.executable, Status: domain.DetectionDetected},
		{ClientID: domain.ClientGemini, ConfigRoot: filepath.Join(filepath.Dir(d.user), ".gemini"), Status: domain.DetectionDetected, DetectionError: d.geminiError},
	}, nil
}

func (d *projectKimiVersionDetector) DetectWithVersionProbe(ctx context.Context) ([]domain.DetectedClient, error) {
	d.broad++
	rows, err := d.Detect(ctx)
	if err != nil {
		return rows, err
	}
	return rows, fmt.Errorf("fixture refuses broad version execution")
}

func (d *projectKimiVersionDetector) DetectTargetsWithVersionProbe(ctx context.Context, targets []domain.ClientID) ([]domain.DetectedClient, error) {
	d.targeted = append(d.targeted, append([]domain.ClientID(nil), targets...))
	if d.probeError != nil {
		return nil, d.probeError
	}
	clients, err := d.Detect(ctx)
	if err != nil {
		return nil, err
	}
	for i := range clients {
		for _, target := range targets {
			if clients[i].ClientID == target && target == domain.ClientKimi {
				clients[i].Version = "2.3.1"
			}
		}
	}
	return clients, nil
}

func (d *projectKimiVersionDetector) assertCalls(t *testing.T, before, added int) {
	t.Helper()
	if d.broad != 0 || len(d.targeted) != before+added {
		t.Fatalf("version execution differs: broad=%d targeted=%v; wanted %d new selected calls", d.broad, d.targeted, added)
	}
	for _, targets := range d.targeted {
		if !reflect.DeepEqual(targets, []domain.ClientID{domain.ClientKimi}) {
			t.Fatalf("unselected client reached version execution: %v", targets)
		}
	}
}

func configureProjectKimiVersionJourney(app agentpluginscli.App, detector *projectKimiVersionDetector) agentpluginscli.App {
	app.Detector = detector
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectKimiVersionJourney(selected, detector), nil
	}
	return app
}

func newProjectKimiVersionJourney(t *testing.T) (projectJourney, *projectKimiVersionDetector) {
	t.Helper()
	journey := newProjectJourney(t, newProjectJourneyTransport(t, "git"))
	detector := &projectKimiVersionDetector{user: filepath.Join(journey.home, ".kimi-code"), executable: filepath.Join(journey.home, "kimi")}
	journey.app = configureProjectKimiVersionJourney(journey.app, detector)
	for _, project := range []string{journey.a, journey.b} {
		if err := os.Mkdir(filepath.Join(project, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, root := range []string{detector.user, filepath.Join(journey.a, ".kimi-code"), filepath.Join(journey.b, ".kimi-code")} {
		projectJourneyWrite(t, filepath.Join(root, "mcp.json"), `{"mcpServers":{"foreign":{"transport":"http","url":"https://foreign.test/mcp","enabled":false}},"policy":"keep"}`)
		projectJourneyWrite(t, filepath.Join(root, "skills", "foreign", "SKILL.md"), "foreign Kimi skill\n")
	}
	return journey, detector
}

func projectKimiVersionOwned(t *testing.T, journey projectJourney, expected string, saved bool) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	state := journey.state(t, journey.a)
	receipts := 0
	if saved {
		receipts = 1
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 || len(state.EnvironmentReceipts) != receipts {
		t.Fatalf("Kimi lifecycle lost exact local/saved ownership: %+v", state)
	}
	installation := state.Installations[0]
	for _, binding := range installation.Clients {
		if binding.ClientID != "kimi" || binding.Scope != "project" || binding.NativeProfileRoot != filepath.Join(journey.a, ".kimi-code") || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
			t.Fatalf("version discovery changed Kimi project authority: %+v", binding)
		}
		for _, root := range []string{binding.TargetLocator, binding.NativeProfileRoot} {
			if projectJourneyRead(t, filepath.Join(root, "skills", "docs", "SKILL.md")) != expected {
				t.Fatal("Kimi selected managed/native bytes differ")
			}
		}
		return installation, binding
	}
	t.Fatal("Kimi binding missing")
	return domain.Installation{}, domain.ClientBinding{}
}
