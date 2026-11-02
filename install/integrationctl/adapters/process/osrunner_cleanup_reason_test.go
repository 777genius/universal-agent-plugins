package process

import (
	"errors"
	"reflect"
	"testing"
)

var cleanupReasonFixtures = []string{
	"exit_observation", "task_directory_read", "task_identity_unavailable", "task_children_observation",
	"child_identity_read", "child_identity_changed", "pidfd_open", "pidfd_identity_read", "pidfd_identity_changed",
	"parent_identity_read", "parent_changed", "tracked_identity_read", "tracked_identity_changed",
	"traversal_deadline", "traversal_limit", "descendant_signal", "cleanup_deadline",
}

func TestOrdinaryCleanupReasonClassIsClosedAndNeverReadsError(t *testing.T) {
	for _, reason := range cleanupReasonFixtures {
		t.Run(reason, func(t *testing.T) {
			cause := &supervisorDiagnosticTextTrap{}
			failure := &ordinarySupervisorFailure{cause: cause, stage: ordinarySupervisorDescendantCleanup, reason: reason}
			joined := errors.Join(errors.New("earlier cause"), failure)
			if got := OrdinarySupervisorFailureClass(joined); got != "ordinary_supervisor_descendant_cleanup_"+reason {
				t.Fatalf("cleanup class = %q", got)
			}
			if reflect.ValueOf(errors.Unwrap(failure)) != reflect.ValueOf(cause) || !errors.Is(joined, cause) {
				t.Fatal("reason metadata changed original cause identity")
			}
		})
	}
	for _, tc := range []struct{ stage, reason, want string }{
		{ordinarySupervisorDescendantCleanup, "", "ordinary_supervisor_descendant_cleanup"},
		{ordinarySupervisorDescendantCleanup, "secret unknown reason", "ordinary_supervisor_descendant_cleanup"},
		{ordinarySupervisorPreflight, "parent_changed", ""},
		{"", "parent_changed", ""},
	} {
		failure := &ordinarySupervisorFailure{cause: supervisorDiagnosticTextTrap{}, stage: tc.stage, reason: tc.reason}
		if got := OrdinarySupervisorFailureClass(failure); got != tc.want {
			t.Fatalf("invalid/legacy reason class = %q, want %q", got, tc.want)
		}
	}
}

func TestOrdinaryCleanupReasonConstructorPreservesExactMessage(t *testing.T) {
	const message = "original supervised failure\nsecond line"
	for _, reason := range append(append([]string{}, cleanupReasonFixtures...), "", "unknown reason") {
		failure := ordinarySupervisorErrorWithReason(ordinarySupervisorDescendantCleanup, reason, message)
		if failure.Error() != message || errors.Unwrap(failure).Error() != message {
			t.Fatal("reason changed original error string")
		}
		want := "ordinary_supervisor_descendant_cleanup"
		if ordinarySupervisorCleanupReason(reason) != "" {
			want += "_" + reason
		}
		if got := OrdinarySupervisorFailureClass(failure); got != want {
			t.Fatalf("constructed class = %q, want %q", got, want)
		}
	}
	for _, stage := range []string{ordinarySupervisorPreflight, "", "unknown stage"} {
		failure := ordinarySupervisorErrorWithReason(stage, "parent_changed", message)
		if failure.Error() != message || OrdinarySupervisorFailureClass(failure) != "" {
			t.Fatal("contradictory stage/reason established diagnostic authority")
		}
	}
}
