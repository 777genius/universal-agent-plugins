package statev2

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectStateCopyCannotTransferMachineAuthority(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	original := Store{ProjectRoot: a, Path: filepath.Join(a, ".uap", "state-v2.json")}
	installation := validInstallation("00000000-0000-4000-8000-000000000001", "src_project", "demo-000000000001")
	for key, binding := range installation.Clients {
		delete(installation.Clients, key)
		binding.Scope = string(domain.ScopeProject)
		binding.TargetLocator = filepath.Join(a, ".uap", "managed", "codex", binding.PhysicalArtifact)
		binding.NativeProfileRoot = filepath.Join(a, ".codex")
		binding.ClientBindingID = domain.ComputeClientBindingID(installation.InstallationID, binding.ClientID, binding.Scope, binding.TargetLocator)
		installation.Clients[binding.ClientBindingID] = binding
	}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}
	if err := original.Save(state); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(original.Path)
	if err != nil {
		t.Fatal(err)
	}
	copied := Store{ProjectRoot: b, Path: filepath.Join(b, ".uap", "state-v2.json")}
	if err := os.Mkdir(filepath.Dir(copied.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copied.Path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := copied.Load(); err == nil {
		t.Fatal("copied project state adopted original project's target authority")
	}
	if err := copied.Save(state); err == nil {
		t.Fatal("cross-project state save adopted original target authority")
	}
	after, err := os.ReadFile(original.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("original state changed: %v", err)
	}
}

func TestProjectStateRejectsUserReceiptsOnLoad(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, ".uap", "state-v2.json")
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{validInstallation("00000000-0000-4000-8000-000000000001", "src_user", "demo-000000000001")}}
	if err := (Store{Path: filename}).Save(state); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{Path: filename, ProjectRoot: root}).Load(); err == nil {
		t.Fatal("user receipts were adopted as project state")
	}
}
