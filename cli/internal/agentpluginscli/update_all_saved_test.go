package agentpluginscli

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
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

func TestUpdateAllSavedConfirmationFailureReportsUnattemptedBatch(t *testing.T) {
	for _, input := range []string{"n\n", ""} {
		t.Run(map[bool]string{true: "declined", false: "closed input"}[input != ""], func(t *testing.T) {
			j, _, _ := savedMixedUpdateJourney(t)
			preview, cleanup, err := dependencyPreviewApp(j.cli.app, &options{dryRun: true})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			preview.Terminal = true
			var output bytes.Buffer
			command := &cobra.Command{}
			command.SetContext(t.Context())
			command.SetIn(strings.NewReader(input))
			command.SetOut(&output)
			command.SetErr(io.Discard)
			session := updateAllSession{ctx: t.Context(), cmd: command, app: preview, opts: &options{format: "human", scope: "user"}}
			if _, err := session.load(); err != nil {
				t.Fatal(err)
			}
			defer session.cleanupPrepared()
			if err := session.preflightAll(); err != nil {
				t.Fatal(err)
			}
			before := savedCheckFiles(t, j.cli.root)
			if err := session.applyAll(); err == nil {
				t.Fatal("confirmation failure was accepted")
			}
			if session.result.Status != "apply_failed" || session.result.Failed != 1 || session.result.Updated != 0 || !strings.Contains(output.String(), "Update all: apply_failed") {
				t.Fatal(output.String(), session.result)
			}
			for _, row := range session.result.Installations {
				if row.Status != "not_attempted" {
					t.Fatal(row)
				}
			}
			output.Reset()
			session.opts.format = "json"
			if err := renderUpdateAll(command, session.opts, session.result); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), `"result":"failure"`) {
				t.Fatal(output.String())
			}
			if !reflect.DeepEqual(before, savedCheckFiles(t, j.cli.root)) {
				t.Fatal("confirmation failure changed persistent files")
			}
		})
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

func TestUpdateAllDoesNotCountPartiallyAttemptedSavedAliasAsUpdated(t *testing.T) {
	session := updateAllSession{result: updateAllResult{Installations: []updateAllInstallation{{Name: "shared", Saved: true, Status: "planned"}}}, saved: &preparedSavedUpdateAll{rows: map[string]int{"shared": 0}, request: dependencySyncRequest{prepared: []dependencyAction{{Alias: "shared", ClientID: "claude", Action: environment.ActionUpdate}, {Alias: "shared", ClientID: "codex", Action: environment.ActionUpdate}}}}}
	session.recordSavedBindings(dependencyApplicationResult{Published: true, Results: []installer.Result{{Outcome: installer.OutcomeCompleted, Mutated: true}}})
	if session.result.Updated != 0 || session.result.Failed != 1 || session.result.Installations[0].Status != "apply_failed" || !strings.Contains(session.result.Installations[0].Reason, "partially") {
		t.Fatal(session.result)
	}
}

func TestUpdateAllRechecksImperativeManagedDriftBeforeSavedPublication(t *testing.T) {
	j, _, _ := savedMixedUpdateJourney(t)
	preview, cleanup, err := dependencyPreviewApp(j.cli.app, &options{dryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	command := &cobra.Command{}
	command.SetContext(t.Context())
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	session := updateAllSession{ctx: t.Context(), cmd: command, app: preview, opts: &options{format: "json", scope: "user"}}
	if _, err := session.load(); err != nil {
		t.Fatal(err)
	}
	defer session.cleanupPrepared()
	if err := session.preflightAll(); err != nil {
		t.Fatal(err)
	}
	changed := false
	for _, installation := range j.state(t).Installations {
		if installation.DeclaredName != "imperative" {
			continue
		}
		for _, binding := range installation.Clients {
			if binding.ClientID != "claude" {
				continue
			}
			if binding.TargetLocator == "" {
				t.Fatal("missing bound path")
			}
			if err := os.RemoveAll(binding.TargetLocator); err != nil {
				t.Fatal(err)
			}
			changed = true
		}
	}
	if !changed {
		t.Fatal("imperative binding not selected")
	}

	before := savedCheckFiles(t, j.cli.root)
	if err := session.applyAll(); err == nil {
		t.Fatal("late managed drift was accepted")
	}
	after := savedCheckFiles(t, j.cli.root)
	// The existing process lock refreshes its coordination metadata even on a
	// refused operation; it is not installed state, intent, package store or data.
	coordination, _ := filepath.Rel(j.cli.root, j.cli.app.EnvironmentLockPath)
	delete(before, coordination)
	delete(after, coordination)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("saved graph or store changed before imperative drift refusal")
	}

}
