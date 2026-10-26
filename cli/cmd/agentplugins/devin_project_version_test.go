package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Plain discovery has no version and no Cascade profile. Only the explicitly
// selected inert callback can supply the observed current Devin version.
type devinProjectVersionDetector struct {
	executable string
	targeted   [][]domain.ClientID
	broad      int
	probeError error
}

func (d *devinProjectVersionDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientWindsurf, ExecutablePath: d.executable, Status: domain.DetectionDetected}}, nil
}

func (d *devinProjectVersionDetector) DetectWithVersionProbe(ctx context.Context) ([]domain.DetectedClient, error) {
	d.broad++
	rows, err := d.Detect(ctx)
	if err != nil {
		return rows, err
	}
	return rows, fmt.Errorf("fixture refuses broad version execution")
}

func (d *devinProjectVersionDetector) DetectTargetsWithVersionProbe(ctx context.Context, targets []domain.ClientID) ([]domain.DetectedClient, error) {
	d.targeted = append(d.targeted, append([]domain.ClientID(nil), targets...))
	if d.probeError != nil {
		return nil, d.probeError
	}
	clients, err := d.Detect(ctx)
	if err != nil {
		return nil, err
	}
	clients[0].Version = "3000.11.205"
	return clients, nil
}

func (d *devinProjectVersionDetector) assertCalls(t *testing.T, before, added int) {
	t.Helper()
	if d.broad != 0 || len(d.targeted) != before+added {
		t.Fatalf("Devin version execution differs: broad=%d targeted=%v, want %d new selected calls", d.broad, d.targeted, added)
	}
	for _, targets := range d.targeted {
		if !reflect.DeepEqual(targets, []domain.ClientID{domain.ClientWindsurf}) {
			t.Fatalf("unselected client reached Devin version execution: %v", targets)
		}
	}
}

func configureDevinVersionJourney(app agentpluginscli.App, detector *devinProjectVersionDetector) agentpluginscli.App {
	app.Detector = detector
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureDevinVersionJourney(selected, detector), nil
	}
	return app
}

func devinVersionOwned(t *testing.T, f devinProjectJourney, skill string, saved bool) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	state := f.state(t, f.a)
	receipts := 0
	if saved {
		receipts = 1
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 || len(state.EnvironmentReceipts) != receipts {
		t.Fatalf("Devin version lifecycle lost exact local/saved ownership: %+v", state)
	}
	installation := state.Installations[0]
	for _, binding := range installation.Clients {
		if binding.ClientID != "windsurf" || binding.Scope != "project" || binding.NativeProfileRoot != f.devinRoot(f.a) || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
			t.Fatalf("version observation changed Devin project authority: %+v", binding)
		}
		for _, root := range []string{binding.TargetLocator, f.devinRoot(f.a)} {
			if projectJourneyRead(t, filepath.Join(root, "skills", "docs", "SKILL.md")) != skill {
				t.Fatal("Devin managed/native selected bytes differ")
			}
		}
		return installation, binding
	}
	t.Fatal("Devin project binding missing")
	return domain.Installation{}, domain.ClientBinding{}
}

func TestProjectDevinEmptyVersionSelectedLifecycle(t *testing.T) {
	for _, saved := range []bool{false, true} {
		name := "direct-local"
		if saved {
			name = "saved-tls-zip"
		}
		t.Run(name, func(t *testing.T) {
			transport := newProjectJourneyTransport(t, "zip")
			devinProjectCommitSource(t, transport, projectJourneySkill, "https://managed.test/v1")
			f := newDevinProjectJourney(t, transport)
			detector := &devinProjectVersionDetector{executable: filepath.Join(f.home, ".devin", "devin")}
			f.app = configureDevinVersionJourney(f.app, detector)
			userBefore := securityFixtureTree(t, f.devinRoot(""))
			add := []string{"add", transport.repo, "--no-save", "--target", "devin"}
			selector := "docs"
			if saved {
				add = []string{"add", transport.endpoint, "--alias", "docs", "--target", "windsurf"}
			}
			for _, root := range []string{f.a, f.b} {
				before := len(detector.targeted)
				f.run(t, root, add...)
				detector.assertCalls(t, before, 1)
			}
			installation, binding := devinVersionOwned(t, f, projectJourneySkill, saved)
			if !saved {
				selector = installation.InstallationID
			}
			// This source uses remote MCP configuration and allocates no plugin
			// data receipt. Preserve an existing unrelated data member instead.
			dataPath := filepath.Join(f.namespace(f.a), "plugin-data", "foreign", "keep.txt")
			projectJourneyWrite(t, dataPath, "retained unrelated data\n")
			bBefore := securityFixtureTree(t, f.b)
			preserved := func() {
				t.Helper()
				if !reflect.DeepEqual(userBefore, securityFixtureTree(t, f.devinRoot(""))) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, f.b)) {
					t.Fatal("version-assisted A lifecycle changed original user or project B")
				}
				if projectJourneyRead(t, dataPath) != "retained unrelated data\n" {
					t.Fatal("version-assisted Devin lifecycle changed unrelated persistent data")
				}
			}
			if saved {
				before := len(detector.targeted)
				state := projectJourneyRead(t, filepath.Join(f.namespace(f.a), "state-v2.json"))
				f.run(t, f.a, "deps", "sync", "--locked", "--yes")
				detector.assertCalls(t, before, 0)
				if state != projectJourneyRead(t, filepath.Join(f.namespace(f.a), "state-v2.json")) {
					t.Fatal("unchanged Devin reconciliation changed ownership state")
				}
			}
			for _, args := range [][]string{{"list"}, {"info", installation.InstallationID}, {"doctor"}} {
				before := len(detector.targeted)
				f.run(t, f.a, args...)
				detector.assertCalls(t, before, 0)
			}
			for _, args := range [][]string{{"update", selector, "--target", "windsurf", "--dry-run"}, {"repair", installation.InstallationID, "--target", "windsurf", "--dry-run"}} {
				before, snapshot := len(detector.targeted), securityFixtureTree(t, f.a)
				_, err := f.execute(f.a, args...)
				if err != nil && !strings.Contains(err.Error(), "explicitly selected primary") {
					t.Fatal(err)
				}
				detector.assertCalls(t, before, 0)
				if !reflect.DeepEqual(snapshot, securityFixtureTree(t, f.a)) {
					t.Fatal("Devin dry-run changed project authority")
				}
			}
			before := len(detector.targeted)
			f.run(t, f.a, "repair", installation.InstallationID, "--target", "windsurf")
			detector.assertCalls(t, before, 1)
			devinVersionOwned(t, f, projectJourneySkill, saved)
			preserved()
			devinProjectCommitSource(t, transport, projectJourneyAdvancedSkill, "https://managed.test/v2")
			before = len(detector.targeted)
			f.run(t, f.a, "update", selector, "--target", "windsurf")
			detector.assertCalls(t, before, 1)
			updated, current := devinVersionOwned(t, f, projectJourneyAdvancedSkill, saved)
			if updated.Source.TreeDigest == installation.Source.TreeDigest || current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator {
				t.Fatal("Devin update lost source or selected binding identity")
			}
			if !reflect.DeepEqual(installation.DataReceipts, updated.DataReceipts) {
				t.Fatal("remote MCP update changed the existing data receipt set")
			}
			preserved()
			before = len(detector.targeted)
			f.run(t, f.a, "remove", selector, "--target", "windsurf")
			detector.assertCalls(t, before, 0)
			projectJourneyAbsent(t, binding.TargetLocator, filepath.Join(f.devinRoot(f.a), "skills", "docs"))
			preserved()
		})
	}
}

func TestProjectDevinEmptyVersionFailedObservationPreservesScopes(t *testing.T) {
	transport := newProjectJourneyTransport(t, "zip")
	devinProjectCommitSource(t, transport, projectJourneySkill, "https://managed.test/v1")
	f := newDevinProjectJourney(t, transport)
	detector := &devinProjectVersionDetector{executable: filepath.Join(f.home, ".devin", "devin"), probeError: errors.New("inert selected Devin version failure")}
	f.app = configureDevinVersionJourney(f.app, detector)
	a, b, user := securityFixtureTree(t, f.a), securityFixtureTree(t, f.b), securityFixtureTree(t, f.devinRoot(""))
	if _, err := f.execute(f.a, "add", transport.repo, "--no-save", "--target", "windsurf"); err == nil || !strings.Contains(err.Error(), "inert selected Devin version failure") {
		t.Fatalf("failed selected version was not surfaced: %v", err)
	}
	detector.assertCalls(t, 0, 1)
	if !reflect.DeepEqual(a, securityFixtureTree(t, f.a)) || !reflect.DeepEqual(b, securityFixtureTree(t, f.b)) || !reflect.DeepEqual(user, securityFixtureTree(t, f.devinRoot(""))) {
		t.Fatal("failed selected Devin version changed project or original-user authority")
	}
}
