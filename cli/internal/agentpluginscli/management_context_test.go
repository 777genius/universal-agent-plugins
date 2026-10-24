package agentpluginscli

import (
	"errors"
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

func TestManagementMissingNativeExecutableIsUnavailableBeforeSourceAcquisition(t *testing.T) {
	repo, _ := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	base := managementJourneyBackend(t, j)
	for _, client := range []string{"codex", "opencode"} {
		t.Run(client, func(t *testing.T) {
			cfg := base.config
			cfg.Targets = []management.StartupTarget{{Id: "selected", Client: client, ConfigRoot: t.TempDir()}}
			backend, err := NewManagementBackend(j.cli.app, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer backend.Close()
			session, err := backend.Session(t.Context())
			if err != nil || len(session.Targets) != 1 || session.Targets[0].Available || session.Targets[0].Reason == "" {
				t.Fatalf("missing executable advertised as available: %+v, %v", session, err)
			}
			prepared, err := backend.Prepare(t.Context(), management.ChangeRequest{Action: management.Install, Source: "github:example/never-acquire", TargetIds: []string{"selected"}}, "unavailable-runtime")
			var refusal *management.Refusal
			if prepared != nil || !errors.As(err, &refusal) || refusal.Code != "TARGET_UNAVAILABLE" {
				t.Fatalf("missing runtime did not produce an actionable pre-acquisition refusal: %T, %v", prepared, err)
			}
		})
	}
}
