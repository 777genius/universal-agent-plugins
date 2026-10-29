package agentpluginscli

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func savedMixedUpdateJourney(t *testing.T) (savedJourney, func([]byte, int), string) {
	t.Helper()
	j, set := savedArchiveCheckJourney(t)
	j.cli.app.Lifecycle.Activator = &cliObservedActivator{outcome: domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid}}
	local := writeNamedCLIPlugin(t, "imperative", "1.0.0")
	j.run(t, "add", local, "--target", "claude", "--format", "json")
	writeNamedCLIPluginVersion(t, local, "imperative", "2.0.0")
	set(managementArchiveBytes(t, 2), http.StatusOK)
	return j, set, local
}
func TestUpdateAllIncludesSavedAndImperativeAfterOnePreflight(t *testing.T) {
	j, _, _ := savedMixedUpdateJourney(t)
	before := savedCheckFiles(t, j.cli.root)
	preview := j.run(t, "update", "--all", "--dry-run", "--format", "json")
	if !strings.Contains(preview, `"planned":2`) || strings.Count(preview, `"saved":true`) != 1 {
		t.Fatal(preview)
	}
	if !reflect.DeepEqual(before, savedCheckFiles(t, j.cli.root)) {
		t.Fatal("mixed preview changed persistent files")
	}
	output := j.run(t, "update", "--all", "--format", "json")
	if !strings.Contains(output, `"updated":2`) || !strings.Contains(output, `"status":"completed"`) {
		t.Fatal(output)
	}
	state := j.state(t)
	if len(state.Installations) != 2 {
		t.Fatal(state)
	}
	for _, installation := range state.Installations {
		if installation.Package.Version != "2.0.0" {
			t.Fatal(installation)
		}
	}
	_, lockPath := j.cli.app.dependencyPaths()
	lock := dependencyRead(t, lockPath)
	output = j.run(t, "update", "--all", "--format", "json")
	if !strings.Contains(output, `"updated":0`) || dependencyRead(t, lockPath) != lock {
		t.Fatal("repeat changed lock", output)
	}
}
func TestUpdateAllSavedCandidateFailureOrImperativeFailureDoesNotPublish(t *testing.T) {
	for _, sourceFailure := range []bool{false, true} {
		t.Run(map[bool]string{true: "saved", false: "imperative"}[sourceFailure], func(t *testing.T) {
			j, set, local := savedMixedUpdateJourney(t)
			if sourceFailure {
				set(nil, http.StatusServiceUnavailable)
			} else if err := os.RemoveAll(local); err != nil {
				t.Fatal(err)
			}
			before := savedCheckFiles(t, j.cli.root)
			output, _, err := j.cli.execute(false, "update", "--all", "--format", "json")
			if err == nil || !strings.Contains(output, `"status":"preflight_failed"`) {
				t.Fatal(output, err)
			}
			if !reflect.DeepEqual(before, savedCheckFiles(t, j.cli.root)) {
				t.Fatal("failed preflight changed persistent files")
			}
		})
	}
}
func TestUpdateAllPublishedSavedIntentReportsPartialAndUnattempted(t *testing.T) {
	j, _, _ := savedMixedUpdateJourney(t)
	original := publishEnvironmentFile
	t.Cleanup(func() { publishEnvironmentFile = original })
	_, lockPath := j.cli.app.dependencyPaths()
	publishEnvironmentFile = func(path string, body []byte) error {
		if err := original(path, body); err != nil {
			return err
		}
		if path == lockPath {
			return errors.New("injected post-publication failure")
		}
		return nil
	}
	output, _, err := j.cli.execute(false, "update", "--all", "--format", "json")
	if err == nil || !strings.Contains(output, `"status":"partial_failure"`) || !strings.Contains(output, `"status":"not_attempted"`) || !strings.Contains(output, "saved intent was published") {
		t.Fatal(output, err)
	}
	for _, installation := range j.state(t).Installations {
		if installation.Package.Version != "1.0.0" {
			t.Fatal("binding changed after publication failure", installation)
		}
	}
}
func TestSavedRegistryNoLaterReleaseRechecksRecordedEligibility(t *testing.T) {
	for _, status := range []string{"revoked", "suspended", "blocked"} {
		t.Run(status, func(t *testing.T) {
			directory := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientClaude}, []domain.ClientID{domain.ClientClaude})
			directory.retainV1Only()
			client := fixtureClient(t, domain.ClientClaude)
			client.ExecutablePath = filepath.Join(t.TempDir(), "claude")
			fixture, _ := newDependencyHarness(t, t.TempDir(), []domain.DetectedClient{client}, installer.Config{Runner: &dependencyClaudeListing{root: client.ConfigRoot}})
			fixture.app.DirectoryClient = directory.directory
			fixture.app.SourceAcquirer = directory.acquirer
			j := savedJourney{cli: fixture}
			backend := managementJourneyBackend(t, j)
			managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Install, Source: "rollout-demo", Alias: "docs", TargetIds: []string{"profile-claude"}}))
			distribution := &directory.directory.bundle.Snapshot.Distributions[0]
			switch status {
			case "revoked":
				distribution.ReleasePolicies[0].Status = domain.ReleaseRevoked
			case "suspended":
				distribution.Status = domain.DistributionSuspended
			case "blocked":
				directory.directory.bundle.Snapshot.Evidence[0].Outcome = "failed"
			}
			snapshot, err := snapshotEnvironmentContext(t.Context(), fixture.app, false)
			if err != nil {
				t.Fatal(err)
			}
			before := savedCheckFiles(t, j.cli.root)
			check, err := checkSavedUpdates(t.Context(), fixture.app, snapshot, []string{"docs"})
			if err != nil || check.Complete || len(check.Entries) != 1 || check.Entries[0].Status == "current" {
				t.Fatal(check, err)
			}
			if !reflect.DeepEqual(before, savedCheckFiles(t, j.cli.root)) {
				t.Fatal("ineligible fallback changed persistent files")
			}
		})
	}
}
