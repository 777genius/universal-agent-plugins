package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestUpdateAllAppliesOnlyAfterCompletePreflight(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	alpha := writeNamedCLIPlugin(t, "alpha", "1.0.0")
	if _, _, err := fixture.execute(false, "add", alpha, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	writeNamedCLIPluginVersion(t, alpha, "alpha", "2.0.0")

	stdout, _, err := fixture.execute(false, "update", "--all", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"batch":true`, `"status":"completed"`, `"planned":1`, `"updated":1`, `"name":"alpha"`, `"status":"updated"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("update --all omitted %q:\n%s", want, stdout)
		}
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || state.Installations[0].Package.Version != "2.0.0" {
		t.Fatalf("update --all did not apply prepared package: %+v", state.Installations)
	}
}

func TestUpdateAllPreflightFailureProducesZeroMutation(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	alpha := writeNamedCLIPlugin(t, "alpha", "1.0.0")
	beta := writeNamedCLIPlugin(t, "beta", "1.0.0")
	if _, _, err := fixture.execute(false, "add", alpha, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "add", beta, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	writeNamedCLIPluginVersion(t, alpha, "alpha", "2.0.0")
	if err := os.RemoveAll(beta); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}

	stdout, _, err := fixture.execute(false, "update", "--all", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "preflight failed; no installation was changed") {
		t.Fatalf("update --all error = %v, output=%s", err, stdout)
	}
	for _, want := range []string{`"status":"preflight_failed"`, `"planned":1`, `"updated":0`, `"failed":1`, `"name":"alpha"`, `"name":"beta"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("failed update --all omitted %q:\n%s", want, stdout)
		}
	}
	after, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	if string(beforeJSON) != string(afterJSON) {
		t.Fatalf("failed batch mutated state\nbefore=%s\nafter=%s", beforeJSON, afterJSON)
	}
	for _, installation := range after.Installations {
		if installation.Package.Version != "1.0.0" {
			t.Fatalf("failed batch updated %s before all preflights passed", installation.DeclaredName)
		}
	}
}

func TestUpdateAllDryRunAndTargetGuardRemainNonInteractive(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeNamedCLIPlugin(t, "alpha", "1.0.0")
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	writeNamedCLIPluginVersion(t, plugin, "alpha", "2.0.0")
	stdout, _, err := fixture.execute(false, "update", "--all", "--dry-run", "--format", "json")
	if err != nil || !strings.Contains(stdout, `"dry_run":true`) || !strings.Contains(stdout, `"status":"planned"`) {
		t.Fatalf("update --all dry run err=%v output=%s", err, stdout)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Installations[0].Package.Version != "1.0.0" {
		t.Fatalf("dry run mutated installation: %+v", state.Installations[0])
	}
	if _, _, err := fixture.execute(false, "update", "--all", "--target", "cursor"); err == nil || !strings.Contains(err.Error(), "keeps its recorded targets") {
		t.Fatalf("update --all accepted an overriding target: %v", err)
	}
}

func TestUpdateAllMarksRemainingPreparedItemsNotAttempted(t *testing.T) {
	result := updateAllResult{Installations: []updateAllInstallation{
		{Name: "alpha", Status: "apply_failed"},
		{Name: "beta", Status: "planned"},
		{Name: "gamma", Status: "planned"},
	}}
	prepared := []preparedUpdateAllItem{{resultIndex: 0}, {resultIndex: 1}, {resultIndex: 2}}
	markUnattemptedUpdateAll(&result, prepared, 1)
	for _, index := range []int{1, 2} {
		if result.Installations[index].Status != "not_attempted" || result.Installations[index].Reason == "" {
			t.Fatalf("remaining installation %d = %+v", index, result.Installations[index])
		}
	}
}

func writeNamedCLIPlugin(t *testing.T, name, version string) string {
	t.Helper()
	root := t.TempDir()
	writeNamedCLIPluginVersion(t, root, name, version)
	return root
}

func writeNamedCLIPluginVersion(t *testing.T, root, name, version string) {
	t.Helper()
	body := `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "` + name + `",
  "version": "` + version + `",
  "description": "Batch update fixture"
}`
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
