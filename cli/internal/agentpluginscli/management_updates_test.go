package agentpluginscli

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func updateCheckAliases(values ...string) *[]string { return &values }
func TestManagementExplicitCheckMatchesSavedCLIAndPassiveInspect(t *testing.T) {
	j, set := savedArchiveCheckJourney(t)
	backend := managementJourneyBackend(t, j)
	before := savedCheckFiles(t, j.cli.root)
	input := management.UpdateCheckRequest{All: true}
	current, err := backend.CheckUpdates(t.Context(), input)
	if err != nil || !current.Complete || current.Changed || len(current.Entries) != 1 || current.Entries[0].Status != management.UpdateCheckEntryStatusCurrent {
		t.Fatal(current, err)
	}
	set(managementArchiveBytes(t, 2), http.StatusOK)
	available, err := backend.CheckUpdates(t.Context(), management.UpdateCheckRequest{Aliases: updateCheckAliases(managementArchiveName)})
	if err != nil || !available.Complete || !available.Changed || available.Entries[0].Available.Version != "2.0.0" || available.Entries[0].Installed.Version != "1.0.0" {
		t.Fatal(available, err)
	}
	prepared := managementPrepare(t, backend, management.ChangeRequest{Action: management.Update, All: true})
	for _, action := range prepared.Review().Actions {
		if action.FromVersion != "1.0.0" || action.ToVersion != "2.0.0" {
			t.Fatal("review lost observed before/after versions", action)
		}
	}
	set(nil, http.StatusServiceUnavailable)
	passive, err := backend.Inspect(t.Context())
	if err != nil || len(passive.Entries) != 1 || passive.Entries[0].UpdateStatus != management.EnvironmentEntryUpdateStatusCheckUnavailable {
		t.Fatal(passive, err)
	}
	unavailable, err := backend.CheckUpdates(t.Context(), input)
	if err != nil || unavailable.Complete || unavailable.Changed || unavailable.Entries[0].Status != management.UpdateCheckEntryStatusCheckUnavailable || len(unavailable.Diagnostics) == 0 {
		t.Fatal(unavailable, err)
	}
	if !reflect.DeepEqual(before, savedCheckFiles(t, j.cli.root)) {
		t.Fatal("checks/prepare/inspect changed persistent files")
	}
}
func TestManagementCheckSelectionAndCancellation(t *testing.T) {
	j, _ := savedArchiveCheckJourney(t)
	backend := managementJourneyBackend(t, j)
	for _, input := range []management.UpdateCheckRequest{{}, {Aliases: updateCheckAliases("docs", "docs")}, {All: true, Aliases: updateCheckAliases(managementArchiveName)}, {Aliases: updateCheckAliases("missing")}} {
		if _, err := backend.CheckUpdates(t.Context(), input); err == nil {
			t.Fatal("invalid selection accepted", input)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := backend.CheckUpdates(ctx, management.UpdateCheckRequest{All: true}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
