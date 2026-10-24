package statev2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func projectRegistryState(root, native string) domain.StateFileV2 {
	installation := validInstallation("00000000-0000-4000-8000-000000000001", "src_project", "demo-000000000001")
	for key, binding := range installation.Clients {
		delete(installation.Clients, key)
		binding.Scope = string(domain.ScopeProject)
		binding.TargetLocator = filepath.Join(root, ".uap", "managed", binding.PhysicalArtifact)
		binding.NativeProfileRoot = native
		binding.ClientBindingID = domain.ComputeClientBindingID(installation.InstallationID, binding.ClientID, binding.Scope, binding.TargetLocator)
		installation.Clients[binding.ClientBindingID] = binding
	}
	return domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}
}

func TestProjectStateAcceptsExactRootNativeRegistry(t *testing.T) {
	root := t.TempDir()
	store := Store{ProjectRoot: root, Path: filepath.Join(root, ".uap", "state-v2.json")}
	if err := store.Save(projectRegistryState(root, root)); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range loaded.Installations[0].Clients {
		if binding.NativeProfileRoot != root || binding.TargetLocator == root {
			t.Fatalf("root-native registry lost its distinct artifact locator: %+v", binding)
		}
	}
}

func TestProjectStateRootNativeRegistryDoesNotBroadenOtherLocators(t *testing.T) {
	for _, kind := range []string{"different project", "relative native root", "root artifact"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			state := projectRegistryState(root, root)
			installation := &state.Installations[0]
			for key, binding := range installation.Clients {
				delete(installation.Clients, key)
				switch kind {
				case "different project":
					binding.NativeProfileRoot = t.TempDir()
				case "relative native root":
					binding.NativeProfileRoot = "."
				case "root artifact":
					binding.TargetLocator = root
				}
				binding.ClientBindingID = domain.ComputeClientBindingID(installation.InstallationID, binding.ClientID, binding.Scope, binding.TargetLocator)
				installation.Clients[binding.ClientBindingID] = binding
			}
			assertProjectRegistryRejected(t, root, state)
		})
	}
}

func assertProjectRegistryRejected(t *testing.T, root string, state domain.StateFileV2) {
	t.Helper()
	store := Store{ProjectRoot: root, Path: filepath.Join(root, ".uap", "state-v2.json")}
	if err := store.Save(state); err == nil {
		t.Fatal("unsafe project registry/target was persisted")
	}
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(store.Path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("unsafe project registry/target was adopted on load")
	}
}

func TestProjectStateRootNativeRegistryRejectsSymlinkAuthority(t *testing.T) {
	parent, actual := t.TempDir(), t.TempDir()
	alias := filepath.Join(parent, "project-alias")
	if err := os.Symlink(actual, alias); err != nil {
		t.Skipf("host cannot create disposable symlink: %v", err)
	}
	assertProjectRegistryRejected(t, alias, projectRegistryState(alias, alias))
}
