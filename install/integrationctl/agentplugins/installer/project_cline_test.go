package installer_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type clineSDKNoNative struct{ calls int }

func (r *clineSDKNoNative) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return legacyports.CommandResult{}, fmt.Errorf("unexpected native command: %v", command.Argv)
}

type clineSDKFixture struct {
	project, state string
	engine         *installer.Engine
	request        installer.Request
	runner         *clineSDKNoNative
}

func newClineSDKFixture(t *testing.T, project, user, pkg string, runner *clineSDKNoNative) *clineSDKFixture {
	t.Helper()
	registry, err := clients.NewRegistry(cline.New())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f := &clineSDKFixture{project: project, state: filepath.Join(project, ".uap"), runner: runner}
	f.engine, err = installer.New(installer.Config{ProjectRoot: project, StateRoot: f.state, Registry: registry, Runner: runner, EnableNativeObserver: true, TrustedLocalPackages: true, HelperExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	f.request = installer.Request{Scope: domain.ScopeProject, ProjectRoot: project, Operation: installer.OpInstall, PackageRoot: pkg, ClientID: "cline", ClientConfigRoot: user, InstallationID: "00000000-0000-4000-8000-000000000073", OperationID: "cline-project-install", RequiredComponents: []string{"skills", "mcp"}}
	return f
}

func (f *clineSDKFixture) execute(t *testing.T, request installer.Request) (installer.Result, error) {
	t.Helper()
	handle, err := f.engine.Prepare(t.Context(), request)
	if err != nil {
		return installer.Result{}, err
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	return f.engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
}
func (f *clineSDKFixture) apply(t *testing.T, op installer.Operation) installer.Result {
	t.Helper()
	req := f.request
	req.Operation, req.OperationID = op, string(op)+"-Cline"
	out, err := f.execute(t, req)
	if err != nil || out.Outcome != installer.OutcomeCompleted && out.Outcome != installer.OutcomeUnchanged {
		t.Fatalf("%s: %+v %v", op, out, err)
	}
	return out
}
func (f *clineSDKFixture) stateBytes(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.state, "state-v2.json"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
func (f *clineSDKFixture) binding(t *testing.T) domain.ClientBinding {
	t.Helper()
	state, err := (statev2.Store{Path: filepath.Join(f.state, "state-v2.json"), ProjectRoot: f.project}).Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("state: %+v %v", state, err)
	}
	for _, b := range state.Installations[0].Clients {
		return b
	}
	t.Fatal("missing binding")
	return domain.ClientBinding{}
}

// The real public installer/core, state and file transactions run. The runner
// refuses every command: local Prepared verification must not need Cline.
func TestProjectClinePublicEarlyAttestationHasNoStateOrNativeEffect(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project, user, pkg := filepath.Join(base, "A"), filepath.Join(base, "home", ".cline"), filepath.Join(base, "package")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	claudeSDKWrite(t, filepath.Join(user, "keep"), "original user")
	claudeSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-cline","version":"1.0.0"}`)
	claudeSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Cline file fixture\n---\nPinned original.\n")
	claudeSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"http://127.0.0.1:9/inert"}}}`)
	runner := &clineSDKNoNative{}
	f := newClineSDKFixture(t, project, user, pkg, runner)
	reject := func(op installer.Operation) {
		t.Helper()
		before, calls := f.stateBytes(t), runner.calls
		req := f.request
		req.Operation, req.OperationID, req.ActivationComplete = op, "refuse-"+string(op), true
		if out, err := f.execute(t, req); err == nil {
			t.Fatalf("attestation accepted: %+v", out)
		}
		if before != f.stateBytes(t) || runner.calls != calls {
			t.Fatal("early refusal changed persisted state or called native client")
		}
	}
	reject(installer.OpInstall)
	f.apply(t, installer.OpInstall)
	binding := f.binding(t)
	if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || binding.NativeProfileRoot != project {
		t.Fatalf("fabricated native activation: %+v", binding)
	}
	skill := filepath.Join(project, ".cline", "skills", "docs", "SKILL.md")
	original := claudeSDKBytes(t, skill)
	config := claudeSDKBytes(t, filepath.Join(project, ".uap", "clients", "cline", "mcp.json"))
	for _, op := range []installer.Operation{installer.OpInstall, installer.OpUpdate, installer.OpRepair} {
		reject(op)
	}
	f = newClineSDKFixture(t, project, user, pkg, runner)
	for _, op := range []installer.Operation{installer.OpInstall, installer.OpUpdate, installer.OpRepair} {
		reject(op)
	}
	if original != claudeSDKBytes(t, skill) || config != claudeSDKBytes(t, filepath.Join(project, ".uap", "clients", "cline", "mcp.json")) {
		t.Fatal("refused public request changed owned file bytes")
	}
	before := runner.calls
	view, err := f.engine.Inspect(t.Context())
	if err != nil || view.Recovery.Required || runner.calls != before {
		t.Fatalf("status is not host-free: %+v %v", view, err)
	}
	f.apply(t, installer.OpInstall)
	if !reflect.DeepEqual(binding, f.binding(t)) {
		t.Fatal("unchanged Prepared replay changed persisted binding")
	}
	if !strings.Contains(config, "127.0.0.1:9") {
		t.Fatal("no actual prepared MCP file")
	}
	f.apply(t, installer.OpRemove)
	if _, err := os.Stat(filepath.Dir(skill)); !os.IsNotExist(err) {
		t.Fatal("owned skill survives removal", err)
	}
	if runner.calls != 0 {
		t.Fatal("project lifecycle called native Cline")
	}
	if claudeSDKBytes(t, filepath.Join(user, "keep")) != "original user" {
		t.Fatal("original profile changed")
	}
}
