//go:build linux

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type grokRepairVersionDetector struct {
	clientdetect.Detector
	t         *testing.T
	probes    int
	requested [][]domain.ClientID
	alter     string
	known     string
}

func (detector *grokRepairVersionDetector) Detect(ctx context.Context) ([]domain.DetectedClient, error) {
	observed, err := detector.Detector.Detect(ctx)
	for index := range observed {
		if observed[index].ClientID == domain.ClientGrok {
			observed[index].Version = detector.known
		}
	}
	return observed, err
}

func (detector *grokRepairVersionDetector) DetectTargetsWithVersionProbe(ctx context.Context, targets []domain.ClientID) ([]domain.DetectedClient, error) {
	detector.requested = append(detector.requested, append([]domain.ClientID(nil), targets...))
	if !reflect.DeepEqual(targets, []domain.ClientID{domain.ClientGrok}) {
		detector.t.Fatal("version probe included an unselected client", targets)
	}
	observed, err := detector.Detector.DetectTargetsWithVersionProbe(ctx, targets)
	if err != nil {
		return nil, err
	}
	for index := range observed {
		if observed[index].ClientID != domain.ClientGrok {
			continue
		}
		switch detector.alter {
		case "executable":
			observed[index].ExecutablePath += "-replacement"
		case "profile":
			observed[index].ConfigRoot += "-replacement"
		case "duplicate":
			observed = append(observed, observed[index])
		case "missing":
			observed[index].ClientID = domain.ClientClaude
		case "failed":
			observed[index].DetectionError = fmt.Errorf("inert version failure")
		}
		break
	}
	return observed, nil
}

type grokRepairVersionRunner struct{ *projectGrokEnableRunner }

func (runner *grokRepairVersionRunner) RunReadonlyDuplexWithPlannedShutdown(ctx context.Context, command legacyports.Command, _ legacyports.ReadonlyDuplexInputs, exchange func(io.Writer, io.Reader) error) error {
	return runner.RunDuplexWithPlannedShutdown(ctx, command, exchange)
}

func configureGrokRepairVersion(app agentpluginscli.App, detector *grokRepairVersionDetector, runner *grokRepairVersionRunner) agentpluginscli.App {
	helper, err := os.Executable()
	if err != nil {
		runner.t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{UserHome: runner.home, ProjectRoot: app.ProjectRoot, StateRoot: filepath.Dir(app.ManagedRoot), ManagedRoot: app.ManagedRoot,
		OperationsDir: app.Lifecycle.Kernel.Directory.JournalDir, Registry: app.ClientRegistry, Runner: runner, EnableNativeObserver: true, HelperExecutable: helper})
	if err != nil {
		runner.t.Fatal(err)
	}
	app.Installer = engine
	app.Detector = detector
	activator := app.Lifecycle.Activator.(providers.Activator)
	activator.Runner = runner
	app.Lifecycle.Activator = activator
	observer := app.Lifecycle.NativeObserver.(providers.NativeIdentityObserver)
	observer.Runner = runner
	app.Lifecycle.NativeObserver = observer
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureGrokRepairVersion(selected, detector, runner), nil
	}
	return app
}

func newGrokRepairVersionJourney(t *testing.T, noSave ...bool) (projectJourney, *grokRepairVersionDetector, *grokRepairVersionRunner) {
	t.Helper()
	transport := newProjectJourneyTransport(t, "zip")
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://inert.test/mcp"}}}`)
	transport.mu.Lock()
	transport.archive = projectJourneyArchive(t, transport.repo)
	transport.mu.Unlock()
	journey := newProjectJourney(t, transport)
	projectJourneyGit(t, journey.a, "init", "--initial-branch=main")
	config := filepath.Join(journey.home, ".grok", "config.toml")
	projectJourneyWrite(t, config, "# retained foreign settings\n[plugins]\nenabled=['foreign']\ndisabled=['other']\n")
	projectJourneyWrite(t, filepath.Join(journey.home, ".grok", ".config-init.lock"), "")
	executable := filepath.Join(journey.home, "grok-version-fixture")
	projectJourneyWrite(t, executable, "#!/bin/sh\n[ \"$#\" -eq 1 ] && [ \"$1\" = --version ] || exit 73\nprintf 'grok 1.0.70 (f50904eaee6c)\\n'\n")
	if err := os.Chmod(executable, 0o700); err != nil {
		t.Fatal(err)
	}
	detector := &grokRepairVersionDetector{Detector: clientdetect.NewOS(journey.home), t: t}
	detector.Registry = journey.app.ClientRegistry
	detector.Environment = map[string]string{"HOME": journey.home}
	detector.LookPath = func(name string) (string, error) {
		if name == "grok" || name == "claude" {
			return executable, nil
		}
		return "", os.ErrNotExist
	}
	probe := detector.ProbeVersionWithEnvironment
	detector.ProbeVersionWithEnvironment = func(ctx context.Context, selected string, env []string) (string, error) {
		if selected != executable {
			t.Fatal("version probe left selected executable", selected)
		}
		detector.probes++
		return probe(ctx, selected, env)
	}
	runner := &grokRepairVersionRunner{&projectGrokEnableRunner{projectGrokRunner: &projectGrokRunner{t: t, executable: executable, version: "1.0.70"}, home: journey.home}}
	journey.app = configureGrokRepairVersion(journey.app, detector, runner)
	args := []string{"add", transport.endpoint, "--alias", "docs", "--target", "grok"}
	if len(noSave) > 0 && noSave[0] {
		args = []string{"add", transport.endpoint, "--target", "grok", "--no-save"}
	}
	journey.run(t, journey.a, args...)
	return journey, detector, runner
}

func declineGrokVersionRemoval(t *testing.T, journey projectJourney) {
	t.Helper()
	var output bytes.Buffer
	app := journey.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader("n\n"), &output, &output, true
	command := agentpluginscli.NewRoot(app)
	command.SetArgs([]string{"remove", "docs", "--scope", "project", "--project-root", journey.a, "--format", "human"})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err, output.String())
	}
}

func TestProjectGrokConfirmedRemovalMeasuresSelectedVersion(t *testing.T) {
	journey, detector, runner := newGrokRepairVersionJourney(t)
	journey.run(t, journey.a, "repair", "docs", "--target", "grok", "--confirm-plugin-enable", "--yes")
	config := filepath.Join(journey.home, ".grok", "config.toml")
	state := filepath.Join(journey.namespace(journey.a), "state-v2.json")
	beforeConfig, beforeState := projectJourneyRead(t, config), projectJourneyRead(t, state)
	beforeProbes, beforeACP := detector.probes, runner.acp
	journey.run(t, journey.a, "remove", "docs", "--target", "grok", "--dry-run", "--yes")
	declineGrokVersionRemoval(t, journey)
	if detector.probes != beforeProbes || runner.acp != beforeACP || projectJourneyRead(t, config) != beforeConfig || projectJourneyRead(t, state) != beforeState {
		t.Fatal("unconfirmed removal probed or changed the owned binding")
	}
	journey.run(t, journey.a, "remove", "docs", "--target", "grok", "--yes")
	if detector.probes != beforeProbes+1 || runner.acp != beforeACP+2 || strings.Contains(projectJourneyRead(t, config), "project/") {
		t.Fatal("confirmed removal did not measure its exact selected version and remove only the owned member")
	}
	for _, installation := range journey.state(t, journey.a).Installations {
		if len(installation.Clients) != 0 {
			t.Fatal("confirmed removal retained a client binding")
		}
	}
}

func TestProjectGrokDirectRepairAndRemovalMeasureSelectedVersion(t *testing.T) {
	journey, detector, runner := newGrokRepairVersionJourney(t, true)
	journey.run(t, journey.a, "repair", "project-demo", "--target", "grok", "--yes")
	if detector.probes != 0 || runner.acp != 0 {
		t.Fatal("ordinary direct repair executed version or ACP")
	}
	config := filepath.Join(journey.home, ".grok", "config.toml")
	state := filepath.Join(journey.namespace(journey.a), "state-v2.json")
	beforeConfig, beforeState := projectJourneyRead(t, config), projectJourneyRead(t, state)
	journey.run(t, journey.a, "repair", "project-demo", "--target", "grok", "--confirm-plugin-enable", "--dry-run", "--yes")
	if detector.probes != 1 || runner.acp != 0 || projectJourneyRead(t, config) != beforeConfig || projectJourneyRead(t, state) != beforeState {
		t.Fatal("confirmed direct preview did not preserve the original prepared binding")
	}
	journey.run(t, journey.a, "repair", "project-demo", "--target", "grok", "--confirm-plugin-enable", "--yes")
	if detector.probes != 2 || runner.acp != 2 {
		t.Fatal("direct confirmed repair did not use one selected version and two readbacks")
	}
	beforeConfig, beforeState = projectJourneyRead(t, config), projectJourneyRead(t, state)
	journey.run(t, journey.a, "remove", "project-demo", "--target", "grok", "--dry-run", "--yes")
	if detector.probes != 2 || runner.acp != 2 || projectJourneyRead(t, config) != beforeConfig || projectJourneyRead(t, state) != beforeState {
		t.Fatal("direct removal preview probed or changed originals")
	}
	journey.run(t, journey.a, "remove", "project-demo", "--target", "grok", "--yes")
	if detector.probes != 3 || runner.acp != 4 || strings.Contains(projectJourneyRead(t, config), "project/") {
		t.Fatal("direct owned removal did not measure and remove its exact native membership")
	}
	for _, installation := range journey.state(t, journey.a).Installations {
		if len(installation.Clients) != 0 {
			t.Fatal("direct removal retained a client binding")
		}
	}
}

func TestProjectGrokPreparedRemovalDoesNotProbeVersion(t *testing.T) {
	for _, route := range []string{"saved", "direct"} {
		t.Run(route, func(t *testing.T) {
			journey, detector, runner := newGrokRepairVersionJourney(t, route == "direct")
			journey.run(t, journey.a, "remove", "project-demo", "--target", "grok", "--yes")
			if detector.probes != 0 || len(detector.requested) != 0 || runner.acp != 0 {
				t.Fatal("Prepared removal executed version or ACP without owned native membership")
			}
		})
	}
}

func TestProjectGrokConfirmedRepairMeasuresSelectedVersion(t *testing.T) {
	journey, detector, runner := newGrokRepairVersionJourney(t)
	journey.run(t, journey.a, "repair", "docs", "--target", "grok", "--yes")
	if detector.probes != 0 || len(detector.requested) != 0 || runner.acp != 0 {
		t.Fatal("ordinary repair executed version or ACP")
	}
	config := filepath.Join(journey.home, ".grok", "config.toml")
	state := filepath.Join(journey.namespace(journey.a), "state-v2.json")
	beforeConfig, beforeState := projectJourneyRead(t, config), projectJourneyRead(t, state)
	journey.run(t, journey.a, "repair", "docs", "--target", "grok", "--confirm-plugin-enable", "--dry-run", "--yes")
	if detector.probes != 1 || len(detector.requested) != 1 || runner.acp != 0 || projectJourneyRead(t, config) != beforeConfig || projectJourneyRead(t, state) != beforeState {
		t.Fatal("confirmed preview did not measure only the selected version without mutation")
	}
	output := journey.run(t, journey.a, "repair", "docs", "--target", "grok", "--confirm-plugin-enable", "--yes")
	if detector.probes != 2 || len(detector.requested) != 2 || runner.acp != 2 || !strings.Contains(output, "selected Grok project plugin is enabled") {
		t.Fatalf("confirmed repair did not use measured version and both native readbacks: probes=%d ACP=%d\n%s", detector.probes, runner.acp, output)
	}
	_, binding := journey.owned(t, journey.a)
	owned := 0
	for _, object := range binding.NativeObjects {
		if object.Kind == "grok_enabled_plugin" {
			owned++
		}
	}
	if owned != 1 || binding.NativeActivationAttempt != "" || binding.Activation != domain.ActivationPrepared {
		t.Fatal("confirmed repair lacks its settled exact native membership")
	}
}

func TestProjectGrokRepairVersionRetainsSelectionAndConsent(t *testing.T) {
	journey, detector, runner := newGrokRepairVersionJourney(t)
	config := filepath.Join(journey.home, ".grok", "config.toml")
	state := filepath.Join(journey.namespace(journey.a), "state-v2.json")
	beforeConfig, beforeState := projectJourneyRead(t, config), projectJourneyRead(t, state)
	for _, condition := range []string{"implicit-target", "known-wrong", "executable", "profile", "duplicate", "missing", "failed"} {
		t.Run(condition, func(t *testing.T) {
			detector.alter, detector.known = condition, ""
			args := []string{"repair", "docs", "--target", "grok", "--confirm-plugin-enable", "--dry-run", "--yes"}
			wantProbes := 1
			switch condition {
			case "implicit-target":
				args = []string{"repair", "docs", "--confirm-plugin-enable", "--dry-run", "--yes"}
				wantProbes = 0
			case "known-wrong":
				detector.known, wantProbes = "1.0.67", 0
			}
			beforeProbes, beforeRequests, beforeCalls := detector.probes, len(detector.requested), runner.calls
			if output, err := journey.execute(journey.a, args...); err == nil {
				t.Fatal("invalid confirmed version accepted", output)
			}
			if detector.probes-beforeProbes != wantProbes || len(detector.requested)-beforeRequests != wantProbes || runner.calls != beforeCalls || runner.acp != 0 || projectJourneyRead(t, config) != beforeConfig || projectJourneyRead(t, state) != beforeState {
				t.Fatal("refused selected version changed originals or executed native activation")
			}
		})
	}
}

func TestProjectGrokRepairRejectsAmbiguousBindingBeforeVersion(t *testing.T) {
	journey, detector, runner := newGrokRepairVersionJourney(t, true)
	app, err := journey.app.ForScope(domain.ScopeProject, journey.a)
	if err != nil {
		t.Fatal(err)
	}
	state, err := app.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	installation := &state.Installations[0]
	for _, binding := range installation.Clients {
		binding.TargetLocator += "-second"
		binding.ClientBindingID = domain.ComputeClientBindingID(installation.InstallationID, binding.ClientID, binding.Scope, binding.TargetLocator)
		binding.Receipts = nil
		installation.Clients[binding.ClientBindingID] = binding
		break
	}
	if err := app.StateStore.Save(state); err != nil {
		t.Fatal("ambiguous binding fixture was not valid persisted state", err)
	}
	config := filepath.Join(journey.home, ".grok", "config.toml")
	statePath := filepath.Join(journey.namespace(journey.a), "state-v2.json")
	beforeConfig, beforeState, beforeCalls := projectJourneyRead(t, config), projectJourneyRead(t, statePath), runner.calls
	output, err := journey.execute(journey.a, "repair", "project-demo", "--target", "grok", "--confirm-plugin-enable", "--dry-run", "--yes")
	if err == nil || !strings.Contains(err.Error(), "repair requires exactly one bound target") {
		t.Fatal("ambiguous binding did not retain its original refusal", err, output)
	}
	if detector.probes != 0 || len(detector.requested) != 0 || runner.calls != beforeCalls || runner.acp != 0 || projectJourneyRead(t, config) != beforeConfig || projectJourneyRead(t, statePath) != beforeState {
		t.Fatalf("ambiguous binding executed or changed originals: version probes=%d", detector.probes)
	}
}
