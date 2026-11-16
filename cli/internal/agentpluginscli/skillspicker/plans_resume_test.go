package skillspicker

import (
	"context"
	"sync/atomic"
	"testing"
)

type reviewedPrepared struct {
	*testPrepared
	review Plan
}

func (p *reviewedPrepared) Review() Plan { return p.review }

// A native setup repair uses the same reviewed admission boundary as install.
// Unknown actions and conflicts must remain nonconfirmable at the HTTP boundary.
func TestTransportReviewsResumeActionsBeforeApplying(t *testing.T) {
	for _, test := range []struct {
		name        string
		action      ActionAction
		conflicts   []Diagnostic
		confirmable bool
	}{
		{name: "repair", action: ActionActionRepair, confirmable: true},
		{name: "refresh projection", action: ActionActionRefreshProjection, confirmable: true},
		{name: "unknown", action: "future_mutation"},
		{name: "conflict", action: ActionActionRepair, conflicts: []Diagnostic{{Code: "FOREIGN_DESTINATION", Severity: Error, Message: "This entry is not owned"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var effects atomic.Int32
			callbacks := testCallbacks(func() Prepared {
				return &reviewedPrepared{testPrepared: &testPrepared{apply: func(context.Context) (Outcome, error) {
					effects.Add(1)
					return Outcome{State: "succeeded"}, nil
				}}, review: Plan{Confirmable: true, Conflicts: test.conflicts, Actions: []Action{{CandidateId: "candidate", TargetId: "codex", Alias: "review", Action: test.action, DestinationPath: "/disposable/skills/review"}}}}
			})
			server, err := Start(context.Background(), Options{Callbacks: callbacks})
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			client := newPickerClient(t, server)
			client.waitReady()
			plan := client.plan()
			if plan.Confirmable != test.confirmable || len(plan.Actions) != 1 || plan.Actions[0].Action != test.action || effects.Load() != 0 {
				t.Fatalf("review lost the action or applied before confirmation: %+v effects=%d", plan, effects.Load())
			}
			status, body := client.request(context.Background(), "POST", "/api/v1/plans/"+plan.PlanId+"/apply", "", nil)
			if test.confirmable {
				if status != 202 {
					t.Fatalf("reviewed resume was refused: %d %s", status, body)
				}
				eventually(t, func() bool { return effects.Load() == 1 })
			} else if status != 409 || effects.Load() != 0 {
				t.Fatalf("unreviewable plan reached apply: %d %s effects=%d", status, body, effects.Load())
			}
		})
	}
}
