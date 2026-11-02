package installer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type kiroSDKFixture struct {
	project, user, pkg, data string
	engine                   *installer.Engine
	request                  installer.Request
}

func kiroSDKWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func kiroSDKRead(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func newKiroSDKFixture(t *testing.T, project, user, pkg, helper string) *kiroSDKFixture {
	t.Helper()
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(kiro.New())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{ProjectRoot: project, StateRoot: filepath.Join(project, ".uap"), Registry: registry, HelperExecutable: helper, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	return &kiroSDKFixture{project: project, user: user, pkg: pkg, engine: engine, request: installer.Request{Scope: domain.ScopeProject, ProjectRoot: project, Operation: installer.OpInstall, PackageRoot: pkg, ClientID: "kiro", ClientConfigRoot: user,
		InstallationID: "00000000-0000-4000-8000-000000002331", OperationID: "kiro-project-sdk", RequiredComponents: []string{"skills", "mcp"}}}
}

func (f *kiroSDKFixture) apply(t *testing.T, op installer.Operation, label string) installer.Result {
	t.Helper()
	f.request.Operation, f.request.OperationID = op, label
	handle, err := f.engine.Prepare(t.Context(), f.request)
	if err != nil {
		t.Fatalf("prepare Kiro SDK %s: %v", label, err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	result, err := f.engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
	if err != nil || result.Outcome != installer.OutcomeCompleted && result.Outcome != installer.OutcomeUnchanged {
		t.Fatalf("Kiro SDK %s=%+v,%v", label, result, err)
	}
	if result.Binding.DataRoot != "" {
		if f.data != "" && f.data != result.Binding.DataRoot {
			t.Fatal("SDK changed selected persistent data authority")
		}
		f.data = result.Binding.DataRoot
	}
	return result
}

func (f *kiroSDKFixture) binding(t *testing.T) domain.ClientBinding {
	t.Helper()
	state, err := (statev2.Store{Path: filepath.Join(f.project, ".uap", "state-v2.json"), ProjectRoot: f.project}).Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("Kiro SDK state=%+v,%v", state, err)
	}
	for _, binding := range state.Installations[0].Clients {
		return binding
	}
	t.Fatal("Kiro SDK binding absent")
	return domain.ClientBinding{}
}
