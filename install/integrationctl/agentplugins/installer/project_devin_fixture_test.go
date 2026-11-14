package installer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type devinSDKFixture struct {
	project, user, pkg, data string
	engine                   *installer.Engine
	request                  installer.Request
}

func devinSDKWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func devinSDKRead(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func newDevinSDKFixture(t *testing.T, project, user, pkg, helper string) *devinSDKFixture {
	t.Helper()
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	if helper == "" {
		var err error
		helper, err = os.Executable()
		if err != nil {
			t.Fatal(err)
		}
	}
	registry, err := clients.NewRegistry(windsurf.New())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{ProjectRoot: project, StateRoot: filepath.Join(project, ".uap"), Registry: registry, HelperExecutable: helper, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	return &devinSDKFixture{project: project, user: user, pkg: pkg, engine: engine, request: installer.Request{Scope: domain.ScopeProject, ProjectRoot: project, Operation: installer.OpInstall, PackageRoot: pkg, ClientID: "windsurf", ClientConfigRoot: user, ClientExecutable: filepath.Join(filepath.Dir(project), "devin"), ClientVersion: "3000.11.205",
		InstallationID: "00000000-0000-4000-8000-000000002331", OperationID: "devin-project-sdk", RequiredComponents: []string{"skills", "mcp"}}}
}

func (f *devinSDKFixture) apply(t *testing.T, op installer.Operation, label string) installer.Result {
	t.Helper()
	f.request.Operation, f.request.OperationID = op, label
	handle, err := f.engine.Prepare(t.Context(), f.request)
	if err != nil {
		t.Fatalf("prepare Devin SDK %s: %v", label, err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	if op != installer.OpRemove && len(handle.Plan().Delivery.Components) != 2 {
		t.Fatalf("Devin SDK omitted supported fixture component: %+v", handle.Plan())
	}
	for _, component := range handle.Plan().Delivery.Components {
		if component.Support == "unsupported" {
			t.Fatalf("Devin SDK unsupported fixture: %+v", handle.Plan())
		}
	}
	result, err := f.engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
	if err != nil || result.Outcome != installer.OutcomeCompleted && result.Outcome != installer.OutcomeUnchanged {
		t.Fatalf("Devin SDK %s=%+v,%v", label, result, err)
	}
	if result.Binding.DataRoot != "" {
		if f.data != "" && f.data != result.Binding.DataRoot {
			t.Fatal("SDK changed selected persistent data authority")
		}
		f.data = result.Binding.DataRoot
	}
	return result
}

func (f *devinSDKFixture) binding(t *testing.T) domain.ClientBinding {
	t.Helper()
	state, err := (statev2.Store{Path: filepath.Join(f.project, ".uap", "state-v2.json"), ProjectRoot: f.project}).Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("Devin SDK state=%+v,%v", state, err)
	}
	for _, binding := range state.Installations[0].Clients {
		return binding
	}
	t.Fatal("Devin SDK binding absent")
	return domain.ClientBinding{}
}

func devinSDKConfig(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		Servers map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	return document.Servers, nil
}

func devinSDKTemp(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
