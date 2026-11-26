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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type declaredLateDetector struct {
	ports.ClientDetector
	failure error
}

func (detector *declaredLateDetector) Detect(ctx context.Context) ([]domain.DetectedClient, error) {
	if detector.failure != nil {
		return nil, detector.failure
	}
	return detector.ClientDetector.Detect(ctx)
}

// Repair succeeds before the real detector or committed-binding port fails in
// the later convergence phase. Both phases must produce one truthful envelope.
func TestDeclaredRepairAndConvergenceEmitOneJSONResult(t *testing.T) {
	for _, scenario := range []string{"noop", "install", "later preflight failure", "later apply failure", "partial output failure", "short output"} {
		t.Run(scenario, func(t *testing.T) {
			client := fixtureClient(t, domain.ClientCodex)
			client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
			runner := &dependencyClaudeListing{root: client.ConfigRoot}
			armed, failSync := false, false
			repairs, syncHandoffs := 0, 0
			cause := installer.ErrTargetFactsUnavailable
			var detector *declaredLateDetector
			fixture, _ := newDependencyHarness(t, "", []domain.DetectedClient{client}, installer.Config{
				Runner: runner,
				Progress: func(event installer.ProgressEvent) {
					if !armed || event.Phase != installer.ProgressComplete {
						return
					}
					armed = false
					repairs++
					if scenario == "later preflight failure" {
						detector.failure = cause
					}
					failSync = scenario == "later apply failure"
				},
				OnCommittedBinding: func(context.Context, installer.BindingFacts) error {
					if failSync {
						syncHandoffs++
						return cause
					}
					return nil
				},
			})
			detector = &declaredLateDetector{ClientDetector: fixture.app.Detector}
			fixture.app.Detector = detector
			pin := seedDependencyCheckPackage(t, fixture, "demo", "")
			writeDependencyCheckAgreement(t, fixture, []environment.LockedPackage{pin}, "demo")
			fixture.app.SourceAcquirer, fixture.app.DirectoryClient = dependencyCheckForbiddenEffects{}, dependencyCheckForbiddenEffects{}
			if output, stderr, err := fixture.execute(false, "install", "--scope", "user", "--yes", "--format", "json"); err != nil {
				t.Fatalf("seed failed: %v\n%s\n%s", err, output, stderr)
			}
			binding := dependencyBinding(t, fixture)
			if err := os.RemoveAll(binding.TargetPath); err != nil {
				t.Fatal(err)
			}
			if scenario == "install" || scenario == "later apply failure" {
				later := seedDependencyCheckPackage(t, fixture, "later", "")
				writeDependencyCheckAgreement(t, fixture, []environment.LockedPackage{pin, later}, "demo", "later")
			}
			var output, stderr bytes.Buffer
			fixture.app.Output, fixture.app.ErrorOutput = &output, &stderr
			partial := &declaredPartialWriter{short: scenario == "short output"}
			if scenario == "partial output failure" || partial.short {
				fixture.app.Output = partial
			}
			command := NewRoot(fixture.app)
			command.SetArgs([]string{"install", "--scope", "user", "--yes", "--format", "json"})
			armed = true
			err := command.ExecuteContext(t.Context())
			if repairs != 1 {
				t.Fatalf("real successful repair boundary was not reached exactly once: repairs=%d err=%v", repairs, err)
			}
			if _, statErr := os.Stat(binding.TargetPath); statErr != nil {
				t.Fatalf("successful repair side effect disappeared: %v", statErr)
			}
			if scenario == "partial output failure" || partial.short {
				expected := io.ErrClosedPipe
				if partial.short {
					expected = io.ErrShortWrite
				}
				if !errors.Is(err, expected) || !errors.Is(err, errInstallOutput) || partial.calls != 1 || ShouldWriteAgentFailure(err) {
					t.Fatalf("partial JSON output triggered another document or lost its cause: calls=%d err=%v", partial.calls, err)
				}
				return
			}
			failed := scenario == "later preflight failure" || scenario == "later apply failure"
			if failed != (err != nil) || failed && !errors.Is(err, cause) {
				t.Fatalf("later failure lost typed cause: %v\n%s", err, output.String())
			}
			if failed && ShouldWriteAgentFailure(err) {
				t.Fatal("the process entrypoint would append an early-error JSON document")
			}
			control := decodeOneDiagnosticEnvelope(t, output.String())
			results, ok := control["results"].([]any)
			if !ok || len(results) == 0 || results[0].(map[string]any)["operation"] != "repair" {
				t.Fatalf("final JSON discarded the committed repair result: %v", control)
			}
			actions, ok := control["actions"].([]any)
			if !ok || len(actions) == 0 || actions[0].(map[string]any)["action"] != "repair" {
				t.Fatalf("final JSON discarded the applied repair action: %v", control)
			}
			if failed && (control["result"] != outputResultFailure || control["outcome"] != "partial" || control["error"] == nil) {
				t.Fatalf("earlier effects made a later failure look successful: %v", control)
			}
			if scenario == "later apply failure" && (syncHandoffs != 1 || len(results) < 2) {
				t.Fatalf("late apply failed outside the real handoff or hid its result: handoffs=%d result=%v", syncHandoffs, control)
			}
			if scenario == "install" && len(results) < 2 {
				t.Fatal("successful convergence skipped the still-missing declared package")
			}
			if scenario == "noop" {
				// Reusing the command must reset its attempt-local presentation.
				output.Reset()
				if err := command.ExecuteContext(t.Context()); err != nil {
					t.Fatal(err)
				}
				repeated := decodeOneDiagnosticEnvelope(t, output.String())
				if rows := repeated["results"].([]any); len(rows) != 0 {
					t.Fatalf("the next command replayed earlier repair presentation: %v", repeated)
				}
			}
		})
	}
}

type declaredPartialWriter struct {
	bytes.Buffer
	calls int
	short bool
}

func (writer *declaredPartialWriter) Write(body []byte) (int, error) {
	writer.calls++
	count := min(12, len(body))
	_, _ = writer.Buffer.Write(body[:count])
	if writer.short {
		return count, nil
	}
	return count, io.ErrClosedPipe
}

func TestDeclaredRepairFinalizationPreventsEntrypointDoubleFailure(t *testing.T) {
	cmd, output, _ := retryTestCommand(t)
	opts := &options{format: "json"}
	retainDeclaredRepairPresentation(opts, []dependencyAction{{Alias: "demo", ClientID: "codex", Action: "repair"}}, []environment.BindingOutcome{{Binding: dependencyAction{Alias: "demo", ClientID: "codex"}, Status: environment.BindingApplied, Result: &installer.Result{Operation: installer.OpRepair, Outcome: installer.OutcomeCompleted}}})
	cause := CLIUsageError{Code: "LATE_USAGE", Message: "The remaining plan cannot be applied."}
	err := finishDeclaredRepairPresentation(cmd, opts, cause)
	if !errors.Is(err, cause) || ShouldWriteAgentFailure(err) {
		t.Fatalf("reported late refusal remained eligible for another JSON envelope: %v", err)
	}
	decodeOneDiagnosticEnvelope(t, output.String())
	before := output.String()
	if again := finishDeclaredRepairPresentation(cmd, opts, err); !errors.Is(again, cause) || output.String() != before {
		t.Fatal("finalizing an already reported repair emitted another document")
	}
}
