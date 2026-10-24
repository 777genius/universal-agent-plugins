package agentpluginscli

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func TestManagementIdentitySeparatesDurableEnvironmentFromCapture(t *testing.T) {
	repo, _ := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	backend := managementJourneyBackend(t, j)
	base := backend.config
	renamed := base
	renamed.Targets = append([]management.StartupTarget{}, base.Targets...)
	renamed.Targets[0].Id = "renamed-client-label"
	renamed.Targets[0].Executable = filepath.Join(t.TempDir(), "upgraded-client")
	renamed.DependenciesRoot = t.TempDir()
	changed, err := NormalizeManagementStartup(renamed)
	if err != nil {
		t.Fatal(err)
	}
	if changed.ContextId == base.ContextId {
		t.Fatal("captured authority revision was not changed")
	}
	if ManagementEnvironmentID(changed) != ManagementEnvironmentID(base) {
		t.Fatal("relabel or relocated intent hid durable native history")
	}
	changed.StateRoot = t.TempDir()
	changed, err = NormalizeManagementStartup(changed)
	if err != nil {
		t.Fatal(err)
	}
	if ManagementEnvironmentID(changed) == ManagementEnvironmentID(base) {
		t.Fatal("different native state shared a ledger environment")
	}
	defaults := base
	defaults.StateRoot, defaults.DependenciesRoot = "", ""
	same, err := NewManagementBackend(j.cli.app, defaults)
	if err != nil {
		t.Fatal(err)
	}
	defer same.Close()
	if same.ContextID() != backend.ContextID() {
		t.Fatal("composed defaults changed captured identity")
	}
}
