package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Install a real saved binding, remove its owned projection, then exercise the
// ordinary install command's recorded-pin repair route. Cancellation at the
// real SDK stage port affects only repair and survives result-output failure.
func TestDeclaredInstallRepairKeepsSafeDiagnosticsAndBothErrors(t *testing.T) {
	for _, scenario := range []string{"human", "human broken result", "json"} {
		t.Run(scenario, func(t *testing.T) {
			client := fixtureClient(t, domain.ClientCodex)
			client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
			runner := &dependencyClaudeListing{root: client.ConfigRoot}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cause := context.Canceled
			failRepair := false
			repairStages := 0
			fixture, _ := newDependencyHarness(t, "", []domain.DetectedClient{client}, installer.Config{
				Runner: runner,
				Progress: func(event installer.ProgressEvent) {
					if failRepair && event.Phase == installer.ProgressStage {
						repairStages++
						cancel()
					}
				},
			})
			pin := seedDependencyCheckPackage(t, fixture, "demo", "")
			writeDependencyCheckAgreement(t, fixture, []environment.LockedPackage{pin}, "demo")
			fixture.app.SourceAcquirer, fixture.app.DirectoryClient = dependencyCheckForbiddenEffects{}, dependencyCheckForbiddenEffects{}
			if output, stderr, err := fixture.execute(false, "install", "--scope", "user", "--yes", "--format", "json"); err != nil {
				t.Fatalf("declared repair fixture did not install: %v\n%s\n%s", err, output, stderr)
			}
			binding := dependencyBinding(t, fixture)
			if err := os.RemoveAll(binding.TargetPath); err != nil {
				t.Fatal(err)
			}
			manifest, lock := fixture.app.dependencyPaths()
			beforeManifest, beforeLock := dependencyRead(t, manifest), dependencyRead(t, lock)
			fixture.app.InstallLogDir = diagnosticLogDirectory(t, "unwritable")
			failRepair = true
			var output, stderr bytes.Buffer
			fixture.app.Output, fixture.app.ErrorOutput = &output, &stderr
			format := scenario
			if scenario == "human broken result" {
				format = "human"
				fixture.app.Output = &diagnosticResultWriter{fail: func([]byte) bool { return repairStages > 0 }, err: io.ErrClosedPipe}
			}
			command := NewRoot(fixture.app)
			command.SetArgs([]string{"install", "--scope", "user", "--yes", "--format", format})
			err := command.ExecuteContext(ctx)
			if !errors.Is(err, cause) || repairStages != 1 {
				t.Fatalf("declared repair lost its stage cancellation or replayed it: stages=%d err=%v\n%s\n%s", repairStages, err, output.String(), stderr.String())
			}
			if scenario == "human broken result" {
				if !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, errInstallOutput) {
					t.Fatalf("declared repair discarded concurrent result-output failure: %v", err)
				}
			} else if format == "human" {
				assertUnavailableDiagnostic(t, output.String()+stderr.String(), "unwritable")
			} else {
				control := decodeOneDiagnosticEnvelope(t, output.String())
				if control["result"] != outputResultFailure || control["bindings"] == nil || control["results"] == nil {
					t.Fatalf("declared repair JSON lost its original native/binding failure fields: %v", control)
				}
			}
			if dependencyRead(t, manifest) != beforeManifest || dependencyRead(t, lock) != beforeLock {
				t.Fatal("diagnostic handling changed the recorded repair pin")
			}
		})
	}
}
