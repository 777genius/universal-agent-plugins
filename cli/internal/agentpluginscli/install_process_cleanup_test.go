package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func unclosedInstallErrors(cause error) map[string]error {
	probe := &domain.UnclosedClientProbe{Operation: "client listing", Err: cause}
	return map[string]error{
		"bare":    probe,
		"wrapped": fmt.Errorf("native observation: %w", probe),
		"deadline": fmt.Errorf("native observation: %w", &domain.UnclosedClientProbe{
			Operation: "client listing", Err: errors.Join(cause, context.DeadlineExceeded),
		}),
	}
}

func TestUnclosedProbeBindingFailureIsNeverRetryable(t *testing.T) {
	private := errors.New("close failed at /private/provider/fixture token=private-value")
	for name, cause := range unclosedInstallErrors(private) {
		t.Run(name, func(t *testing.T) {
			for _, status := range []environment.BindingStatus{environment.BindingFailed, environment.BindingNotAttempted} {
				outcome := environment.BindingOutcome{Status: status, Err: cause}
				want := failureProcessCleanup
				if status == environment.BindingFailed {
					outcome.Result = &installer.Result{Outcome: installer.OutcomeIncomplete}
				}
				failure := describeBindingFailure(outcome)
				if failure == nil || failure.Kind != want || failure.Retryable {
					t.Fatalf("%s must keep cleanup refusal: %+v", status, failure)
				}
				if !strings.Contains(failure.Detail, private.Error()) {
					t.Fatal("the technical log must retain the original cause")
				}
				reports := reportByAgent([]agentItem{{Alias: "demo", ClientID: "claude", Status: string(status), failure: failure}})
				if len(retryableAgents(reports)) != 0 || retryTargetHint(reports) != "" {
					t.Fatal("cleanup uncertainty must not offer a retry")
				}
				var output bytes.Buffer
				if err := renderAgentAttention(&output, reports, "", retryTargetHint(reports)); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(output.String(), "/private/provider/fixture") || strings.Contains(output.String(), "private-value") {
					t.Fatalf("provider detail reached the terminal: %s", output.String())
				}
			}
			if failure := describeSyncFailure(cause); failure.Kind != failureProcessCleanup || failure.Retryable {
				t.Fatalf("a pre-binding cleanup refusal must also block retry: %+v", failure)
			}
		})
	}
}

func TestOrdinaryInstallerFailuresKeepTheirRetryBehavior(t *testing.T) {
	for _, cause := range []error{errors.New("closed client command failed"), context.DeadlineExceeded} {
		failure := describeBindingFailure(environment.BindingOutcome{Status: environment.BindingFailed, Err: cause})
		if failure == nil || !failure.Retryable || failure.Kind == failureProcessCleanup {
			t.Fatalf("ordinary failures must keep their existing retry classification: %+v", failure)
		}
		summary := errors.New("public group summary")
		if got := retainUnclosedInstallCause(summary, cause); reflect.ValueOf(got) != reflect.ValueOf(summary) {
			t.Fatal("ordinary group errors must keep their existing public summary")
		}
	}
}

func TestUnclosedGroupFailureKeepsCauseWithoutOfferingRetryOrLeakingDetail(t *testing.T) {
	private := errors.New("close failed at /private/provider/group token=private-value")
	for name, cause := range unclosedInstallErrors(private) {
		t.Run(name, func(t *testing.T) {
			var offered []string
			app, opts := retryAppAndOptions(t, &offered)
			cmd, _, _ := retryTestCommand(t)
			failed := &usecase.GroupTargetFailure{Stage: "activation", Message: private.Error()}
			combined := addMultiResult{
				Plugin: "demo", Status: string(usecase.GroupPhaseExternalPartialFailure), Succeeded: 1, Failed: 1,
				Targets: []addTargetResult{
					{Target: "codex", Status: string(usecase.GroupTargetExternalCompleted)},
					{Target: "claude", Error: failed, RetryCommand: "agentplugins add demo --target claude"},
					{Target: "gemini", RetryCommand: "agentplugins add demo --target gemini"},
				},
			}
			combined.Targets[1].Output.Result.Failure = failed
			combined.Targets[2].Output.Result.GroupPhase = usecase.GroupTargetExternalNotAttempted
			safeCause := app.recordGroupOutcome(cmd, opts, &combined, cause)
			summary := batchActivationAggregateError(combined)
			got := retainUnclosedInstallCause(summary, safeCause)
			var original, retained *domain.UnclosedClientProbe
			if !errors.As(cause, &original) || !errors.As(got, &retained) || retained != original || !errors.Is(got, private) {
				t.Fatal("the exact cleanup marker and underlying cause must survive the group aggregate")
			}
			if got.Error() != "1 of 2 client installations failed; see results above" {
				t.Fatalf("the public aggregate must keep its safe summary: %s", got.Error())
			}
			if opts.installResult.Agents[0].Failed() || len(retryableAgents(opts.installResult.Agents)) != 0 {
				t.Fatal("success must stay successful and unresolved targets must not be retryable")
			}
			if !strings.Contains(opts.installResult.Items[1].failure.Detail, private.Error()) {
				t.Fatal("the full original cause must remain available to the technical log")
			}
			for _, target := range combined.Targets[1:] {
				if target.RetryCommand != "" || target.Error == nil || target.Error.Stage != "cleanup" {
					t.Fatalf("public failed target must carry cleanup guidance without a retry command: %+v", target)
				}
			}
			wire, err := json.Marshal(combined)
			if err != nil {
				t.Fatal(err)
			}
			var terminal bytes.Buffer
			if err := renderAddMultiApplySummary(&terminal, combined, domain.PackageEnvelope{}); err != nil {
				t.Fatal(err)
			}
			for _, output := range []string{got.Error(), safeCause.Error(), string(wire), terminal.String()} {
				if strings.Contains(output, "/private/provider/group") || strings.Contains(output, "private-value") {
					t.Fatalf("provider detail reached public output: %s", output)
				}
			}
			if result := retryFailedAgents(cmd, app, opts, got, func(string, []string) error {
				t.Fatal("a group with unconfirmed process cleanup must not rerun")
				return nil
			}); !errors.Is(result, private) || len(offered) != 0 {
				t.Fatalf("cleanup refusal must be preserved without prompting: %v, offered=%v", result, offered)
			}
		})
	}
}

func TestUnclosedProbeBlocksRetryOfOtherRetryableFailures(t *testing.T) {
	private := errors.New("client process cleanup failed")
	for name, cause := range unclosedInstallErrors(private) {
		t.Run(name, func(t *testing.T) {
			var offered []string
			app, opts := retryAppAndOptions(t, &offered)
			cmd, _, _ := retryTestCommand(t)
			// Codex's ordinary failure is retryable, but another client's
			// unconfirmed process makes starting any further client unsafe.
			got := retryFailedAgents(cmd, app, opts, cause, func(string, []string) error {
				t.Fatal("a separate retryable failure must not restart work after uncertain cleanup")
				return nil
			})
			if reflect.ValueOf(got) != reflect.ValueOf(cause) || len(offered) != 0 {
				t.Fatalf("the run-wide cleanup refusal must survive without prompting: %v, offered=%v", got, offered)
			}
		})
	}
}
