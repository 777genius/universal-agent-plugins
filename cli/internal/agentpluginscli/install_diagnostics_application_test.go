package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type diagnosticStageFailure struct {
	ports.PackageStager
	cause error
	calls int
}

func (stager *diagnosticStageFailure) Stage(context.Context, domain.PackageEnvelope, domain.DeliveryPlan, string, domain.CompatibilityHints) (domain.StagedDelivery, error) {
	stager.calls++
	return domain.StagedDelivery{}, stager.cause
}

// Use the actual single-add command application owner, with ordinary source
// loading and planning. The stager fails only after the reviewed apply begins.
func TestSingleAddApplicationUnavailableLogPreservesCauseAndEffects(t *testing.T) {
	for _, mode := range []string{"disabled", "unwritable"} {
		for _, format := range []string{"human", "json"} {
			t.Run(mode+"/"+format, func(t *testing.T) {
				client := fixtureClient(t, domain.ClientCodex)
				fixture := newCLIFixture(t, []domain.DetectedClient{client})
				cause := fmt.Errorf("opaque-single-provider-731254: %w", os.ErrPermission)
				stager := &diagnosticStageFailure{PackageStager: fixture.app.Lifecycle.Stager, cause: cause}
				fixture.app.Lifecycle.Stager = stager
				fixture.app.InstallLogDir = diagnosticLogDirectory(t, mode)
				loaded, err := fixture.app.loadPackage(t.Context(), writeCLIPlugin(t))
				if err != nil {
					t.Fatal(err)
				}
				if loaded.cleanup != nil {
					defer func() { _ = loaded.cleanup() }()
				}
				cmd, output, stderr := retryTestCommand(t)
				fixture.app.Output, fixture.app.ErrorOutput = output, stderr
				opts := &options{format: format, scope: "user", target: "codex", yes: true, noSave: true}
				err = runAddLoaded(t.Context(), cmd, fixture.app, opts, loaded, false, false, []domain.DetectedClient{client}, false)
				if !errors.Is(err, cause) || stager.calls != 1 || len(installedProjection(t, fixture)) != 0 {
					t.Fatalf("single-add failure was changed or replayed: stages=%d err=%v", stager.calls, err)
				}
				if opts.installResult == nil || len(opts.installResult.failedAgents()) != 1 || opts.installResult.LogFile != "" {
					t.Fatalf("single add lost its failed agent: %+v", opts.installResult)
				}
				if format == "human" {
					public := output.String() + stderr.String() + err.Error()
					assertUnavailableDiagnostic(t, public, mode)
					if strings.Contains(public, "opaque-single-provider") || !strings.Contains(public, "does not have permission") {
						t.Fatalf("single-add public fallback leaked provider detail: %q", public)
					}
				} else {
					decodeOneDiagnosticEnvelope(t, output.String())
					if strings.Contains(stderr.String(), "Technical details") || strings.Contains(stderr.String(), "Failure codes:") {
						t.Fatalf("single-add JSON gained human diagnostics: %q", stderr.String())
					}
				}
			})
		}
	}
}

func TestSingleAddResultWriteFailureKeepsApplyCause(t *testing.T) {
	client := fixtureClient(t, domain.ClientCodex)
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	cause := fmt.Errorf("opaque-primary-single-182736: %w", os.ErrPermission)
	stager := &diagnosticStageFailure{PackageStager: fixture.app.Lifecycle.Stager, cause: cause}
	fixture.app.Lifecycle.Stager = stager
	loaded, err := fixture.app.loadPackage(t.Context(), writeCLIPlugin(t))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.cleanup != nil {
		defer func() { _ = loaded.cleanup() }()
	}
	writerFailure := errors.New("opaque-single-writer-736182")
	broken := &diagnosticResultWriter{fail: func([]byte) bool { return stager.calls > 0 }, err: writerFailure}
	cmd, _, stderr := retryTestCommand(t)
	cmd.SetOut(broken)
	fixture.app.Output, fixture.app.ErrorOutput = broken, stderr
	opts := &options{format: "human", scope: "user", target: "codex", yes: true, noSave: true}
	err = runAddLoaded(t.Context(), cmd, fixture.app, opts, loaded, false, false, []domain.DetectedClient{client}, false)
	if !errors.Is(err, cause) || !errors.Is(err, writerFailure) || !errors.Is(err, errInstallOutput) || broken.failures == 0 || stager.calls != 1 {
		t.Fatalf("single-add failed result output lost a cause or replayed: stages=%d writes=%d err=%v", stager.calls, broken.failures, err)
	}
	if strings.Contains(err.Error(), "opaque-") {
		t.Fatalf("joined single-add error exposed arbitrary detail: %q", err.Error())
	}
}

func TestGroupCommandUnavailableLogKeepsPreparationFailure(t *testing.T) {
	for _, mode := range []string{"disabled", "unwritable"} {
		for _, format := range []string{"human", "json"} {
			t.Run(mode+"/"+format, func(t *testing.T) {
				fixture := newIsolationFixture(t, nil)
				cause := fmt.Errorf("opaque-group-provider-219837: %w", syscall.ENOSPC)
				stager := &diagnosticStageFailure{PackageStager: fixture.app.Lifecycle.Stager, cause: cause}
				fixture.app.Lifecycle.Stager = stager
				fixture.app.InstallLogDir = diagnosticLogDirectory(t, mode)
				output, stderr, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor", "--yes", "--format", format)
				if !errors.Is(err, cause) || !errors.Is(err, syscall.ENOSPC) || stager.calls != 1 || len(installedProjection(t, fixture)) != 0 {
					t.Fatalf("group preparation failure changed or replayed: stages=%d err=%v", stager.calls, err)
				}
				if format == "human" {
					public := output + stderr + err.Error()
					assertUnavailableDiagnostic(t, public, mode)
					if strings.Contains(public, "opaque-group-provider") || !strings.Contains(public, "not enough free disk space") {
						t.Fatalf("group fallback exposed provider detail or lost typed cause: %q", public)
					}
				} else {
					decodeOneDiagnosticEnvelope(t, output)
					if strings.Contains(stderr, "Technical details") || strings.Contains(stderr, "Failure codes:") {
						t.Fatalf("group JSON gained a human notice: %q", stderr)
					}
				}
			})
		}
	}
}

func TestGroupFailedResultOutputRetainsStageAndWriterCauses(t *testing.T) {
	fixture := newIsolationFixture(t, nil)
	cause := fmt.Errorf("opaque-group-stage-291837: %w", syscall.ENOSPC)
	stager := &diagnosticStageFailure{PackageStager: fixture.app.Lifecycle.Stager, cause: cause}
	fixture.app.Lifecycle.Stager = stager
	writerFailure := errors.New("opaque-group-writer-192837")
	broken := &diagnosticResultWriter{fail: func([]byte) bool { return stager.calls > 0 }, err: writerFailure}
	var stderr bytes.Buffer
	fixture.app.Output, fixture.app.ErrorOutput = broken, &stderr
	command := NewRoot(fixture.app)
	command.SetArgs([]string{"add", writeCLIPlugin(t), "--target", "codex,cursor", "--yes", "--format", "human"})
	err := command.ExecuteContext(t.Context())
	if !errors.Is(err, cause) || !errors.Is(err, writerFailure) || !errors.Is(err, errInstallOutput) || broken.failures == 0 || stager.calls != 1 {
		t.Fatalf("group result output lost original typed cause or replayed: stages=%d writes=%d err=%v", stager.calls, broken.failures, err)
	}
	if strings.Contains(err.Error(), "opaque-") || len(installedProjection(t, fixture)) != 0 {
		t.Fatalf("group output failure leaked detail or changed installation: %v", err)
	}
}

func TestOutputFailurePreventsEveryInteractiveRetryRoute(t *testing.T) {
	for _, route := range []string{"selected agents", "complete environment"} {
		t.Run(route, func(t *testing.T) {
			primary := fmt.Errorf("opaque-primary-319827: %w", installer.ErrTargetFactsUnavailable)
			writerFailure := errors.New("opaque-writer-918273")
			items := []agentItem{failedItem("pdf", "codex", primary)}
			outcome := &installOutcome{Agents: reportByAgent(items), Items: items}
			err := errors.Join(humanInstallApplyError("human", outcome, primary), installOutputError(writerFailure))
			asked, reruns := 0, 0
			app := App{Terminal: true, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				asked++
				return prompt.ConfirmationResult{Accepted: true}, nil
			}}}
			opts := &options{format: "human", installResult: outcome}
			cmd, output, stderr := retryTestCommand(t)
			var actual error
			if route == "selected agents" {
				actual = retryFailedAgents(cmd, app, opts, err, func(string, []string) error { reruns++; return nil })
			} else {
				actual = retryConverge(cmd, app, opts, err, func() error { reruns++; return nil })
			}
			if !errors.Is(actual, primary) || !errors.Is(actual, writerFailure) || !errors.Is(actual, errInstallOutput) || asked != 0 || reruns != 0 || opts.retryAttempt != 0 || output.Len()+stderr.Len() != 0 {
				t.Fatalf("failed output started another interactive path: asked=%d reruns=%d err=%v", asked, reruns, actual)
			}
			if strings.Contains(actual.Error(), "opaque-") {
				t.Fatalf("retained output error published underlying text: %q", actual.Error())
			}
		})
	}
}

func TestAppliedAgentItemsPreserveObservedAndMissingBindingFacts(t *testing.T) {
	actions := []dependencyAction{
		{Alias: "same", ClientID: "claude", Action: environment.ActionUnchanged},
		{Alias: "broken", ClientID: "codex", Action: environment.ActionInstall},
		{Alias: "unreached", ClientID: "gemini", Action: environment.ActionUnchanged},
	}
	cause := installer.ErrRecoveryRequired
	native := installer.Result{Outcome: installer.OutcomeCompleted, InstallationID: "extra-id"}
	extra := environment.BindingOutcome{Binding: dependencyAction{Alias: "extra", ClientID: "claude", Action: environment.ActionInstall}, Status: environment.BindingApplied, Result: &native}
	observed := []environment.BindingOutcome{
		{Binding: actions[1], Status: environment.BindingFailed, Err: cause},
		{Binding: actions[0], Status: environment.BindingUnchanged},
		extra,
	}
	before := append([]environment.BindingOutcome(nil), observed...)
	items := appliedAgentItems(actions, observed, cause)
	if len(items) != 4 || items[0].Status != "unchanged" || items[0].failure != nil || items[1].Status != "failed" || items[2].Status != "not_attempted" || items[3].installationID != "extra-id" || items[3].failure != nil {
		t.Fatalf("projection overwrote or dropped authoritative facts: %+v", items)
	}
	if items[2].failure.Kind != failureRecovery || len(retryableAgents(reportByAgent(items))) != 0 {
		t.Fatal("missing recovery binding bypassed namespace retry stop")
	}
	if !reflect.DeepEqual(before, observed) {
		t.Fatal("presentation mutated coordinator outcomes")
	}
	if clean := appliedAgentItems(actions, observed[:0], nil); len(clean) != 0 {
		t.Fatal("presentation invented success without an outcome")
	}
	early := appliedAgentItems(actions, nil, cause)
	if len(early) != len(actions) {
		t.Fatal("early failure dropped reviewed actions")
	}
	for _, item := range early {
		if item.Status != "not_attempted" || item.failure == nil || item.failure.Kind != failureRecovery {
			t.Fatalf("early refusal invented a completed no-op: %+v", item)
		}
	}
}

func TestUpdateAllHumanUsesSharedDiagnosticWithoutChangingJSON(t *testing.T) {
	cmd, output, _ := retryTestCommand(t)
	items := []agentItem{failedItem("pdf", "codex", installer.ErrTargetFactsUnavailable)}
	opts := &options{format: "human", installResult: &installOutcome{Agents: reportByAgent(items), Items: items}}
	result := updateAllResult{Batch: true, Status: "apply_failed", Failed: 1, Installations: []updateAllInstallation{{Name: "pdf", Saved: true, Status: "apply_failed"}}}
	if err := renderUpdateAll(cmd, opts, result); err != nil {
		t.Fatal(err)
	}
	assertUnavailableDiagnostic(t, output.String(), "disabled")
	opts.format = "json"
	output.Reset()
	if err := renderUpdateAll(cmd, opts, result); err != nil {
		t.Fatal(err)
	}
	withDiagnostics := append([]byte(nil), output.Bytes()...)
	decodeOneDiagnosticEnvelope(t, output.String())
	opts.installResult = nil
	output.Reset()
	if err := renderUpdateAll(cmd, opts, result); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(withDiagnostics, output.Bytes()) {
		t.Fatalf("human diagnostic state changed update-all JSON:\n%s\n%s", withDiagnostics, output.Bytes())
	}
	cmd.SetOut(targetNoticeWriter{err: io.ErrClosedPipe})
	opts.format = "human"
	if err := renderUpdateAll(cmd, opts, result); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("update-all output failure disappeared: %v", err)
	}
}

func TestSkillsApplyBrokenResultStreamRetainsCauseAndStopsRetry(t *testing.T) {
	for _, scenario := range []string{"human result", "human notice", "json result", "human short notice"} {
		t.Run(scenario, func(t *testing.T) {
			journey, request, runner := unchangedSkillsFixture(t)
			prepared, err := journey.cli.app.PrepareSkills(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer prepared.Close()
			primary := fmt.Errorf("opaque-skills-primary-738192: %w", os.ErrPermission)
			guard := &unclosedSkillsStore{Store: journey.cli.store, failure: primary}
			prepared.app.StateStore = guard
			app := journey.cli.app
			asked := 0
			app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				asked++
				return prompt.ConfirmationResult{Accepted: true}, nil
			}}
			format := "human"
			if strings.HasPrefix(scenario, "json") {
				format = "json"
			}
			writerFailure := errors.New("opaque-skills-writer-912837")
			broken := &diagnosticResultWriter{
				fail: func(body []byte) bool {
					return !strings.Contains(scenario, "notice") || bytes.Contains(body, []byte("Technical details"))
				},
				err: writerFailure, short: strings.Contains(scenario, "short"),
			}
			cmd, _, stderr := retryTestCommand(t)
			cmd.SetOut(broken)
			app.Output, app.ErrorOutput = broken, stderr
			err = applySkillsWithRecovery(cmd, app, &options{format: format, scope: "user"}, prepared, true)
			expectedWriter := writerFailure
			if broken.short {
				expectedWriter = io.ErrShortWrite
			}
			if !errors.Is(err, primary) || !errors.Is(err, expectedWriter) || !errors.Is(err, errInstallOutput) || guard.checks != 1 || asked != 0 || runner.mutations != 0 {
				t.Fatalf("broken Skills stream lost a cause or started retry: checks=%d prompts=%d native=%d err=%v", guard.checks, asked, runner.mutations, err)
			}
			if format == "human" && strings.Contains(err.Error(), "opaque-") {
				t.Fatalf("Skills output error published underlying detail: %q", err.Error())
			}
		})
	}
}

func TestSuccessfulSkillsCommandIgnoresUnavailableDiagnosticDirectory(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			journey, request, runner := unchangedSkillsFixture(t)
			app := journey.cli.app
			app.InstallLogDir = diagnosticLogDirectory(t, "unwritable")
			before := savedSkillsDurableTree(t, app.DependenciesRoot)
			var output, stderr bytes.Buffer
			app.Output, app.ErrorOutput = &output, &stderr
			command := NewRoot(app)
			command.SetArgs([]string{"skills", "add", request.Selections[0].Website, "--skill", "pdf", "--target", "codex", "--no-ui", "--yes", "--scope", "user", "--format", format})
			if err := command.ExecuteContext(t.Context()); err != nil {
				t.Fatalf("diagnostic location changed successful installation: %v\n%s\n%s", err, output.String(), stderr.String())
			}
			if runner.mutations != 0 || !reflect.DeepEqual(before, savedSkillsDurableTree(t, app.DependenciesRoot)) {
				t.Fatal("successful repeat changed native/durable state")
			}
			public := output.String() + stderr.String()
			if strings.Contains(public, "could not be saved") || strings.Contains(public, "not recorded") || strings.Contains(public, "Failure codes:") {
				t.Fatalf("successful command acquired diagnostic failure notice: %q", public)
			}
			if format == "json" {
				envelope := decodeOneDiagnosticEnvelope(t, output.String())
				if envelope["status"] != "succeeded" || envelope["result"] != outputResultSuccess {
					t.Fatalf("successful JSON contract changed: %v", envelope)
				}
			}
		})
	}
}

func TestGroupMissingLogWithholdsRawDetailsInExistingTable(t *testing.T) {
	for _, mode := range []string{"disabled", "unwritable"} {
		t.Run(mode, func(t *testing.T) {
			combined := cleanupFailureGroup()
			successful, manual := combined.Targets[2], combined.Targets[3]
			cause := errors.New("opaque-group-result-219837")
			cmd, output, stderr := retryTestCommand(t)
			opts := &options{format: "human"}
			app := App{InstallLogDir: diagnosticLogDirectory(t, mode)}
			err := app.recordGroupOutcome(cmd, opts, &combined, cause)
			if !errors.Is(err, cause) {
				t.Fatalf("group presentation lost cause: %v", err)
			}
			if err := renderAddMultiResult(cmd, opts, combined, domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo"}}); err != nil {
				t.Fatal(err)
			}
			public := output.String() + stderr.String() + err.Error()
			assertUnavailableDiagnostic(t, public, mode)
			if strings.Contains(public, "private native diagnostic") || strings.Contains(public, "opaque-group-result") {
				t.Fatalf("existing group table bypassed safe fallback: %q", public)
			}
			if !reflect.DeepEqual(combined.Targets[2], successful) || !reflect.DeepEqual(combined.Targets[3], manual) || combined.Targets[0].Error.Stage != "activation" {
				t.Fatal("fallback changed successful peers or stage identity")
			}
		})
	}
	combined := cleanupFailureGroup()
	original := combined.Targets[0].Error.Message
	cmd, _, stderr := retryTestCommand(t)
	err := (App{}).recordGroupOutcome(cmd, &options{format: "json"}, &combined, errors.New("opaque-json-cause"))
	if err == nil || combined.Targets[0].Error.Message != original || stderr.Len() != 0 {
		t.Fatal("human fallback changed machine-readable group facts")
	}
}
