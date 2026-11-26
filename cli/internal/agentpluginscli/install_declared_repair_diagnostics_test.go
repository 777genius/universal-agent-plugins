package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Install a real saved binding, remove its owned projection, then exercise the
// ordinary install command's recorded-pin repair route. The host handoff fails
// only on repair; its original typed cause must survive result-output failure.
func TestDeclaredInstallRepairKeepsSafeDiagnosticsAndBothErrors(t *testing.T) {
	for _, scenario := range []string{"human", "human broken result", "json"} {
		t.Run(scenario, func(t *testing.T) {
			client := fixtureClient(t, domain.ClientCodex)
			client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
			runner := &dependencyClaudeListing{root: client.ConfigRoot}
			marker := "opaque-declared-repair-provider-172938"
			cause := fmt.Errorf("%s: %w", marker, os.ErrPermission)
			failRepair := false
			repairHandoffs := 0
			fixture, _ := newDependencyHarness(t, "", []domain.DetectedClient{client}, installer.Config{
				Runner: runner,
				OnCommittedBinding: func(context.Context, installer.BindingFacts) error {
					if failRepair {
						repairHandoffs++
						return cause
					}
					return nil
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
				fixture.app.Output = &diagnosticResultWriter{fail: func([]byte) bool { return repairHandoffs > 0 }, err: io.ErrClosedPipe}
			}
			command := NewRoot(fixture.app)
			command.SetArgs([]string{"install", "--scope", "user", "--yes", "--format", format})
			err := command.ExecuteContext(t.Context())
			if !errors.Is(err, cause) || !errors.Is(err, os.ErrPermission) || repairHandoffs != 1 {
				t.Fatalf("declared repair lost its handoff failure or replayed it: handoffs=%d err=%v\n%s\n%s", repairHandoffs, err, output.String(), stderr.String())
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
			if format == "human" && strings.Contains(output.String()+stderr.String()+err.Error(), marker) {
				t.Fatal("declared repair exposed the original provider chain in human output")
			}
			if dependencyRead(t, manifest) != beforeManifest || dependencyRead(t, lock) != beforeLock {
				t.Fatal("diagnostic handling changed the recorded repair pin")
			}
		})
	}
}
