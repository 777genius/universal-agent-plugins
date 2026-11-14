package statev2

import (
	"encoding/json"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnsafePersistedPhysicalIdentityStillFailsClosed(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state-v2.json")}
	installation := validInstallation("00000000-0000-4000-8000-000000000001", "src_one", "con.foo-000000000001")
	installation.DeclaredName = "con.foo"
	installation.Package.DeclaredName = "con.foo"
	for key, binding := range installation.Clients {
		binding.PhysicalArtifact = domain.ComputePhysicalArtifactID("con.foo", installation.InstallationID)
		installation.Clients[key] = binding
	}
	safe := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}
	if err := store.Save(safe); err != nil {
		t.Fatal("safe control failed", err)
	}
	for key, binding := range installation.Clients {
		binding.PhysicalArtifact = "con.foo-000000000001"
		installation.Clients[key] = binding
	}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}
	if err := store.Save(state); err == nil || !strings.Contains(err.Error(), "physical artifact") {
		t.Fatal("unsafe physical identity saved")
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "physical artifact") {
		t.Fatal("unsafe imported identity bypassed validation")
	}
	after, err := os.ReadFile(store.Path)
	if err != nil || string(after) != string(raw) {
		t.Fatal("unsafe state automatically mutated")
	}
}
