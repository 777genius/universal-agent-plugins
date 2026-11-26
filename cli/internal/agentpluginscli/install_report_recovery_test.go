package agentpluginscli

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestSkippedRecoveryStopsOtherAgentsRetry(t *testing.T) {
	for _, recoveryFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("recovery-first=%t", recoveryFirst), func(t *testing.T) {
			skipped := func(alias string, cause error) agentItem {
				return agentItem{Alias: alias, ClientID: "codex", Status: "not_attempted",
					failure: describeBindingFailure(environment.BindingOutcome{Status: environment.BindingNotAttempted, Err: cause})}
			}
			items := []agentItem{
				failedItem("retryable", "gemini", fs.ErrPermission),
				skipped("security", installer.ErrAssessmentRejected),
				skipped("recovery", errors.Join(installer.ErrRecoveryRequired, retainedRevisionTestCause())),
			}
			if recoveryFirst {
				items[1], items[2] = items[2], items[1]
			}
			reports := reportByAgent(items)
			if len(reports) != 2 || len(reports[1].Problems) != 1 {
				t.Fatalf("identical public skipped explanations must still group: %+v", reports)
			}
			if retryable := retryableAgents(reports); len(retryable) != 0 {
				t.Fatalf("skipped recovery must stop all namespace retries: %+v", retryable)
			}
			if hint := retryTargetHint(reports); hint != "" {
				t.Fatalf("skipped recovery offered a target retry: %q", hint)
			}
			for _, verb := range []string{"add", "install", "sync", "update"} {
				if hint := retryHintFor(&cobra.Command{Use: verb}, reports); hint != "" {
					t.Fatalf("%s offered a namespace retry during recovery: %q", verb, hint)
				}
			}
			bindings := (&installOutcome{Items: items, Agents: reports}).bindingsJSON()
			for _, binding := range bindings[1:] {
				if binding.Code != string(failureSkipped) || binding.Status != "not_attempted" {
					t.Fatalf("retaining the stop policy changed skipped presentation: %+v", binding)
				}
			}
		})
	}
}

func TestSkippedSecurityFailureAllowsOtherAgentsRetry(t *testing.T) {
	items := []agentItem{
		{Alias: "security", ClientID: "codex", Status: "not_attempted",
			failure: describeBindingFailure(environment.BindingOutcome{
				Status: environment.BindingNotAttempted, Err: installer.ErrAssessmentRejected,
			})},
		failedItem("retryable", "gemini", fs.ErrPermission),
	}
	reports := reportByAgent(items)
	if got := retryTargetHint(reports); got != "To retry only these agents, run the same command with --target gemini" {
		t.Fatalf("a per-agent security refusal must not become a namespace stop: %q", got)
	}
}
