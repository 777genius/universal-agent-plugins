package usecase_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"
)

type physicalLocal struct {
	*vscode.LocalAdapter
	captures int
}

func (a *physicalLocal) CaptureProfileAuthority(ctx context.Context, c domain.DetectedClient) (domain.ProfileAuthority, error) {
	a.captures++
	return profileauthority.Capture(ctx, c.ConfigRoot)
}
func (a *physicalLocal) RevalidateProfileAuthority(ctx context.Context, _ domain.ClientID, p domain.ProfileAuthority) error {
	return profileauthority.Revalidate(ctx, p)
}
func actualPhysicalLocal(t *testing.T) (string, *vscode.LocalAdapter, installer.Request) {
	t.Helper()
	root := physicalTempDir(t)
	pkg := filepath.Join(root, "TEST-package")
	profile := filepath.Join(root, "TEST-profile")
	for _, dir := range []string{profile, filepath.Dir(filepath.Join(pkg, filepath.FromSlash(vscodelocalhooks.PluginPath)))} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	settings := filepath.Join(profile, "settings.json")
	if err := os.WriteFile(settings, []byte(`{"foreign":17,"chat.pluginLocations":{"/TEST-foreign-disabled":false}}`), 0600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	target := vscodelocalhooks.Target{Shell: vscodelocalhooks.LinuxSH}
	specs := []vscodelocalhooks.Spec{{Event: vscodelocalhooks.Stop, Executable: executable, TimeoutSeconds: 5}}
	hooks, err := vscodelocalhooks.Render(target, specs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, filepath.FromSlash(vscodelocalhooks.PluginPath)), hooks, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"physical-local-test","version":"1.0.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	adapter, err := vscode.NewLocal(vscode.LocalConfig{ProfileSettingsPath: settings, QualifiedTuple: vscode.SourceQualifiedTESTTuple("linux"), TargetShell: target, NativeStop: true, HookSpecs: specs, DeclaredHookDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(hooks))})
	if err != nil {
		t.Fatal(err)
	}
	return root, adapter, installer.Request{Operation: installer.OpInstall, PackageRoot: pkg, ClientID: "vscode", ClientConfigRoot: profile, ClientExecutable: executable, InstallationID: "00000000-0000-4000-8000-000000000091", OperationID: "TEST-local"}
}

// Regression: changing a real historical NewLocal owner to an opted adapter
// recaptures its missing token and mutates profiles/state instead of refusing.
func TestPhysicalProfileHistoricalLocalCannotRecapture(t *testing.T) {
	root, adapter, req := actualPhysicalLocal(t)
	stateRoot := filepath.Join(root, "TEST-state")
	eng := localTestEngine(t, stateRoot, adapter)
	h, err := eng.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := h.Close(); err != nil {
			t.Errorf("close prepared operation: %v", err)
		}
	}()
	if _, err := eng.Apply(t.Context(), h, confirmedDecision()); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(req.ClientConfigRoot, "settings.json")
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(before, []byte(`"/TEST-foreign-disabled":false`)) {
		t.Fatal("historical NewLocal lost explicit false")
	}
	stateBefore, err := os.ReadFile(filepath.Join(stateRoot, "state-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	opted := &physicalLocal{LocalAdapter: adapter}
	eng = localTestEngine(t, stateRoot, opted)
	unexpected, prepareErr := eng.Prepare(t.Context(), req)
	if unexpected != nil {
		defer func() {
			if err := unexpected.Close(); err != nil {
				t.Errorf("close prepared operation: %v", err)
			}
		}()
	}
	if prepareErr == nil {
		t.Fatal("missing opted token was accepted")
	}
	after, err := os.ReadFile(settings)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("missing token changed actual profile")
	}
	stateAfter, err := os.ReadFile(filepath.Join(stateRoot, "state-v2.json"))
	if err != nil || !bytes.Equal(stateBefore, stateAfter) || opted.captures != 0 {
		t.Fatal("refusal saved state or recaptured owner")
	}
}

// Regression: missing opted Local profile permits a namespace effect.
func TestPhysicalProfileLocalBootstrapBeforePrepare(t *testing.T) {
	root, adapter, req := actualPhysicalLocal(t)
	opted := &physicalLocal{LocalAdapter: adapter}
	registry, err := clients.NewRegistry(opted)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := installer.New(installer.Config{StateRoot: filepath.Join(root, "TEST-state"), Registry: registry, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	// Missing input is independent of the verifier and must precede namespace creation.
	if err := os.RemoveAll(req.ClientConfigRoot); err != nil {
		t.Fatal(err)
	}
	unexpected, prepareErr := eng.Prepare(t.Context(), req)
	if unexpected != nil {
		defer func() {
			if err := unexpected.Close(); err != nil {
				t.Errorf("close prepared operation: %v", err)
			}
		}()
	}
	if !errors.Is(prepareErr, directoryidentity.ErrBootstrapRequired) {
		t.Fatalf("missing profile did not return bootstrap refusal: %v", prepareErr)
	}
	if _, err := os.Lstat(filepath.Join(root, "TEST-state")); !os.IsNotExist(err) {
		t.Fatal("entry refusal created owned namespace")
	}
}

func physicalTempDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
