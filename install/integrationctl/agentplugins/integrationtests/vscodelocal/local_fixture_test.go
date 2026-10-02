package vscodelocal_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// This TEST executable contains the genuine public helper dispatch. It is
// recorded by NewSource when supplied; no helper readiness is fabricated.
func TestMain(m *testing.M) {
	if handled, code := managedstdio.Dispatch(os.Args[1:], os.Stderr); handled {
		os.Exit(code)
	}
	if len(os.Args) > 1 && os.Args[1] == "--TEST-argv" {
		_ = json.NewEncoder(os.Stdout).Encode(os.Args[2:])
		return
	}
	os.Exit(m.Run())
}

type localFixture struct {
	root, pkg, settings, state, runtime string
	config                              vscode.LocalConfig
	adapter                             *vscode.LocalAdapter
	registry                            *clients.Registry
}

func freshLocal(t *testing.T, optional bool) *localFixture {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("Linux source tuple; native Windows qualification remains separate")
	}
	root := t.TempDir()
	t.Cleanup(func() {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				return os.Chmod(path, 0700)
			}
			return err
		})
	})
	f := &localFixture{root: root, pkg: filepath.Join(root, "package"), settings: filepath.Join(root, "selected-profile", "settings.json"), state: filepath.Join(root, "state")}
	f.runtime = filepath.Join(root, "TEST runtime ' Ω $(touch sentinel)")
	executable, err := os.Executable()
	must(t, err)
	body, err := os.ReadFile(executable)
	must(t, err)
	writeLocal(t, f.runtime, body, 0700)
	writeLocal(t, f.settings, []byte("{\n // foreign comment\n \"foreign\": {\"number\":1e2},\n \"chat.pluginLocations\":{\"/TEST-disabled-sibling\":false,},\n}\n"), 0600)
	writeLocal(t, filepath.Join(f.pkg, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"test-local","version":"1.0.0"}`), 0600)
	writeLocal(t, filepath.Join(f.pkg, "com.example.opaque", "preserved.txt"), []byte("TEST opaque namespace bytes"), 0600)
	writeLocal(t, filepath.Join(f.pkg, "hooks", "root.json"), []byte("TEST unsupported root hook"), 0600)
	if optional {
		writeLocal(t, filepath.Join(f.pkg, "mcp.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"notify":{"type":"stdio","command":"./bin/TEST-server","args":["--TEST-argv","${PLUGIN_DATA}/mcp-locator"]}}}`), 0600)
		writeLocal(t, filepath.Join(f.pkg, "bin", "TEST-server"), body, 0700)
		writeLocal(t, filepath.Join(f.pkg, "skills", "notify", "SKILL.md"), []byte("---\nname: notify\ndescription: TEST optional notification\n---\nTEST skill\n"), 0600)
	}
	f.config = vscode.LocalConfig{ProfileSettingsPath: f.settings, QualifiedTuple: vscode.SourceQualifiedTESTTuple("linux"), TargetShell: linuxTarget(), NativeStop: true, HookSpecs: localSpecs(f.runtime)}
	declared := declaredHooks(t, f.config)
	writeLocal(t, filepath.Join(f.pkg, hookPath()), declared, 0600)
	f.config.DeclaredHookDigest = testDigest(declared)
	f.adapter, err = vscode.NewLocal(f.config)
	must(t, err)
	f.registry, err = clients.NewRegistry(f.adapter)
	must(t, err)
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func writeLocal(t *testing.T, path string, body []byte, mode os.FileMode) {
	t.Helper()
	must(t, os.MkdirAll(filepath.Dir(path), 0700))
	must(t, os.WriteFile(path, body, mode))
}
func readLocal(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	must(t, err)
	return body
}
func testDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func runLocalProcess(t *testing.T, command *exec.Cmd) ([]byte, error) {
	t.Helper()
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	argv, marshalErr := json.Marshal(command.Args)
	must(t, marshalErr)
	if command.ProcessState == nil {
		t.Fatal("TEST process did not start", err)
	}
	t.Logf("TEST process argv=%s exit=%d stdout_sha256=%s stderr_sha256=%s stdout_bytes=%d stderr_bytes=%d", argv, command.ProcessState.ExitCode(), testDigest(output), testDigest(stderr.Bytes()), len(output), stderr.Len())
	return output, err
}

func (f *localFixture) envelope(t *testing.T) domain.PackageEnvelope {
	t.Helper()
	snapshot, err := (packagedigest.Builder{TempRoot: f.root}).Snapshot(t.Context(), f.pkg, domain.SourceIdentity{})
	must(t, err)
	t.Cleanup(func() { _ = packagedigest.Remove(snapshot) })
	schemas, err := specregistry.New()
	must(t, err)
	env, err := (loader.Loader{Registry: schemas}).LoadSnapshot(t.Context(), snapshot)
	must(t, err)
	return env
}
func (f *localFixture) plan(t *testing.T, env domain.PackageEnvelope) domain.DeliveryPlan {
	t.Helper()
	plan, err := (planner.Planner{Registry: f.registry, Paths: pathpolicy.Policy{}, ManagedRoot: filepath.Join(f.state, "managed")}).Plan(t.Context(), domain.PlanRequest{Envelope: env, Client: domain.DetectedClient{ClientID: domain.ClientVSCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Dir(f.settings)}, Scope: domain.ScopeUser, PhysicalArtifactID: "TEST-installation"})
	must(t, err)
	if plan.Status == domain.PlanUnsupported {
		t.Fatalf("Local plan unsupported: %+v", plan)
	}
	return plan
}
func (f *localFixture) engine(t *testing.T, helper bool) *installer.Engine {
	t.Helper()
	cfg := installer.Config{StateRoot: f.state, Registry: f.registry, TrustedLocalPackages: true, EnableNativeObserver: true, Runner: noLocalCommands{t: t}}
	if helper {
		executable, err := os.Executable()
		must(t, err)
		cfg.HelperExecutable = executable
	}
	engine, err := installer.New(cfg)
	must(t, err)
	return engine
}
func (f *localFixture) request(op installer.Operation, id string) installer.Request {
	return installer.Request{Operation: op, PackageRoot: f.pkg, ClientID: "vscode", ClientExecutable: f.runtime, ClientConfigRoot: filepath.Dir(f.settings), InstallationID: id}
}
func applyLocal(t *testing.T, engine *installer.Engine, req installer.Request) installer.Result {
	t.Helper()
	handle, err := engine.Prepare(t.Context(), req)
	must(t, err)
	t.Cleanup(func() { _ = handle.Close() })
	result, err := engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
	must(t, err)
	if result.Outcome == installer.OutcomeConflict || result.Outcome == installer.OutcomeRecovery {
		t.Fatalf("Local lifecycle refused: %+v", result)
	}
	return result
}

type noLocalCommands struct{ t *testing.T }

func (r noLocalCommands) Run(_ context.Context, command ports.Command) (ports.CommandResult, error) {
	r.t.Errorf("Local attempted command %+v", command)
	return ports.CommandResult{}, fmt.Errorf("TEST Local must never use CLI")
}

func (f *localFixture) stage(t *testing.T) (domain.PackageEnvelope, domain.DeliveryPlan, domain.StagedDelivery) {
	t.Helper()
	env := f.envelope(t)
	plan := f.plan(t, env)
	must(t, os.MkdirAll(plan.TargetRoot, 0700))
	data := filepath.Join(f.root, "TEST plugin data")
	must(t, os.MkdirAll(data, 0700))
	staged, err := (providers.Stager{Registry: f.registry, Paths: pathpolicy.Policy{}}).StageWithPluginData(t.Context(), env, plan, "TEST-stage", domain.CompatibilityHints{}, data)
	must(t, err)
	selected, err := plan.SelectedDelivery.WithProjectionDigest(staged.ArtifactDigest)
	must(t, err)
	plan.SelectedDelivery = selected
	return env, plan, staged
}

func assertForeign(t *testing.T, body []byte) {
	t.Helper()
	for _, text := range []string{"// foreign comment", `"number":1e2`, `"/TEST-disabled-sibling":false`} {
		if !strings.Contains(string(body), text) {
			t.Fatalf("foreign bytes lost: %s", text)
		}
	}
}
