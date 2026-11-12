package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type sessionDetector struct {
	user, executable, version string
	probes                    *int
}

func (d sessionDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientCline, ConfigRoot: d.user,
		ExecutablePath: d.executable, Status: domain.DetectionDetected}}, nil
}
func (d sessionDetector) DetectTargetsWithVersionProbe(ctx context.Context, ids []domain.ClientID) ([]domain.DetectedClient, error) {
	*d.probes++
	found, err := d.Detect(ctx)
	found[0].Version = d.version
	return found, err
}

func configureSessionJourney(t *testing.T, app agentpluginscli.App, detector sessionDetector, starts *[]clients.SessionPlan) agentpluginscli.App {
	t.Helper()
	kernel := nativeconfig.New()
	app.Detector = detector
	app.SessionEnv = clients.Env{NativeConfig: kernel, Runner: sessionInventoryRunner{calls: detector.probes}}
	app.Lifecycle.NativeObserver = providers.NativeIdentityObserver{Registry: app.ClientRegistry,
		Stager: app.Lifecycle.Stager, NativeConfig: &kernel}
	app.StartNativeSession = func(_ context.Context, plan clients.SessionPlan) error {
		*starts = append(*starts, plan)
		return nil
	}
	original := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := original(scope, root)
		if err != nil {
			return selected, err
		}
		return configureSessionJourney(t, selected, detector, starts), nil
	}
	return app
}

// Uses the ordinary composed installer and real source/state/files. The sole
// injected execution boundary captures the planned native start; it is not a
// claim that a vendor CLI or a model ran in this test.
func TestClineProjectSessionComposedLifecycle(t *testing.T) {
	transport := newProjectJourneyTransport(t, "archive")
	journey := newProjectJourney(t, transport)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(journey.home, ".cline")
	projectJourneyWrite(t, filepath.Join(user, "foreign"), "original profile")
	probes := 0
	starts := []clients.SessionPlan{}
	detector := sessionDetector{user: user, executable: executable, version: "3.0.73", probes: &probes}
	journey.app = configureSessionJourney(t, journey.app, detector, &starts)
	journey.run(t, journey.a, "add", transport.endpoint, "--target", "cline", "--alias", "docs")
	installed, binding := journey.owned(t, journey.a)
	userBefore, bBefore := securityFixtureTree(t, user), securityFixtureTree(t, journey.b)
	statePath := filepath.Join(journey.a, ".uap", "state-v2.json")
	before := projectJourneyRead(t, statePath)
	run := []string{"run", "--target", "cline"}
	for _, flags := range [][]string{nil, {"--yes"}} {
		if _, err := journey.execute(journey.a, append(append([]string{}, run...), flags...)...); err == nil {
			t.Fatal("startup accepted missing action-specific consent")
		}
	}
	preview := journey.run(t, journey.a, append(run, "--dry-run")...)
	if !strings.Contains(preview, "requires_client_start_consent") || len(starts) != 0 || probes != 0 {
		t.Fatalf("dry-run/decline started native execution: %s %d %d", preview, len(starts), probes)
	}
	// The fixture defaults to JSON for ordinary commands; native run deliberately
	// inherits terminal I/O instead of mixing it into UAP's JSON envelope.
	args := append(append([]string{}, run...), "--confirm-client-start", "--inspect", "--format", "human")
	if output, err := executeSession(journey, args...); err != nil {
		t.Fatalf("native session: %v\n%s", err, output)
	}
	if len(starts) != 1 || probes != 1 {
		t.Fatalf("startup count: %d probes %d", len(starts), probes)
	}
	p := starts[0]
	settings := filepath.Join(journey.a, ".uap", "clients", "cline", "mcp.json")
	if p.Executable != executable || p.Directory != journey.a ||
		p.Environment["CLINE_MCP_SETTINGS_PATH"] != settings || p.Environment["CLINE_NO_AUTO_UPDATE"] != "1" ||
		!reflect.DeepEqual(p.Arguments, []string{"--auto-approve", "false", "--cwd", journey.a, "config", "mcp", "--json"}) {
		t.Fatalf("native selection differs: %+v", p)
	}
	if before != projectJourneyRead(t, statePath) {
		t.Fatal("run promoted activation or changed installation state")
	}
	original := projectJourneyRead(t, settings)
	var config map[string]any
	if err := json.Unmarshal([]byte(original), &config); err != nil {
		t.Fatal(err)
	}
	config["mcpServers"].(map[string]any)["foreign"] = map[string]any{"command": "foreign"}
	body, _ := json.Marshal(config)
	projectJourneyWrite(t, settings, string(body))
	if _, err := executeSession(journey, args...); err == nil {
		t.Fatal("run adopted an unowned MCP member")
	}
	if len(starts) != 1 || projectJourneyRead(t, settings) != string(body) {
		t.Fatal("refusal started client or rewrote foreign config")
	}
	projectJourneyWrite(t, settings, original)
	if err := os.RemoveAll(binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	if _, err := executeSession(journey, args...); err == nil {
		t.Fatal("missing package was started")
	}
	journey.run(t, journey.a, "repair", installed.InstallationID, "--target", "cline")
	if output, err := executeSession(journey, args...); err != nil {
		t.Fatalf("native session: %v\n%s", err, output)
	}
	if len(starts) != 2 {
		t.Fatal("repaired package did not become selectable")
	}
	journey.run(t, journey.a, "remove", installed.InstallationID, "--target", "cline")
	if _, err := executeSession(journey, args...); err == nil {
		t.Fatal("removed package was started")
	}
	if len(starts) != 2 || !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) ||
		!reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) {
		t.Fatal("session affected original User/Project B")
	}
}

func TestSessionEnvironmentOverridesOnlySelectedKeys(t *testing.T) {
	base := []string{"HOME=/original", "CLINE_MCP_SETTINGS_PATH=/user", "KEEP=value"}
	overrides := map[string]string{"CLINE_MCP_SETTINGS_PATH": "/project", "CLINE_NO_AUTO_UPDATE": "1"}
	got := (clients.SessionPlan{Environment: overrides}).ChildEnvironment(base)
	if !reflect.DeepEqual(got, []string{"HOME=/original", "KEEP=value", "CLINE_MCP_SETTINGS_PATH=/project", "CLINE_NO_AUTO_UPDATE=1"}) {
		t.Fatalf("unexpected child environment: %v", got)
	}
	if base[1] != "CLINE_MCP_SETTINGS_PATH=/user" || len(overrides) != 2 {
		t.Fatal("environment input mutated")
	}
}

func executeSession(journey projectJourney, args ...string) (string, error) {
	var out bytes.Buffer
	app := journey.app
	app.Input, app.Output, app.ErrorOutput = strings.NewReader(""), &out, &out
	root := agentpluginscli.NewRoot(app)
	root.SetArgs(append(append([]string{}, args...), "--scope", "project", "--project-root", journey.a))
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

// This responder verifies selection and returns metadata only; native hosted
// acceptance separately uses the unmodified pinned Cline executable.
type sessionInventoryRunner struct{ calls *int }

func (runner sessionInventoryRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	*runner.calls++
	path := ""
	for _, entry := range command.Env {
		if strings.HasPrefix(entry, "CLINE_MCP_SETTINGS_PATH=") {
			path = strings.TrimPrefix(entry, "CLINE_MCP_SETTINGS_PATH=")
		}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	var document struct {
		Servers map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return legacyports.CommandResult{}, err
	}
	rows := []map[string]any{}
	for name := range document.Servers {
		rows = append(rows, map[string]any{"name": name, "path": path, "transportType": "stdio", "disabled": false})
	}
	output, err := json.Marshal(rows)
	return legacyports.CommandResult{ExitCode: 0, Stdout: output}, err
}
