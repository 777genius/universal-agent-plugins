package agentpluginscli

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestFailedNativeSuccessStatusIsNormalizedWithOrWithoutSavedLog(t *testing.T) {
	for _, nativeStatus := range []installer.Outcome{installer.OutcomeCompleted, installer.OutcomeUnchanged} {
		for _, saved := range []bool{false, true} {
			name := string(nativeStatus) + "/disabled-log"
			if saved {
				name = string(nativeStatus) + "/saved-log"
			}
			t.Run(name, func(t *testing.T) {
				failed := installer.Result{Outcome: nativeStatus, Reason: "original native reason", ManualActions: []string{"original native instruction"}}
				healthy := installer.Result{Outcome: nativeStatus, Reason: "healthy native reason", ManualActions: []string{"healthy native instruction"}}
				outcomes := []environment.BindingOutcome{
					{Binding: dependencyAction{Alias: "failed", ClientID: "codex"}, Status: environment.BindingFailed, Result: &failed, Err: context.Canceled},
					{Binding: dependencyAction{Alias: "healthy", ClientID: "claude"}, Status: environment.BindingApplied, Result: &healthy},
				}
				app := App{}
				if saved {
					app.InstallLogDir = t.TempDir()
				}
				opts := &options{format: "human"}
				outcome := app.finishInstall(opts, "install", outcomeAgentItems(outcomes))
				if saved {
					if _, err := os.Stat(outcome.LogFile); err != nil {
						t.Fatalf("fixture did not save the real technical record: %v", err)
					}
				}
				rows := dependencyBindingResultOutput(outcomes)
				before, _ := json.Marshal(rows)
				displayed := dependencyHumanResult(opts, rows[0])
				if displayed.Outcome != "failed" {
					t.Fatalf("log availability promoted failed binding to native success: %+v", displayed)
				}
				if saved && (displayed.Reason != failed.Reason || !reflect.DeepEqual(displayed.ManualActions, failed.ManualActions)) {
					t.Fatal("status correction changed the established saved-log detail policy")
				}
				if !reflect.DeepEqual(dependencyHumanResult(opts, rows[1]), rows[1]) {
					t.Fatal("failed peer changed an authoritative healthy row")
				}
				after, _ := json.Marshal(rows)
				if string(before) != string(after) || failed.Outcome != nativeStatus || healthy.Outcome != nativeStatus {
					t.Fatal("human status correction changed original native/JSON facts")
				}
			})
		}
	}
}

func TestUnknownObservedBindingStatusDoesNotBorrowNativeSuccess(t *testing.T) {
	native := installer.Result{Outcome: installer.OutcomeCompleted, Reason: "private unknown result", ManualActions: []string{"private unknown instruction"}}
	outcomes := []environment.BindingOutcome{{Binding: dependencyAction{Alias: "unknown", ClientID: "codex"}, Result: &native}}
	opts := &options{format: "human"}
	(App{}).finishInstall(opts, "install", outcomeAgentItems(outcomes))
	if !opts.installResult.Agents[0].Failed() || len(opts.installResult.Agents[0].Succeeded) != 0 {
		t.Fatal("unknown binding status became successful in the agent report")
	}
	observed := dependencyBindingResultOutput(outcomes)[0]
	if displayed := dependencyHumanResult(opts, observed); displayed.Outcome != "failed" || len(displayed.ManualActions) != 0 || displayed.Reason == native.Reason {
		t.Fatalf("unknown observed binding was treated as unattributed success: %+v", displayed)
	}
	// A legacy/native-only result carries no binding authority. An unrelated
	// unknown report cannot manufacture authority over that healthy native row.
	unattributed := dependencyResultOutput([]installer.Result{native})[0]
	if displayed := dependencyHumanResult(opts, unattributed); !reflect.DeepEqual(displayed, unattributed) {
		t.Fatal("legacy identity fallback inferred failure without a matching binding")
	}
}
