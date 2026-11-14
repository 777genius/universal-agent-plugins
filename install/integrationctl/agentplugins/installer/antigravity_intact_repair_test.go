package installer_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/antigravity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestAntigravityIntactMCPOnlyRepairPreservesDelivery(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			engine, request, statePath, protected := antigravityRepairFixture(t, scope)
			installed := antigravityApply(t, engine, request)
			before := antigravityInstallation(t, statePath, request.ProjectRoot)
			binding := antigravityBinding(t, before)
			if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
				t.Fatalf("native preparation not committed: %+v", binding)
			}
			beforeNative := qoderSDKTree(t, installed.Binding.TargetPath)
			beforeSource := qoderSDKTree(t, request.PackageRoot)
			beforeProtected := make([]map[string]string, len(protected))
			for i, path := range protected {
				beforeProtected[i] = qoderSDKTree(t, path)
			}
			if _, err := os.Stat(filepath.Join(installed.Binding.TargetPath, "mcp.json")); !os.IsNotExist(err) {
				t.Fatalf("portable selection survived native projection: %v", err)
			}
			if _, err := os.Stat(filepath.Join(installed.Binding.TargetPath, "mcp_config.json")); err != nil {
				t.Fatal(err)
			}
			request.Operation, request.OperationID = installer.OpRepair, "intact-mcp-repair"
			repaired := antigravityApply(t, engine, request)
			if repaired.Delivery == nil || len(repaired.Delivery.Components) != 1 {
				t.Fatalf("healthy MCP-only repair omitted its selected component: %+v", repaired.Delivery)
			}
			component := repaired.Delivery.Components[0]
			if component.Kind != string(domain.ComponentMCPServer) || component.Name != "notion" || component.Support != string(domain.SupportNative) || component.Reason != "" {
				t.Fatalf("healthy repair lost native MCP selection: %+v", component)
			}
			after := antigravityInstallation(t, statePath, request.ProjectRoot)
			afterBinding := antigravityBinding(t, after)
			if before.InstallationID != after.InstallationID || !reflect.DeepEqual(before.Source, after.Source) || !reflect.DeepEqual(before.Package, after.Package) || !reflect.DeepEqual(before.DataReceipts, after.DataReceipts) {
				t.Fatal("healthy repair changed the installed package or exact source identity")
			}
			if binding.ClientBindingID != afterBinding.ClientBindingID || binding.Scope != afterBinding.Scope || binding.TargetLocator != afterBinding.TargetLocator || binding.PhysicalArtifact != afterBinding.PhysicalArtifact || binding.DataReceiptID != afterBinding.DataReceiptID || !reflect.DeepEqual(binding.NativeObjects, afterBinding.NativeObjects) || afterBinding.Activation != domain.ActivationPrepared || afterBinding.Verification != domain.VerificationInstalled {
				t.Fatal("healthy repair changed binding, artifact/data ownership or preparation state")
			}
			if !reflect.DeepEqual(beforeNative, qoderSDKTree(t, installed.Binding.TargetPath)) || !reflect.DeepEqual(beforeSource, qoderSDKTree(t, request.PackageRoot)) {
				t.Fatal("healthy repair changed native package or source bytes")
			}
			for i, path := range protected {
				if !reflect.DeepEqual(beforeProtected[i], qoderSDKTree(t, path)) {
					t.Fatalf("healthy repair changed foreign or unselected scope tree %s", path)
				}
			}
		})
	}
}

func antigravityRepairFixture(t *testing.T, scope domain.InstallScope) (*installer.Engine, installer.Request, string, []string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, project, other, pkg := filepath.Join(base, "user"), filepath.Join(base, "project"), filepath.Join(base, "other-project"), filepath.Join(base, "package")
	for _, root := range []string{user, project, other} {
		kimiSDKWrite(t, filepath.Join(root, "sentinel"), "unselected bytes")
	}
	kimiSDKWrite(t, filepath.Join(project, ".git", "HEAD"), "ref: refs/heads/main\n")
	foreign := filepath.Join(user, "plugins", "foreign")
	kimiSDKWrite(t, filepath.Join(foreign, "plugin.json"), `{"name":"foreign","version":"1.0.0"}`)
	kimiSDKWrite(t, filepath.Join(foreign, "sentinel"), "foreign native bytes")
	kimiSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"notion","version":"1.0.0"}`)
	kimiSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"notion":{"type":"streamable-http","url":"https://example.test/mcp"}}}`)
	helper := filepath.Join(base, "inert-helper")
	kimiSDKWrite(t, helper, "MCP-only native fixture; helper is never executed\n")
	if err := os.Chmod(helper, 0700); err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(antigravity.New())
	if err != nil {
		t.Fatal(err)
	}
	config := installer.Config{StateRoot: filepath.Join(base, "state"), Registry: registry, TrustedLocalPackages: true, EnableNativeObserver: true, Runner: processadapter.OS{}, HelperExecutable: helper}
	request := installer.Request{Scope: scope, Operation: installer.OpInstall, PackageRoot: pkg, ClientID: "antigravity", ClientConfigRoot: user, InstallationID: "00000000-0000-4000-8000-000000001114", OperationID: "install-mcp-only", RequiredComponents: []string{"mcp"}}
	protected := []string{project, other, foreign}
	if scope == domain.ScopeProject {
		config.ProjectRoot, request.ProjectRoot = project, project
		config.StateRoot = filepath.Join(project, ".uap")
		protected = []string{user, other}
	}
	engine, err := installer.New(config)
	if err != nil {
		t.Fatal(err)
	}
	return engine, request, filepath.Join(config.StateRoot, "state-v2.json"), protected
}

func antigravityApply(t *testing.T, engine *installer.Engine, request installer.Request) installer.Result {
	t.Helper()
	handle, err := engine.Prepare(t.Context(), request)
	if err != nil {
		t.Fatalf("prepare %s: %v", request.Operation, err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	result, err := engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
	if err != nil || result.Outcome != installer.OutcomeCompleted && result.Outcome != installer.OutcomeUnchanged {
		t.Fatalf("apply %s: result=%+v err=%v", request.Operation, result, err)
	}
	return result
}

func antigravityInstallation(t *testing.T, path, project string) domain.Installation {
	t.Helper()
	state, err := (statev2.Store{Path: path, ProjectRoot: project}).Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("installed state=%+v err=%v", state, err)
	}
	return state.Installations[0]
}

func antigravityBinding(t *testing.T, installation domain.Installation) domain.ClientBinding {
	t.Helper()
	if len(installation.Clients) != 1 {
		t.Fatalf("expected one exact native binding, got %+v", installation.Clients)
	}
	for _, binding := range installation.Clients {
		if binding.ClientID != string(domain.ClientAntigravity) {
			t.Fatalf("unexpected native client: %s", binding.ClientID)
		}
		return binding
	}
	t.Fatal("missing native binding")
	return domain.ClientBinding{}
}
