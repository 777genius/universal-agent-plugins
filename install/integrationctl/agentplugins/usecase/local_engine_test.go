package usecase_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Red: a CLI-backed binding is silently reused by a Local selection, allowing
// CLI listing/activation to masquerade as evidence about the selected profile.
func TestLocalEngineRejectsHistoricalModeConfusion(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(func() {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				return os.Chmod(path, 0700)
			}
			return err
		})
	})
	pkg := filepath.Join(root, "package")
	if err := os.MkdirAll(pkg, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"local-test","version":"1.0.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(root, "TEST-executable")
	if err := os.WriteFile(probe, []byte("TEST fixture never executed"), 0700); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(root, "state")
	req := installer.Request{Operation: installer.OpInstall, PackageRoot: pkg, ClientID: "vscode", ClientExecutable: probe, ClientConfigRoot: filepath.Join(root, "profile")}
	eng := localTestEngine(t, stateRoot, vscode.New())
	handle, err := eng.Prepare(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = eng.Apply(context.Background(), handle, installer.Decision{Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(stateRoot, "state-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	local := &testLocalAdapter{Adapter: vscode.New()}
	eng = localTestEngine(t, stateRoot, local)
	handle, err = eng.Prepare(context.Background(), req)
	if handle != nil {
		defer func() { _ = handle.Close() }()
	}
	if err == nil || !strings.Contains(err.Error(), "delivery mode") {
		t.Fatalf("Local accepted historical CLI binding: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(stateRoot, "state-v2.json"))
	if err != nil || string(before) != string(after) {
		t.Fatalf("mode conflict changed state: %v", err)
	}
}

type testLocalAdapter struct {
	*vscode.Adapter
	targetShell string
	nativeStop  *bool
}

func (a *testLocalAdapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	plan.NativeRegistryRoot = in.Client.ConfigRoot
	enabled := true
	shell := a.targetShell
	if shell == "" {
		shell = "bash"
	}
	nativeStop := true
	if a.nativeStop != nil {
		nativeStop = *a.nativeStop
	}
	return clients.SelectLocalDelivery(plan, domain.LocalDeliveryFacts{ProfileRoot: in.Client.ConfigRoot, SettingsPath: filepath.Join(in.Client.ConfigRoot, "settings.json"), ProfileIdentity: "TEST-profile", SettingsIdentity: "TEST-settings", Tuple: domain.LocalQualifiedTuple{VSCodeVersion: "TEST-code", CopilotVersion: "TEST-copilot", TargetOS: "linux", TargetShell: shell, QualificationID: "TEST-process-contract"}, NativeStop: nativeStop, CanonicalDigest: in.Envelope.TreeDigest, Registration: domain.OwnedProfileEntry{ObjectID: "TEST-profile-entry", Selector: plan.ActivePath, DesiredValue: &enabled}})
}
func localTestEngine(t *testing.T, stateRoot string, adapter clients.Adapter) *installer.Engine {
	t.Helper()
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	eng, err := installer.New(installer.Config{HelperExecutable: helper, StateRoot: stateRoot, Registry: registry, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	return eng
}
