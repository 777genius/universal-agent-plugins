package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestSkillsRepeatedPrepublicationFailureReportsEveryBinding(t *testing.T) {
	journey, request, runner := unchangedSkillsFixture(t)
	prepared, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	if len(prepared.Actions()) != 1 || prepared.Actions()[0].Action != "unchanged" {
		t.Fatalf("fixture did not prepare a repeat: %+v", prepared.Actions())
	}
	before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
	cause := errors.New("private readiness defect")
	prepared.app.StateStore = &unclosedSkillsStore{Store: journey.cli.store, failure: cause}
	result, err := prepared.Apply(t.Context())
	if !errors.Is(err, cause) || result.Status != "failed" || len(result.Items) != 1 {
		t.Fatalf("repeat lost its refusal: result=%+v err=%v", result, err)
	}
	item := result.Items[0]
	if item.Status != "not_attempted" || item.failure == nil || item.failure.Kind != failureUnknown {
		t.Fatalf("unobserved repeat was promoted to success: %+v", item)
	}
	items := skillsAgentItems(result.Items)
	reports := reportByAgent(items)
	entries := agentLogEntries(items)
	if len(reports) != 1 || !reports[0].Failed() || len(reports[0].Succeeded) != 0 || len(retryableAgents(reports)) != 1 {
		t.Fatalf("failed repeat disappeared from attention/retry: %+v", reports)
	}
	if len(entries) != 1 || entries[0].Status != "not_attempted" || entries[0].Detail != cause.Error() {
		t.Fatalf("diagnostic record lost its original cause: %+v", entries)
	}
	if runner.mutations != 0 || !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) {
		t.Fatal("a refused repeat changed native or durable state")
	}
}

func TestSkillsEarlyApplyRefusalsKeepReviewedBindings(t *testing.T) {
	journey, request, runner := unchangedSkillsFixture(t)
	for _, refusal := range []string{"closed", "input changed", "already applied"} {
		t.Run(refusal, func(t *testing.T) {
			prepared, err := journey.cli.app.PrepareSkills(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer prepared.Close()
			switch refusal {
			case "closed":
				prepared.Close()
			case "input changed":
				path := prepared.request.intent.ManifestPath
				before, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.WriteFile(path, before, 0o600); err != nil {
						t.Error(err)
					}
				})
				if err := os.WriteFile(path, append(append([]byte(nil), before...), []byte("\n# changed after review\n")...), 0o600); err != nil {
					t.Fatal(err)
				}
			case "already applied":
				if _, err := prepared.Apply(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			result, err := prepared.Apply(t.Context())
			if err == nil || result.Status != "failed" || len(result.Items) != len(prepared.Actions()) {
				t.Fatalf("refusal lost reviewed bindings: result=%+v err=%v", result, err)
			}
			for _, item := range result.Items {
				if item.Status != "not_attempted" || item.failure == nil {
					t.Fatalf("refused action claims success: %+v", item)
				}
			}
			if runner.mutations != 0 {
				t.Fatal("refusal attempted a native mutation")
			}
		})
	}
}

func TestSkillsObservedOutcomesPrecedeUnrelatedApplyFailure(t *testing.T) {
	actions := []SkillsAction{
		{Alias: "steady", ClientID: "claude", Action: "unchanged"},
		{Alias: "new", ClientID: "claude", Action: "install"},
		{Alias: "broken", ClientID: "codex", Action: "install"},
		{Alias: "unreached", ClientID: "gemini", Action: "unchanged"},
	}
	cause := installer.ErrTargetFactsUnavailable
	native := &installer.Result{Outcome: installer.OutcomeCompleted, Mutated: true, Client: completedSkillClient(), ManualActions: []string{"Reload this client"}}
	outcome := func(index int, status environment.BindingStatus, result *installer.Result, err error) environment.BindingOutcome {
		action := actions[index]
		return environment.BindingOutcome{Binding: environment.PreparedBinding{Alias: action.Alias, ClientID: action.ClientID, Action: environment.DesiredAction(action.Action)}, Status: status, Result: result, Err: err}
	}
	// Results deliberately arrive out of order and omit an unreached planned no-op.
	observed := []environment.BindingOutcome{outcome(2, environment.BindingFailed, nil, cause), outcome(1, environment.BindingApplied, native, nil), outcome(0, environment.BindingUnchanged, nil, nil)}
	items, mutated := skillsBatchItems(actions, observed, cause)
	if !mutated || len(items) != 4 {
		t.Fatalf("lost mutation or binding: %+v mutated=%t", items, mutated)
	}
	for index, want := range []string{"unchanged", "completed", "failed", "not_attempted"} {
		if items[index].Status != want || (items[index].failure != nil) != (index >= 2) {
			t.Fatalf("item %d: %+v; wanted %s", index, items[index], want)
		}
	}
	if !reflect.DeepEqual(items[1].Client, native.Client) || !reflect.DeepEqual(items[1].ManualActions, native.ManualActions) {
		t.Fatal("native facts or guidance were lost")
	}
	reports := reportByAgent(skillsAgentItems(items))
	if len(reports) != 3 || reports[0].Failed() || len(reports[0].Succeeded) != 2 || !reports[1].Failed() || !reports[2].Failed() {
		t.Fatalf("unrelated failure tainted observed successes: %+v", reports)
	}
}

func TestFailedBindingCannotBorrowSuccessfulNativeStatus(t *testing.T) {
	for _, nativeOutcome := range []installer.Outcome{installer.OutcomeCompleted, installer.OutcomeUnchanged} {
		for _, cause := range []error{context.Canceled, installer.ErrRecoveryRequired, errors.New("commit failed")} {
			t.Run(fmt.Sprintf("%s/%s", nativeOutcome, failureKindOf(cause)), func(t *testing.T) {
				native := installer.Result{Outcome: nativeOutcome, Mutated: true, Client: completedSkillClient(), ManualActions: []string{"Check the installed files"}}
				outcome := environment.BindingOutcome{Binding: environment.PreparedBinding{Alias: "pdf", ClientID: "codex", Action: environment.ActionInstall}, Status: environment.BindingFailed, Result: &native, Err: cause}
				status, _ := outcomeStatusReason(outcome)
				if status != "failed" {
					t.Fatalf("failed transaction borrowed successful native status: %s", status)
				}
				skills, mutated := skillsBatchItems([]SkillsAction{{Alias: "pdf", ClientID: "codex", Action: "install"}}, []environment.BindingOutcome{outcome}, cause)
				if !mutated || skills[0].Status != "failed" || skills[0].failure == nil || skills[0].failure.Kind != failureKindOf(cause) || !reflect.DeepEqual(skills[0].Client, native.Client) {
					t.Fatalf("failed native completion lost facts or cause: %+v", skills)
				}
				binding := (&installOutcome{Items: outcomeAgentItems([]environment.BindingOutcome{outcome})}).bindingsJSON()[0]
				if binding.Status != "failed" || binding.Code != string(failureKindOf(cause)) {
					t.Fatalf("binding JSON disagrees with result: %+v", binding)
				}
				var output bytes.Buffer
				if err := renderAgentAttention(&output, reportByAgent(skillsAgentItems(skills)), "", ""); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(output.String(), skills[0].failure.Summary) || strings.Contains(output.String(), "commit failed") {
					t.Fatalf("attention lost safe classified explanation: %q", output.String())
				}
			})
		}
	}
}

func TestItemNormalizationAgreesAcrossReportsLogsAndJSON(t *testing.T) {
	for _, tc := range []struct {
		status string
		kind   failureKind
	}{
		{"failed", failureUnknown}, {"incomplete", failureUnknown}, {"not_attempted", failureSkipped},
		{string(installer.OutcomeCancelled), failureCanceled}, {"recovery_required", failureRecovery}, {"conflict", failureEnvironmentChange},
	} {
		t.Run(tc.status, func(t *testing.T) {
			items := []agentItem{{Alias: "pdf", ClientID: "codex", Status: tc.status}}
			report := reportByAgent(items)[0]
			entry := agentLogEntries(items)[0]
			binding := (&installOutcome{Items: items}).bindingsJSON()[0]
			if !report.Failed() || len(report.Succeeded) != 0 || report.Problems[0].Failure.Kind != tc.kind || entry.Code != string(tc.kind) || binding.Code != string(tc.kind) {
				t.Fatalf("presentation channels disagree: report=%+v log=%+v json=%+v", report, entry, binding)
			}
			if items[0].failure != nil {
				t.Fatal("normalization changed the caller's input")
			}
			if tc.kind == failureRecovery && len(retryableAgents([]agentReport{report})) != 0 {
				t.Fatal("missing optional failure metadata bypassed recovery stop")
			}
		})
	}
	items := []agentItem{{Alias: "new", ClientID: "codex", Status: "completed"}, {Alias: "same", ClientID: "codex", Status: "unchanged"}}
	if report := reportByAgent(items)[0]; report.Failed() || len(report.Succeeded) != 2 {
		t.Fatalf("normalization tainted success: %+v", report)
	}
	for _, entry := range agentLogEntries(items) {
		if entry.Code != "" || entry.Summary != "" || entry.Detail != "" {
			t.Fatalf("successful diagnostic entry changed: %+v", entry)
		}
	}
}
