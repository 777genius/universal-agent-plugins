package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	clientregistry "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestManagementDiscoveryUsesPassiveClientAuthority(t *testing.T) {
	home := t.TempDir()
	home, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	extension := filepath.Join(home, ".cursor", "extensions", "saoudrizwan.claude-dev-1.0.0")
	if err := os.MkdirAll(extension, 0700); err != nil {
		t.Fatal(err)
	}
	registry := clientregistry.Default()
	detector := clientdetect.NewOS(home)
	detector.Registry, detector.Environment = registry, map[string]string{}
	detector.SystemApplicationsDir, detector.LinuxApplicationDirs, detector.WindowsProgramFiles = home, nil, nil
	gemini := filepath.Join(home, "gemini-never-executed")
	if err := os.WriteFile(gemini, []byte("native execution is forbidden during discovery"), 0700); err != nil {
		t.Fatal(err)
	}
	detector.LookPath = func(name string) (string, error) {
		if name == "gemini" {
			return gemini, nil
		}
		return "", exec.ErrNotFound
	}
	detector.ProbeVersionWithEnvironment = func(context.Context, string, []string) (string, error) {
		t.Fatal("passive discovery executed a client")
		return "", nil
	}
	stateRoot := filepath.Join(home, "unused-uap-state")
	engine, err := installer.New(installer.Config{StateRoot: stateRoot, Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	app := App{Version: "fixture", UserHome: home, Detector: detector, ClientRegistry: registry, Installer: engine, Output: &output}
	command := NewRoot(app)
	command.SetArgs([]string{"manage", "discover"})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	var result management.TargetDiscovery
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ApiVersion != 1 {
		t.Fatal("missing versioned discovery", output.String())
	}
	seen := map[string]bool{}
	for _, target := range result.Targets {
		switch target.Client {
		case "cline":
			seen[target.Client] = true
			if target.DetectionStatus != "detected" || !target.Available || target.Profile == nil || target.Profile.ConfigRoot != filepath.Join(home, ".cline") {
				t.Fatal("editor storage was substituted for Cline's native authority", target)
			}
		case "cursor":
			seen[target.Client] = true
			if target.DetectionStatus != "not_detected" || target.Available || target.Profile == nil || target.Profile.ConfigRoot != filepath.Join(home, ".cursor") {
				t.Fatal("leftover Cursor configuration either became selectable or lost profile authority", target)
			}
		}
	}
	if !seen["cline"] || !seen["cursor"] {
		t.Fatal("missing registered client observations", seen)
	}
	if _, err := os.Stat(stateRoot); !os.IsNotExist(err) {
		t.Fatal("read-only discovery touched durable state", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".cline")); !os.IsNotExist(err) {
		t.Fatal("read-only discovery materialized a client profile", err)
	}
}

func TestManagementDiscoveryExcludesUnsupportedAndUnsafeAuthority(t *testing.T) {
	root := t.TempDir()
	registry := clientregistry.Default()
	engine, err := installer.New(installer.Config{StateRoot: filepath.Join(root, "unused-state"), Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	detector := &observedProbingDetector{clients: []domain.DetectedClient{
		{ClientID: domain.ClientChatGPT, Status: domain.DetectionDetected, ConfigRoot: root},
		{ClientID: domain.ClientCline, Status: domain.DetectionDetected, ConfigRoot: "relative/unsafe"},
		{ClientID: domain.ClientCursor, Status: domain.DetectionDetected, ConfigRoot: root, DetectionError: errors.New("private native details must not escape")},
	}}
	var output bytes.Buffer
	app := App{Version: "fixture", Detector: detector, ClientRegistry: registry, Installer: engine}
	if err := runManagementDiscovery(t.Context(), app, &output); err != nil {
		t.Fatal(err)
	}
	var result management.TargetDiscovery
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Targets) != 3 || detector.readOnlyCalls != 1 || detector.probeCalls != 0 || detector.targetedCalls != 0 {
		t.Fatal("discovery bypassed the passive detector", result, detector)
	}
	for _, target := range result.Targets {
		if target.Profile != nil || target.Available || target.Reason == "" {
			t.Fatal("unsafe or unsupported observation became authority", target)
		}
	}
	if bytes.Contains(output.Bytes(), []byte("private native details")) {
		t.Fatal("private detector error escaped")
	}
	for _, args := range [][]string{{"manage", "discover", "--scope", "project"}, {"manage", "discover", "--target", "cline"}, {"manage", "discover", root}} {
		command := NewRoot(app)
		command.SetArgs(args)
		if err := command.ExecuteContext(t.Context()); err == nil {
			t.Fatal("discovery accepted host authority input", args)
		}
	}
	if detector.readOnlyCalls != 1 {
		t.Fatal("refused options invoked discovery")
	}
}

func TestManagementNotDetectedProfileRetainsExistingBindingRemoval(t *testing.T) {
	transport := newDirectTransportFixture(t, "zip", false)
	fixture, _, _ := dependencySpecHarness(t, transport.work, dependencyClientSpec{id: domain.ClientCline})
	fixture.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: transport.server.Client()}
	detected, err := fixture.app.Detector.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cfg := management.Startup{Scope: management.User, StateRoot: filepath.Dir(fixture.app.ManagedRoot), DependenciesRoot: fixture.app.DependenciesRoot, Targets: []management.StartupTarget{{Id: "cline", Client: "cline", ConfigRoot: detected[0].ConfigRoot}}}
	backend, err := NewManagementBackend(fixture.app, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Install, Source: transport.server.URL + "/package.zip", TargetIds: []string{"cline"}}))
	skill := filepath.Join(detected[0].ConfigRoot, "skills", "demo", "SKILL.md")
	if _, err := os.Stat(skill); err != nil {
		t.Fatal("fixture did not materialize the real Cline Skill", err)
	}
	cfg.Targets[0].DetectionStatus = management.ClientNotDetected
	absent, err := NewManagementBackend(fixture.app, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer absent.Close()
	if absent.ContextID() == backend.ContextID() || absent.EnvironmentID() != backend.EnvironmentID() {
		t.Fatal("availability did not revise capture while retaining the durable environment")
	}
	session, err := absent.Session(t.Context())
	if err != nil || len(session.Targets) != 1 || session.Targets[0].Available || session.Targets[0].DetectionStatus != management.ClientNotDetected {
		t.Fatal("missing client remained installable", session, err)
	}
	view, err := absent.Inspect(t.Context())
	if err != nil || len(view.Entries) != 1 || len(view.Entries[0].Bindings) != 1 {
		t.Fatal("existing binding disappeared with the client", view, err)
	}
	transport.mu.Lock()
	downloads := transport.downloads
	transport.mu.Unlock()
	prepared, err := absent.Prepare(t.Context(), management.ChangeRequest{Action: management.Install, Source: transport.server.URL + "/must-not-acquire.zip", TargetIds: []string{"cline"}}, "not-detected")
	var refusal *management.Refusal
	if prepared != nil || !errors.As(err, &refusal) || refusal.Code != "TARGET_UNAVAILABLE" {
		t.Fatal("missing-client install did not refuse before source acquisition", prepared, err)
	}
	transport.mu.Lock()
	unchanged := transport.downloads == downloads
	transport.mu.Unlock()
	if !unchanged {
		t.Fatal("missing-client installation acquired a source")
	}
	managementApply(t, managementPrepare(t, absent, management.ChangeRequest{Action: management.Remove, Aliases: []string{"demo"}, TargetIds: []string{"cline"}}))
	if _, err := os.Stat(skill); !os.IsNotExist(err) {
		t.Fatal("targeted removal left owned Skill bytes", err)
	}
	view, err = absent.Inspect(t.Context())
	if err != nil || len(view.Entries) != 0 {
		t.Fatal("targeted removal left the unavailable client binding", view, err)
	}
	cfg.Targets[0].DetectionStatus = "unknown"
	if _, err := NormalizeManagementStartup(cfg); err == nil {
		t.Fatal("unknown detection status was accepted")
	}
}
