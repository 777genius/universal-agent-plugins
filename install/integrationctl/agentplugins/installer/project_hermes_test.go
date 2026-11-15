package installer_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/hermes"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type hermesSDKFixture struct {
	project, user, data string
	engine              *installer.Engine
	request             installer.Request
	runner              *clineSDKNoNative
}

func newHermesSDKFixture(t *testing.T, project, user, pkg string) *hermesSDKFixture {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(hermes.New())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runner := &clineSDKNoNative{}
	engine, err := installer.New(installer.Config{ProjectRoot: project, StateRoot: filepath.Join(project, ".uap"), Registry: registry,
		Runner: runner, EnableNativeObserver: true, TrustedLocalPackages: true, HelperExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	return &hermesSDKFixture{project: project, user: user, engine: engine, runner: runner, request: installer.Request{
		Scope: domain.ScopeProject, ProjectRoot: project, PackageRoot: pkg, ClientID: "hermes", ClientConfigRoot: user,
		ClientExecutable: hermesSDKBundle(t, filepath.Dir(project)), ClientVersion: "0.23.0",
		InstallationID: "00000000-0000-4000-8000-000000000023", RequiredComponents: []string{"skills", "mcp"},
	}}
}

func (fixture *hermesSDKFixture) apply(t *testing.T, operation installer.Operation, label string) installer.Result {
	t.Helper()
	fixture.request.Operation, fixture.request.OperationID = operation, label
	handle, err := fixture.engine.Prepare(t.Context(), fixture.request)
	if err != nil {
		t.Fatalf("prepare Hermes %s: %v", label, err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	result, err := fixture.engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
	if err != nil || result.Outcome != installer.OutcomeCompleted && result.Outcome != installer.OutcomeUnchanged {
		t.Fatalf("Hermes %s: %+v %v", label, result, err)
	}
	if result.Binding.DataRoot != "" {
		if fixture.data != "" && fixture.data != result.Binding.DataRoot {
			t.Fatal("Hermes changed persistent data authority")
		}
		fixture.data = result.Binding.DataRoot
	}
	return result
}

func (fixture *hermesSDKFixture) state(t *testing.T) domain.StateFileV2 {
	t.Helper()
	state, err := (statev2.Store{Path: filepath.Join(fixture.project, ".uap", "state-v2.json"), ProjectRoot: fixture.project}).Load()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func (fixture *hermesSDKFixture) binding(t *testing.T) domain.ClientBinding {
	t.Helper()
	for _, installation := range fixture.state(t).Installations {
		if installation.InstallationID == fixture.request.InstallationID {
			for _, binding := range installation.Clients {
				return binding
			}
		}
	}
	t.Fatal("Hermes binding missing")
	return domain.ClientBinding{}
}

// This is the real public SDK composition and transaction kernel with the
// candidate adapter injected. Every native process remains forbidden.
func TestProjectHermesSDKLifecycleKeepsRootReceiptAndUserIsolation(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, pkg := filepath.Join(base, "user"), filepath.Join(base, "package")
	const userConfig = "model: original\nmcp_servers:\n  foreign:\n    command: original\n"
	const skill = "---\nname: docs\ndescription: Inert Hermes fixture\n---\nOriginal.\n"
	claudeSDKWrite(t, filepath.Join(user, "config.yaml"), userConfig)
	claudeSDKWrite(t, filepath.Join(user, "auth.json"), "original credentials; never loaded")
	claudeSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-hermes","version":"1.0.0"}`)
	claudeSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), skill)
	claudeSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/server.js","${PLUGIN_DATA}"],"cwd":"${PLUGIN_DATA}"}}}`)
	claudeSDKWrite(t, filepath.Join(pkg, "server.js"), "// Inert; never executed.\n")
	a, b := newHermesSDKFixture(t, filepath.Join(base, "A"), user, pkg), newHermesSDKFixture(t, filepath.Join(base, "B"), user, pkg)
	for _, fixture := range []*hermesSDKFixture{a, b} {
		fixture.apply(t, installer.OpInstall, "first-install")
		binding := fixture.binding(t)
		state := fixture.state(t)
		if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || binding.NativeActivationAttempt != "" || len(state.TransactionReceipts) != 1 || len(binding.Receipts) != 1 || fixture.data == "" {
			t.Fatalf("Hermes package/root lifecycle differs: %+v / %+v", binding, state.TransactionReceipts)
		}
		claudeSDKWrite(t, filepath.Join(fixture.data, "keep"), fixture.project)
		config := claudeSDKBytes(t, filepath.Join(fixture.project, ".hermes", "uap-session", "config.yaml"))
		if !strings.Contains(config, binding.TargetLocator) || !strings.Contains(config, fixture.data) || strings.Contains(config, "${PLUGIN_") {
			t.Fatalf("Hermes did not expand exact package/data selection: %s", config)
		}
		hermesSDKSession(t, fixture, binding)
		if result := fixture.apply(t, installer.OpInstall, "repeat-install"); result.Mutated {
			t.Fatalf("repeat changed already prepared package: %+v", result)
		}
	}
	if a.data == b.data {
		t.Fatal("A/B share persistent data")
	}
	beforeB := b.state(t)
	hermesSDKUpdateRepairRemove(t, a, pkg, skill)
	if !reflect.DeepEqual(beforeB, b.state(t)) || claudeSDKBytes(t, filepath.Join(user, "config.yaml")) != userConfig || claudeSDKBytes(t, filepath.Join(user, "auth.json")) != "original credentials; never loaded" {
		t.Fatal("A lifecycle changed B or original User")
	}
	for _, fixture := range []*hermesSDKFixture{a, b} {
		if fixture.runner.calls != 0 || claudeSDKBytes(t, filepath.Join(fixture.data, "keep")) != fixture.project {
			t.Fatal("Hermes lifecycle ran native code or removed data")
		}
	}
}

func hermesSDKSession(t *testing.T, fixture *hermesSDKFixture, binding domain.ClientBinding) {
	t.Helper()
	store := statev2.Store{Path: filepath.Join(fixture.project, ".uap", "state-v2.json"), ProjectRoot: fixture.project}
	request := clients.SessionRequest{ProjectRoot: fixture.project, Inspect: true, Bindings: []domain.ClientBinding{binding},
		Client: domain.DetectedClient{ClientID: domain.ClientHermes, ConfigRoot: fixture.user,
			ExecutablePath: fixture.request.ClientExecutable, Status: domain.DetectionDetected,
			Surfaces: []domain.ClientSurface{{ID: "hermes_cli", Detected: true}}}}
	env := clients.Env{ProjectRoots: transaction.ProjectRootPublisher{Kernel: transaction.Kernel{StateStore: store}}, Runner: fixture.runner}
	before := fixture.state(t)
	plan, err := hermes.New().PlanSession(t.Context(), env, request)
	if err != nil || plan.Environment["HERMES_HOME"] != filepath.Join(fixture.project, ".hermes", "uap-session") ||
		!reflect.DeepEqual(plan.Servers, []string{"docs"}) || !reflect.DeepEqual(plan.Arguments, []string{"config", "get", "mcp_servers", "--json"}) {
		t.Fatalf("session did not consume actual committed root/entry receipts: %+v %v", plan, err)
	}
	if !reflect.DeepEqual(before, fixture.state(t)) || fixture.runner.calls != 0 {
		t.Fatal("session planning changed state or ran native client")
	}
}

func hermesSDKUpdateRepairRemove(t *testing.T, fixture *hermesSDKFixture, pkg, original string) {
	t.Helper()
	skill := filepath.Join(fixture.project, ".hermes", "skills", "docs", "SKILL.md")
	if err := os.RemoveAll(filepath.Dir(skill)); err != nil {
		t.Fatal(err)
	}
	fixture.apply(t, installer.OpRepair, "repair-missing-skill")
	if claudeSDKBytes(t, skill) != original {
		t.Fatal("repair failed to restore selected skill")
	}
	claudeSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), strings.ReplaceAll(original, "Original.", "Updated."))
	fresh := newHermesSDKFixture(t, fixture.project, fixture.user, pkg)
	fresh.data = fixture.data
	*fixture = *fresh
	fixture.apply(t, installer.OpUpdate, "update-after-restart")
	if claudeSDKBytes(t, skill) == original {
		t.Fatal("update did not advance package")
	}
	binding := fixture.binding(t)
	root := filepath.Join(fixture.project, ".hermes", "uap-session")
	marker := claudeSDKBytes(t, filepath.Join(root, ".uap-owner"))
	view, err := fixture.engine.Inspect(t.Context())
	if err != nil || view.Recovery.Required {
		t.Fatalf("Hermes restart requires recovery: %+v %v", view, err)
	}
	fixture.apply(t, installer.OpRemove, "remove")
	if _, err := os.Stat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("managed package survived removal: %v", err)
	}
	if _, err := os.Stat(skill); !os.IsNotExist(err) {
		t.Fatalf("owned skill survived removal: %v", err)
	}
	if got := claudeSDKBytes(t, filepath.Join(root, "config.yaml")); strings.Contains(got, "docs:") || claudeSDKBytes(t, filepath.Join(root, ".uap-owner")) != marker {
		t.Fatalf("remove lost selected root or retained MCP: %s", got)
	}
	rootReceipts := 0
	for _, receipt := range fixture.state(t).TransactionReceipts {
		if strings.HasPrefix(receipt.AfterDigest, "native-root-v1:") && receipt.Phase == "committed" {
			rootReceipts++
		}
	}
	if rootReceipts != 1 {
		t.Fatal("removal lost or duplicated auxiliary authority")
	}
}

func hermesSDKBundle(t *testing.T, base string) string {
	t.Helper()
	payload := filepath.Join(base, "inert-bundle")
	launcher := "hermes"
	if runtime.GOOS == "windows" {
		launcher += ".exe"
	}
	target := map[string]string{"linux/amd64": "linux-x64", "darwin/arm64": "darwin-arm64", "windows/amd64": "win32-x64"}[runtime.GOOS+"/"+runtime.GOARCH]
	for _, name := range []string{"hermes-agent", "venv/site", "tools/python", "bin"} {
		if err := os.MkdirAll(filepath.Join(payload, filepath.FromSlash(name)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"bin/" + launcher, "tools/python/python"} {
		if err := os.WriteFile(filepath.Join(payload, filepath.FromSlash(name)), []byte("inert fixture; never execute"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := fmt.Sprintf(`{"schema":1,"ref":"2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19","target":%q,"repo":"hermes-agent","venv":"venv","store":"tools","launchers":["hermes"],"runtime":{"repoDir":"hermes-agent","toolsDir":"tools","storePython":"tools/python/python","sitePackages":"venv/site","commands":{"hermes":%q}}}`, target, "bin/"+launcher)
	if err := os.WriteFile(filepath.Join(payload, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(payload, "bin", launcher)
}

func TestProjectHermesSDKSourceInstallRefusesBeforeMutation(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, pkg := filepath.Join(base, "user"), filepath.Join(base, "package")
	claudeSDKWrite(t, filepath.Join(user, "config.yaml"), "model: foreign\n")
	claudeSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-hermes","version":"1.0.0"}`)
	claudeSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://example.test/mcp"}}}`)
	fixture := newHermesSDKFixture(t, filepath.Join(base, "A"), user, pkg)
	fixture.request.RequiredComponents = []string{"mcp"}
	fixture.request.ClientExecutable = filepath.Join(base, "source", "hermes")
	claudeSDKWrite(t, fixture.request.ClientExecutable, "inert source install; never execute")
	fixture.request.Operation, fixture.request.OperationID = installer.OpInstall, "source-refusal"
	handle, err := fixture.engine.Prepare(t.Context(), fixture.request)
	if handle != nil {
		if closeErr := handle.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("source-installed request produced an applicable handle")
	}
	if err == nil || !strings.Contains(err.Error(), "delivery plan for hermes is unsupported") || fixture.runner.calls != 0 {
		t.Fatal("source-installed request reached beyond unsupported planning", err)
	}
	if claudeSDKBytes(t, filepath.Join(user, "config.yaml")) != "model: foreign\n" {
		t.Fatal("unsupported source install changed User")
	}
	if _, err := os.Lstat(filepath.Join(fixture.project, ".hermes", "uap-session")); !os.IsNotExist(err) {
		t.Fatal("unsupported source install created a selected runtime", err)
	}
}
