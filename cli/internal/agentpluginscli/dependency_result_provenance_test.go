package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Native results may omit every ID and even be identical. The coordinator's
// binding status remains the authority; neither row position in reports nor a
// peer's failure can determine whether this particular result succeeded.
func TestObservedDependencyResultsKeepMixedBindingProvenance(t *testing.T) {
	for _, status := range []environment.BindingStatus{environment.BindingApplied, environment.BindingUnchanged} {
		for _, failedFirst := range []bool{false, true} {
			name := string(status)
			if failedFirst {
				name += "/failed-first"
			}
			t.Run(name, func(t *testing.T) {
				native := installer.Result{Outcome: installer.OutcomeCompleted, Reason: "Keep this native reason",
					NextActions:   []installer.NextAction{{Kind: "activate", Reason: "Keep this native next action"}},
					ManualActions: []string{"Keep this native manual action"}}
				healthy := environment.BindingOutcome{Binding: dependencyAction{Alias: "healthy", ClientID: "claude"}, Status: status, Result: &native}
				failed := environment.BindingOutcome{Binding: dependencyAction{Alias: "failed", ClientID: "codex"}, Status: environment.BindingFailed, Result: &native, Err: context.Canceled}
				outcomes := []environment.BindingOutcome{healthy, failed}
				if failedFirst {
					outcomes[0], outcomes[1] = outcomes[1], outcomes[0]
				}
				// A preceding binding without a Result must not shift the
				// correspondence between produced native rows and their facts.
				outcomes = append([]environment.BindingOutcome{{Binding: dependencyAction{Alias: "unreached", ClientID: "gemini"}, Status: environment.BindingNotAttempted, Err: context.Canceled}}, outcomes...)
				items := outcomeAgentItems(outcomes)
				// Reports may use a different canonical order from native results.
				items[0], items[2] = items[2], items[0]
				opts := &options{format: "human", installResult: &installOutcome{Items: items, Agents: reportByAgent(items)}}
				projected := dependencyBindingResultOutput(outcomes)
				if len(projected) != 2 {
					t.Fatalf("nil native result changed the produced row count: %+v", projected)
				}
				for index, row := range projected {
					displayed := dependencyHumanResult(opts, row)
					isHealthy := index == 0 != failedFirst
					if isHealthy {
						if !reflect.DeepEqual(displayed, row) {
							t.Fatalf("unrelated failure rewrote healthy native result with no IDs: before=%+v after=%+v", row, displayed)
						}
					} else if displayed.Outcome != "failed" || len(displayed.NextActions)+len(displayed.ManualActions) != 0 || displayed.Reason != "The installation was canceled." {
						t.Fatalf("failed binding borrowed identical completed native facts: %+v", displayed)
					}
				}
				// Private provenance must be absent from the public JSON and
				// must leave the original native value and slices unchanged.
				plainJSON, _ := json.Marshal(dependencyResultOutput(environment.Results(outcomes)))
				boundJSON, _ := json.Marshal(projected)
				if !bytes.Equal(plainJSON, boundJSON) {
					t.Fatalf("binding provenance changed public JSON:\n%s\n%s", plainJSON, boundJSON)
				}
				cmd, output, _ := retryTestCommand(t)
				if err := renderDependencyBindingResults(cmd, opts, nil, outcomes, context.Canceled); err != nil {
					t.Fatal(err)
				}
				for _, retained := range []string{"Keep this native reason", "Keep this native next action", "Keep this native manual action"} {
					if strings.Count(output.String(), retained) != 1 {
						t.Fatalf("mixed output lost healthy guidance or leaked failed guidance %q: %s", retained, output.String())
					}
				}
			})
		}
	}
}

func TestUnattributedHealthyMissingIdentityIgnoresUnrelatedFailure(t *testing.T) {
	result := dependencyResultDTO{Outcome: "completed", Reason: "Healthy result", ManualActions: []string{"Keep this action"}}
	items := []agentItem{failedItem("unrelated", "codex", context.Canceled)}
	opts := &options{installResult: &installOutcome{Agents: reportByAgent(items), Items: items}}
	if displayed := dependencyHumanResult(opts, result); !reflect.DeepEqual(displayed, result) {
		t.Fatal("another binding's failure was treated as authority over an unattributed healthy result")
	}
}
