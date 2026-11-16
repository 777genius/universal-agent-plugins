package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func cleanupFailureGroup() addMultiResult {
	failure := &usecase.GroupTargetFailure{Stage: "activation", Message: "private native diagnostic"}
	return addMultiResult{Plugin: "demo", Status: string(usecase.GroupPhaseExternalPartialFailure), Succeeded: 1, Failed: 2, Targets: []addTargetResult{
		{Target: "codex", Status: string(usecase.GroupTargetExternalFailed), Error: failure,
			RetryCommand: "npx universal-agent-plugins add demo --target codex",
			Output:       addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalFailed, Failure: failure}}},
		{Target: "gemini", Status: string(usecase.GroupTargetExternalNotAttempted),
			RetryCommand: "npx universal-agent-plugins add demo --target gemini",
			Output:       addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalNotAttempted}}},
		{Target: "claude", Status: string(usecase.GroupTargetExternalCompleted),
			Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalCompleted}}},
		{Target: "chatgpt", Status: "action_required", NextAction: "Finish ChatGPT setup."},
	}}
}

func TestGroupCleanupFailureKeepsTypedCauseAndPrivateDiagnostics(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			cmd, out, _ := retryTestCommand(t)
			app := App{Terminal: true, InstallLogDir: filepath.Join(t.TempDir(), "logs")}
			opts := &options{format: format}
			combined := cleanupFailureGroup()
			completed, manual := combined.Targets[2], combined.Targets[3]
			probe := &domain.UnclosedClientProbe{Operation: "private native operation", Err: fmt.Errorf("private native cause: %w", context.DeadlineExceeded)}
			ordinary := errors.New("private earlier failure")
			cause := errors.Join(ordinary, probe)
			actual := app.recordGroupOutcome(cmd, opts, &combined, cause)
			var retained *domain.UnclosedClientProbe
			if !errors.Is(actual, ordinary) || !errors.Is(actual, context.DeadlineExceeded) || !errors.As(actual, &retained) || retained != probe || strings.Contains(actual.Error(), "private") {
				t.Fatalf("lost typed cleanup cause or exposed native diagnostics: %v", actual)
			}
			if !reflect.DeepEqual(combined.Targets[2], completed) || !reflect.DeepEqual(combined.Targets[3], manual) {
				t.Fatal("cleanup failure changed successful or manual peer results")
			}
			setPreflightNextActions(combined.Targets[:2])
			for _, target := range combined.Targets[:2] {
				if target.RetryCommand != "" || target.Error.Stage != "native_process_cleanup" || !strings.Contains(target.NextAction, "process has stopped") || target.Output.NextAction != target.NextAction {
					t.Fatalf("cleanup row offered a retry or lost its action: %+v", target)
				}
			}
			if opts.installResult == nil || len(opts.installResult.failedAgents()) != 2 || len(retryableAgents(opts.installResult.Agents)) != 0 {
				t.Fatal("group cleanup left a retryable report")
			}
			if err := renderAddMultiResult(cmd, opts, combined, domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo"}}); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "private") || strings.Contains(out.String(), "npx universal-agent-plugins add demo") || !strings.Contains(out.String(), "process has stopped") {
				t.Fatalf("unsafe public group output: %s", out.String())
			}
			log, err := os.ReadFile(opts.installResult.LogFile)
			if err != nil || !strings.Contains(string(log), "private native diagnostic") || !strings.Contains(string(log), "private earlier failure") {
				t.Fatalf("technical log lost the original details: %v %s", err, log)
			}
		})
	}
}

func TestGroupCleanupBeforeMutationRetainsNoRetryRecord(t *testing.T) {
	for _, phase := range []usecase.GroupPhase{usecase.GroupPhasePlanned, usecase.GroupPhasePreparationFailed} {
		t.Run(string(phase), func(t *testing.T) {
			cmd, _, errOut := retryTestCommand(t)
			app := App{Terminal: true, InstallLogDir: filepath.Join(t.TempDir(), "logs")}
			opts := &options{format: "human"}
			combined := cleanupFailureGroup()
			combined.Status = string(phase)
			probe := &domain.UnclosedClientProbe{Operation: "private native operation", Err: context.DeadlineExceeded}
			actual := app.recordGroupOutcome(cmd, opts, &combined, probe)
			if !errors.Is(actual, probe) || strings.Contains(actual.Error(), "private") || opts.installResult != nil || errOut.Len() != 0 {
				t.Fatalf("preflight cleanup changed no-retry/no-log contract: %v %+v %q", actual, opts.installResult, errOut.String())
			}
		})
	}
}

func TestUnclosedCleanupBlocksMixedRetryHintsEvenWhenNotAttempted(t *testing.T) {
	for _, status := range []environment.BindingStatus{environment.BindingFailed, environment.BindingNotAttempted} {
		t.Run(string(status), func(t *testing.T) {
			failure := describeBindingFailure(environment.BindingOutcome{Status: status, Err: &domain.UnclosedClientProbe{Operation: "native inventory", Err: context.DeadlineExceeded}})
			if failure.Kind != failureNativeCleanup || failure.Retryable {
				t.Fatalf("skipped cleanup lost its retry barrier: %+v", failure)
			}
			reports := reportByAgent([]agentItem{failedItem("demo", "claude", errors.New("ordinary retryable failure")), {Alias: "demo", ClientID: "codex", Status: string(status), failure: failure}})
			if retryTargetHint(reports) != "" || len(retryableAgents(reports)) != 0 {
				t.Fatal("mixed cleanup failure advertised a partial retry")
			}
			cmd, _, _ := retryTestCommand(t)
			for _, verb := range []string{"add", "install", "repair"} {
				cmd.Use = verb
				if retryHintFor(cmd, reports) != "" {
					t.Fatalf("%s advertised retry before process cleanup", verb)
				}
			}
		})
	}
}
