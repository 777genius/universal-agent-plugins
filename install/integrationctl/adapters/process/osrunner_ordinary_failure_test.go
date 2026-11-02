package process

import (
	"errors"
	"testing"
)

type supervisorDiagnosticTextTrap struct{}

func (supervisorDiagnosticTextTrap) Error() string {
	panic("structural supervisor diagnostics must not read error text")
}

func TestOrdinaryForcedCleanupPreservesMessageAndIdentity(t *testing.T) {
	const original = "process left live descendants that required forced cleanup"
	if ErrOrdinaryForcedCleanup.Error() != original {
		t.Fatal("forced cleanup changed its existing error text")
	}
	err := errors.Join(ErrOrdinaryForcedCleanup, supervisorDiagnosticTextTrap{})
	if !errors.Is(err, ErrOrdinaryForcedCleanup) || OrdinarySupervisorFailureClass(err) != "" {
		t.Fatal("ordinary forced cleanup lost its distinct structural identity")
	}
}

func TestOrdinarySupervisorFailureClassNeverReadsErrorText(t *testing.T) {
	for _, test := range []struct{ stage, class string }{
		{ordinarySupervisorRequestDecode, "ordinary_supervisor_request_decode"},
		{ordinarySupervisorSubreaperSetup, "ordinary_supervisor_subreaper_setup"},
		{ordinarySupervisorPreflight, "ordinary_supervisor_preflight"},
		{ordinarySupervisorCommandStart, "ordinary_supervisor_command_start"},
		{ordinarySupervisorDescendantCleanup, "ordinary_supervisor_descendant_cleanup"},
		{"", ""},
		{"unrecognized stage with sensitive text", ""},
	} {
		t.Run(test.stage, func(t *testing.T) {
			cause := supervisorDiagnosticTextTrap{}
			failure := &ordinarySupervisorFailure{cause: cause, stage: test.stage}
			err := errors.Join(errors.New("outer cause"), failure)
			if got := OrdinarySupervisorFailureClass(err); got != test.class {
				t.Fatalf("class = %q, want %q", got, test.class)
			}
			if !errors.Is(err, cause) || !errors.Is(errors.Unwrap(failure), cause) {
				t.Fatal("classification carrier lost its original cause")
			}
		})
	}
	for _, err := range []error{nil, supervisorDiagnosticTextTrap{}, (*ordinarySupervisorFailure)(nil)} {
		if got := OrdinarySupervisorFailureClass(err); got != "" {
			t.Fatalf("unrelated error received class %q", got)
		}
	}
}
