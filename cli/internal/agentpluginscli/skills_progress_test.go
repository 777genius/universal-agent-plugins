package agentpluginscli

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func completedSkillClient() installer.ClientResult {
	return installer.ClientResult{Activation: string(domain.ActivationActive), Verification: string(domain.VerificationInstalled), Authentication: string(domain.AuthenticationNotRequired)}
}

func TestSkillsProgressCountsAllBindingsBeforeFinishingAgent(t *testing.T) {
	var output bytes.Buffer
	actions := []SkillsAction{{Alias: "first", ClientID: "codex", Action: "install"}, {Alias: "second", ClientID: "codex", Action: "install"}, {Alias: "first", ClientID: "claude", Action: "install"}}
	progress := startSkillsProgressBoard(App{Terminal: true, ErrorOutput: &output}, &options{format: "human", plain: true}, actions)
	event := SkillsProgress{Alias: "first", ClientID: "codex", Action: "install", Phase: "finished", Result: installer.Result{Outcome: installer.OutcomeCompleted, Client: completedSkillClient()}}
	progress.observe(SkillsProgress{Alias: "first", ClientID: "codex", Action: "install", Phase: "preparing"})
	progress.observe(event)
	row := progress.board.rows[0]
	if row.bindingsCompleted != 1 || row.bindingsTotal != 2 || row.bindingState == "installed" {
		t.Fatalf("first Skill prematurely finished the agent: %+v", row)
	}
	progress.observe(event) // duplicate observations cannot inflate counts.
	if progress.board.rows[0].bindingsCompleted != 1 {
		t.Fatal("duplicate event inflated completion")
	}
	progress.finish(SkillsBatchResult{Status: "partial", Items: []SkillsBatchItem{
		{Alias: "first", ClientID: "codex", Action: "install", Status: "completed", Client: completedSkillClient()},
		{Alias: "second", ClientID: "codex", Action: "install", Status: "failed"},
		{Alias: "first", ClientID: "claude", Action: "install", Status: "not_attempted"},
	}})
	if progress.board.rows[0].bindingState != "failed" || progress.board.rows[1].bindingState != "not attempted" {
		t.Fatalf("wrong final rows: %+v", progress.board.rows)
	}
	for _, want := range []string{"1/2 waiting", "1/2 failed", "0/1 not attempted"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing %q: %s", want, output.String())
		}
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatal("plain progress contains terminal controls")
	}
}

func TestSkillsProgressFinalResultOverridesEarlierEvents(t *testing.T) {
	var output bytes.Buffer
	progress := startSkillsProgressBoard(App{Terminal: true, ErrorOutput: &output}, &options{format: "human"}, []SkillsAction{{Alias: "same-name-source-a", ClientID: "codex", Action: "install"}, {Alias: "same-name-source-b", ClientID: "codex", Action: "unchanged"}})
	progress.observe(SkillsProgress{Alias: "same-name-source-a", ClientID: "codex", Action: "install", Phase: "finished", Result: installer.Result{Outcome: installer.OutcomeCompleted, Client: completedSkillClient()}})
	progress.finish(SkillsBatchResult{Status: "failed"})
	if row := progress.board.rows[0]; row.bindingsCompleted != 0 || row.bindingState != "not attempted" {
		t.Fatalf("animation overrode transaction result: %+v", row)
	}
}

func TestSkillsProgressCancellationAfterOneBindingKeepsItsSuccess(t *testing.T) {
	var output bytes.Buffer
	progress := startSkillsProgressBoard(App{Terminal: true, ErrorOutput: &output}, &options{format: "human"}, []SkillsAction{{Alias: "one", ClientID: "codex", Action: "install"}, {Alias: "two", ClientID: "codex", Action: "install"}})
	progress.finish(SkillsBatchResult{Status: "partial", Items: []SkillsBatchItem{{Alias: "one", ClientID: "codex", Action: "install", Status: "completed", Client: completedSkillClient()}, {Alias: "two", ClientID: "codex", Action: "install", Status: "not_attempted"}}})
	if row := progress.board.rows[0]; row.bindingsCompleted != 1 || row.bindingState != "not completed" {
		t.Fatalf("partial agent was presented as wholly unattempted: %+v", row)
	}
}

func TestSkillsProgressPreservesReadinessAndUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name string
		item SkillsBatchItem
		want string
	}{
		{"verified", SkillsBatchItem{Status: "completed", Client: completedSkillClient()}, "installed"},
		{"untrusted", SkillsBatchItem{Status: "completed", Client: installer.ClientResult{Activation: string(domain.ActivationPrepared), Verification: string(domain.VerificationInstalled), Authentication: string(domain.AuthenticationNotRequired)}}, "setup required"},
		{"sign in", SkillsBatchItem{Status: "completed", Client: installer.ClientResult{Authentication: string(domain.AuthenticationPending)}}, "sign-in required"},
		{"rollback", SkillsBatchItem{Status: "incomplete", Client: installer.ClientResult{GroupPhase: string(usecase.GroupTargetManagedRolledBack)}}, "rolled back"},
		{"unchanged", SkillsBatchItem{Status: "unchanged"}, "unchanged"},
		{"cancel", SkillsBatchItem{Status: string(installer.OutcomeCancelled)}, "canceled"},
		{"removed", SkillsBatchItem{Status: "completed", Action: "remove"}, "removed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := skillsItemPresentation(tc.item); got != tc.want {
				t.Fatalf("%s != %s", got, tc.want)
			}
		})
	}
}

func TestSkillsProgressCompletedCounterDoesNotHideSetupRequired(t *testing.T) {
	var output bytes.Buffer
	progress := startSkillsProgressBoard(App{Terminal: true, ErrorOutput: &output}, &options{format: "human", plain: true}, []SkillsAction{
		{Alias: "one", ClientID: "claude", Action: "install"}, {Alias: "two", ClientID: "claude", Action: "unchanged"},
	})
	progress.finish(SkillsBatchResult{Status: "succeeded", Items: []SkillsBatchItem{
		{Alias: "one", ClientID: "claude", Action: "install", Status: "completed", Client: installer.ClientResult{Activation: string(domain.ActivationPrepared), Verification: string(domain.VerificationInstalled), Authentication: string(domain.AuthenticationNotRequired)}},
		{Alias: "two", ClientID: "claude", Action: "unchanged", Status: "unchanged"},
	}})
	row := progress.board.rows[0]
	if row.bindingsCompleted != 2 || row.bindingState != "setup required" || !strings.Contains(output.String(), "! 2/2 setup required") {
		t.Fatalf("coarse completion overstated readiness: %+v: %s", row, output.String())
	}
}

func TestSkillsResultMappingDoesNotBlameAnUnattemptedBinding(t *testing.T) {
	actions := []SkillsAction{{Alias: "one", ClientID: "claude", Action: "install"}, {Alias: "one", ClientID: "codex", Action: "install"}, {Alias: "two", ClientID: "claude", Action: "install"}}
	failure := errors.New("native error")
	outcome := func(index int, status environment.BindingStatus, result *installer.Result, err error) environment.BindingOutcome {
		action := actions[index]
		return environment.BindingOutcome{Binding: environment.PreparedBinding{Alias: action.Alias, ClientID: action.ClientID, Action: environment.ActionInstall}, Status: status, Result: result, Err: err}
	}
	completed := &installer.Result{Outcome: installer.OutcomeCompleted}
	for _, tc := range []struct {
		name    string
		native  []environment.BindingOutcome
		want    []string
		mutated bool
	}{
		{"before bindings", nil, []string{"not_attempted", "not_attempted", "not_attempted"}, false},
		{"first prepare", []environment.BindingOutcome{outcome(0, environment.BindingFailed, nil, failure)}, []string{"failed", "not_attempted", "not_attempted"}, false},
		{"first apply", []environment.BindingOutcome{outcome(0, environment.BindingFailed, &installer.Result{Outcome: installer.OutcomeIncomplete, Mutated: true}, failure)}, []string{"incomplete", "not_attempted", "not_attempted"}, true},
		{"second prepare", []environment.BindingOutcome{outcome(0, environment.BindingApplied, completed, nil), outcome(1, environment.BindingFailed, nil, failure)}, []string{"completed", "failed", "not_attempted"}, true},
		{"later success after prepare failure", []environment.BindingOutcome{outcome(1, environment.BindingApplied, completed, nil), outcome(0, environment.BindingFailed, nil, failure), outcome(2, environment.BindingNotAttempted, nil, failure)}, []string{"failed", "completed", "not_attempted"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			items, mutated := skillsBatchItems(actions, tc.native, failure)
			for _, item := range items {
				got = append(got, item.Status)
			}
			if !reflect.DeepEqual(got, tc.want) || mutated != tc.mutated {
				t.Fatalf("statuses=%v mutated=%v; want %v mutated=%v", got, mutated, tc.want, tc.mutated)
			}
		})
	}
}

func TestSkillsOutcomeMappingPreservesLaterNativeFactsAndManualActions(t *testing.T) {
	actions := []SkillsAction{{Alias: "one", ClientID: "codex", Action: "install"}, {Alias: "one", ClientID: "claude", Action: "install"}}
	native := installer.Result{Outcome: installer.OutcomeCompleted, Client: completedSkillClient(), ManualActions: []string{"Reload the selected plugin"}}
	outcomes := []environment.BindingOutcome{
		{Binding: environment.PreparedBinding{Alias: "one", ClientID: "claude", Action: environment.ActionInstall}, Status: environment.BindingApplied, Result: &native},
		{Binding: environment.PreparedBinding{Alias: "one", ClientID: "codex", Action: environment.ActionInstall}, Status: environment.BindingFailed, Err: installer.ErrTargetFactsUnavailable},
	}
	items, mutated := skillsBatchItems(actions, outcomes, installer.ErrTargetFactsUnavailable)
	if !mutated || items[0].Status != "failed" || items[0].failure == nil || items[0].failure.Kind != failureTargetUnavailable {
		t.Fatalf("prepare failure lost its own classification: %+v", items)
	}
	if items[1].Status != "completed" || items[1].failure != nil || !reflect.DeepEqual(items[1].Client, native.Client) || !reflect.DeepEqual(items[1].ManualActions, native.ManualActions) {
		t.Fatalf("later native facts shifted onto another binding: %+v", items)
	}
	items[1].ManualActions[0] = "changed presentation"
	if native.ManualActions[0] != "Reload the selected plugin" {
		t.Fatal("presentation changed the installer's manual action")
	}
}

type progressPanicWriter struct{}

func (progressPanicWriter) Write([]byte) (int, error) { panic("broken terminal") }

func TestSkillsProgressObserversAndWritersAreNonAuthoritative(t *testing.T) {
	observed := 0
	for range 3 {
		emitSkillsProgress(func(SkillsProgress) { observed++; panic("observer failure") }, SkillsProgress{})
	}
	if observed != 3 {
		t.Fatal("observer interrupted caller")
	}
	native := installer.Result{Outcome: installer.OutcomeCompleted, ManualActions: []string{"trust workspace"}, Client: installer.ClientResult{RequiredComponents: []string{"skill"}, Failure: &installer.ClientFailure{Message: "native fact"}}}
	emitSkillsProgress(func(event SkillsProgress) {
		event.Result.ManualActions[0] = "changed"
		event.Result.Client.RequiredComponents[0] = "changed"
		event.Result.Client.Failure.Message = "changed"
	}, SkillsProgress{Result: native})
	if native.ManualActions[0] != "trust workspace" || native.Client.RequiredComponents[0] != "skill" || native.Client.Failure.Message != "native fact" {
		t.Fatal("observer changed transaction result facts")
	}
	progress := startSkillsProgressBoard(App{Terminal: true, ErrorOutput: progressPanicWriter{}}, &options{format: "human"}, []SkillsAction{{Alias: "one", ClientID: "codex", Action: "install"}})
	emitSkillsProgress(progress.observe, SkillsProgress{Alias: "one", ClientID: "codex", Action: "install", Phase: "preparing"})
	progress.finish(SkillsBatchResult{Status: "failed"})
	if startSkillsProgressBoard(App{Terminal: true}, &options{format: "json"}, []SkillsAction{{Alias: "one"}}) != nil {
		t.Fatal("JSON created progress renderer")
	}
}
