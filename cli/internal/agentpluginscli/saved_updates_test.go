package agentpluginscli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func savedArchiveCheckJourney(t *testing.T) (savedJourney, func([]byte, int)) {
	t.Helper()
	var mu sync.Mutex
	body, status := managementArchiveBytes(t, 1), http.StatusOK
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	client := fixtureClient(t, domain.ClientClaude)
	client.ExecutablePath = filepath.Join(t.TempDir(), "claude")
	fixture, _ := newDependencyHarness(t, t.TempDir(), []domain.DetectedClient{client}, installer.Config{Runner: &dependencyClaudeListing{root: client.ConfigRoot}})
	fixture.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
	j := savedJourney{cli: fixture}
	backend := managementJourneyBackend(t, j)
	managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Install, Source: server.URL + "/fixture.tar.gz", Alias: managementArchiveName, TargetIds: []string{"profile-claude"}}))
	return j, func(next []byte, code int) { mu.Lock(); defer mu.Unlock(); body, status = next, code }
}
func savedCheckFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, _ := filepath.Rel(root, path)
			out[relative] = string(body)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestSavedFreshnessChecksIdentityWithoutPersistentEffects(t *testing.T) {
	j, set := savedArchiveCheckJourney(t)
	snapshot, err := snapshotEnvironmentContext(t.Context(), j.cli.app, false)
	if err != nil {
		t.Fatal(err)
	}
	before := savedCheckFiles(t, j.cli.root)
	check, err := checkSavedUpdates(t.Context(), j.cli.app, snapshot, []string{managementArchiveName})
	if err != nil || !check.Complete || check.Changed || len(check.Entries) != 1 || check.Entries[0].Status != "current" {
		t.Fatal(check, err)
	}
	next := managementArchiveBytes(t, 2)
	set(next, http.StatusOK)
	check, err = checkSavedUpdates(t.Context(), j.cli.app, snapshot, []string{managementArchiveName})
	if err != nil || !check.Complete || !check.Changed || check.Entries[0].Status != "available" || check.Entries[0].Available.PackageVersion != "2.0.0" {
		t.Fatal(check, err)
	}
	set(nil, http.StatusServiceUnavailable)
	check, err = checkSavedUpdates(t.Context(), j.cli.app, snapshot, []string{managementArchiveName})
	if err != nil || check.Complete || check.Changed || check.Entries[0].Status != "check_unavailable" {
		t.Fatal(check, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := checkSavedUpdates(ctx, j.cli.app, snapshot, []string{managementArchiveName}); err != context.Canceled {
		t.Fatal(err)
	}
	if after := savedCheckFiles(t, j.cli.root); !reflect.DeepEqual(before, after) {
		t.Fatal("read-only checks changed persistent files")
	}
}
func TestOutdatedIncludesSavedArchiveInsteadOfUnmanaged(t *testing.T) {
	j, set := savedArchiveCheckJourney(t)
	set(managementArchiveBytes(t, 2), http.StatusOK)
	output := j.run(t, "outdated", managementArchiveName, "--format", "json")
	for _, want := range []string{`"outdated":1`, `"alias":"` + managementArchiveName + `"`, `"complete":true`, `"changed":true`, `"package_version":"2.0.0"`} {
		if !strings.Contains(output, want) {
			t.Fatal(want, output)
		}
	}
	if strings.Contains(output, `"status":"unmanaged"`) {
		t.Fatal(output)
	}
}
