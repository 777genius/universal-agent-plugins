package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// The unavailable path is a genuine filesystem refusal, including on root and
// Windows test hosts; it does not depend on Unix permission-bit enforcement.
func diagnosticLogDirectory(t *testing.T, mode string) string {
	t.Helper()
	switch mode {
	case "disabled":
		return ""
	case "unwritable":
		block := filepath.Join(t.TempDir(), "private-log-parent")
		if err := os.WriteFile(block, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		return filepath.Join(block, "logs")
	case "saved":
		return filepath.Join(t.TempDir(), "logs")
	default:
		t.Fatalf("unknown diagnostic test mode: %s", mode)
		return ""
	}
}

func assertUnavailableDiagnostic(t *testing.T, output, mode string) {
	t.Helper()
	want := "Technical details were not recorded for this attempt."
	if mode == "unwritable" {
		want = "Technical details could not be saved."
	}
	if !strings.Contains(output, want) || !strings.Contains(output, "Failure codes:") {
		t.Fatalf("unavailable diagnostic lost safe guidance: %q", output)
	}
	for _, forbidden := range []string{"Technical details: ", "see the technical log", "hand the technical log"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("unavailable diagnostic advertises a missing file: %q", output)
		}
	}
}

func decodeOneDiagnosticEnvelope(t *testing.T, output string) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(output))
	var envelope map[string]any
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("invalid JSON envelope: %v\n%s", err, output)
	}
	var additional any
	if err := decoder.Decode(&additional); !errors.Is(err, io.EOF) {
		t.Fatalf("command emitted more than one envelope: %v\n%s", err, output)
	}
	if _, exists := envelope["log_file"]; exists {
		t.Fatalf("unavailable log acquired a JSON path: %v", envelope)
	}
	if strings.Contains(output, "Technical details could not") || strings.Contains(output, "Failure codes:") {
		t.Fatal("human diagnostic notice contaminated JSON")
	}
	return envelope
}

// This runs NewRoot with the real HTTPS publisher, immutable package store,
// saved intent and receipts. The only injected failure is the existing
// prepublication readiness seam; native inventory refuses mutation on replay.
func TestSkillsCommandUnavailableLogPreservesFailureAndDurableState(t *testing.T) {
	for _, mode := range []string{"disabled", "unwritable"} {
		for _, format := range []string{"human", "json"} {
			t.Run(mode+"/"+format, func(t *testing.T) {
				journey, request, runner := unchangedSkillsFixture(t)
				app := journey.cli.app
				private := "opaque-native-output-932817"
				cause := fmt.Errorf("%s: %w", private, fs.ErrPermission)
				guard := &unclosedSkillsStore{Store: journey.cli.store, failure: cause}
				app.StateStore = guard
				app.InstallLogDir = diagnosticLogDirectory(t, mode)
				before := savedSkillsDurableTree(t, app.DependenciesRoot)
				var output, stderr bytes.Buffer
				app.Output, app.ErrorOutput = &output, &stderr
				command := NewRoot(app)
				args := []string{"skills", "add", request.Selections[0].Website, "--skill", "pdf", "--target", "codex", "--no-ui", "--yes", "--scope", "user", "--format", format}
				command.SetArgs(args)
				err := command.ExecuteContext(t.Context())
				if !errors.Is(err, cause) || !errors.Is(err, fs.ErrPermission) || guard.checks != 1 {
					t.Fatalf("command lost its cause or replayed the failed apply: checks=%d err=%v", guard.checks, err)
				}
				if runner.mutations != 0 || !reflect.DeepEqual(before, savedSkillsDurableTree(t, app.DependenciesRoot)) {
					t.Fatal("diagnostic recording changed the installation")
				}
				if format == "json" {
					envelope := decodeOneDiagnosticEnvelope(t, output.String())
					if envelope["status"] != "failed" || envelope["result"] != outputResultFailure || envelope["readiness"] != "failed" {
						t.Fatalf("JSON hid the failed repeated install: %v", envelope)
					}
					items, ok := envelope["items"].([]any)
					if !ok || len(items) != 1 {
						t.Fatalf("JSON lost reviewed binding: %v", envelope)
					}
				} else {
					public := output.String() + stderr.String() + ErrorText(args, &stderr, err)
					assertUnavailableDiagnostic(t, public, mode)
					if strings.Contains(public, private) || !strings.Contains(public, "does not have permission") || !strings.Contains(public, "PERMISSION_DENIED") {
						t.Fatalf("command lost classified cause or exposed native detail: %q", public)
					}
				}
			})
		}
	}
}

// A real accepted dependency command reaches the installer facade once and
// fails its committed-binding handoff. Logs cannot change that transaction,
// its classification, the reviewed manifest/lock, or retry admission.
func diagnosticDependencyFixture(t *testing.T, cause error) (cliFixture, *cliRunOnlyRunner, *int) {
	t.Helper()
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	runner := &cliRunOnlyRunner{}
	commits := new(int)
	fixture, _ := newDependencyHarness(t, "", []domain.DetectedClient{client}, installer.Config{
		UserHome: t.TempDir(), Runner: runner,
		OnCommittedBinding: func(_ context.Context, _ installer.BindingFacts) error {
			*commits++
			return cause
		},
	})
	pin := seedDependencyCheckPackage(t, fixture, "demo", "")
	writeDependencyCheckAgreement(t, fixture, []environment.LockedPackage{pin}, "demo")
	fixture.app.SourceAcquirer = dependencyCheckForbiddenEffects{}
	fixture.app.DirectoryClient = dependencyCheckForbiddenEffects{}
	return fixture, runner, commits
}

func TestDependencyCommandUnavailableLogPreservesBindingOutcome(t *testing.T) {
	for _, mode := range []string{"disabled", "unwritable"} {
		for _, format := range []string{"human", "json"} {
			t.Run(mode+"/"+format, func(t *testing.T) {
				private := "opaque-dependency-provider-274192"
				cause := fmt.Errorf("%s: %w", private, installer.ErrTargetFactsUnavailable)
				fixture, runner, commits := diagnosticDependencyFixture(t, cause)
				fixture.app.InstallLogDir = diagnosticLogDirectory(t, mode)
				manifest, lock := fixture.app.dependencyPaths()
				beforeManifest, beforeLock := dependencyRead(t, manifest), dependencyRead(t, lock)
				output, stderr, err := fixture.execute(false, "install", "--scope", "user", "--yes", "--format", format)
				if !errors.Is(err, cause) || *commits != 1 || runner.calls != 0 {
					t.Fatalf("dependency outcome changed or replayed: commits=%d native=%d err=%v", *commits, runner.calls, err)
				}
				if dependencyRead(t, manifest) != beforeManifest || dependencyRead(t, lock) != beforeLock {
					t.Fatal("diagnostic failure changed reviewed intent")
				}
				if format == "json" {
					envelope := decodeOneDiagnosticEnvelope(t, output)
					if envelope["result"] != outputResultFailure {
						t.Fatalf("dependency JSON hid failure: %v", envelope)
					}
					bindings, ok := envelope["bindings"].([]any)
					if !ok || len(bindings) != 1 {
						t.Fatalf("dependency JSON lost binding: %v", envelope)
					}
					binding := bindings[0].(map[string]any)
					if binding["code"] != string(failureTargetUnavailable) || binding["status"] == "completed" {
						t.Fatalf("dependency binding invented completion: %v", binding)
					}
				} else {
					public := output + stderr + err.Error()
					assertUnavailableDiagnostic(t, public, mode)
					if strings.Contains(public, private) || !strings.Contains(public, "configuration or version could not be verified") {
						t.Fatalf("dependency fallback exposed provider detail or hid cause: %q", public)
					}
				}
			})
		}
	}
}

type diagnosticResultWriter struct {
	bytes.Buffer
	failures int
	fail     func([]byte) bool
	err      error
	short    bool
}

func (writer *diagnosticResultWriter) Write(body []byte) (int, error) {
	if writer.fail(body) {
		writer.failures++
		if writer.short {
			return 0, nil
		}
		return 0, writer.err
	}
	return writer.Buffer.Write(body)
}

func TestDependencyApplyOutputFailurePreservesPrimaryCauseWithoutRetry(t *testing.T) {
	for _, short := range []bool{false, true} {
		t.Run(fmt.Sprint("short=", short), func(t *testing.T) {
			cause := fmt.Errorf("opaque-primary-713927: %w", installer.ErrTargetFactsUnavailable)
			fixture, runner, commits := diagnosticDependencyFixture(t, cause)
			fixture.app.InstallLogDir = diagnosticLogDirectory(t, "unwritable")
			broken := &diagnosticResultWriter{fail: func(body []byte) bool { return bytes.Contains(body, []byte("Needs attention")) }, short: short, err: errors.New("opaque-result-writer-812937")}
			fixture.app.Output = broken
			var stderr bytes.Buffer
			fixture.app.ErrorOutput = &stderr
			command := NewRoot(fixture.app)
			command.SetArgs([]string{"install", "--scope", "user", "--yes", "--format", "human"})
			err := command.ExecuteContext(t.Context())
			outputCause := broken.err
			if short {
				outputCause = io.ErrShortWrite
			}
			if !errors.Is(err, cause) || !errors.Is(err, outputCause) || !errors.Is(err, errInstallOutput) || broken.failures != 1 || *commits != 1 || runner.calls != 0 {
				t.Fatalf("output failure hid the apply cause or replayed effects: writes=%d commits=%d native=%d err=%v", broken.failures, *commits, runner.calls, err)
			}
			if strings.Contains(err.Error(), "opaque-") {
				t.Fatalf("joined human error exposed arbitrary detail: %q", err.Error())
			}
		})
	}
}
