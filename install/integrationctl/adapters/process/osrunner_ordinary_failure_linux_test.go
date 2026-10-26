//go:build linux

package process

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func supervisorStatusFixture(t *testing.T, body []byte) *commandContainment {
	t.Helper()
	status, statusWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = status.Close() })
	if _, err := statusWriter.Write(body); err != nil {
		_ = statusWriter.Close()
		t.Fatal(err)
	}
	if err := statusWriter.Close(); err != nil {
		t.Fatal(err)
	}
	request, requestWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = request.Close(); _ = requestWriter.Close() })
	return &commandContainment{
		supervisorStatus: status, supervisorRequest: requestWriter,
		observeExit: func() (bool, error) { return true, nil },
	}
}

func TestOrdinarySupervisorFailureStatusPreservesMessageAndClass(t *testing.T) {
	for _, test := range []struct{ stage, class string }{
		{ordinarySupervisorRequestDecode, "ordinary_supervisor_request_decode"},
		{ordinarySupervisorSubreaperSetup, "ordinary_supervisor_subreaper_setup"},
		{ordinarySupervisorPreflight, "ordinary_supervisor_preflight"},
		{ordinarySupervisorCommandStart, "ordinary_supervisor_command_start"},
		{ordinarySupervisorDescendantCleanup, "ordinary_supervisor_descendant_cleanup"},
		{"", ""},
		{"future_stage", ""},
	} {
		t.Run(test.stage, func(t *testing.T) {
			const message = "original failure: argument and stream content\nsecond line"
			for _, forced := range []bool{false, true} {
				body, err := json.Marshal(linuxOrdinarySupervisorResult{Forced: forced, Error: message, FailureStage: test.stage})
				if err != nil {
					t.Fatal(err)
				}
				containment := supervisorStatusFixture(t, body)
				result := containment.terminateSupervisor()
				if result.err == nil || result.err.Error() != message {
					t.Fatalf("wire failure did not preserve exact message: %v", result.err)
				}
				if got := OrdinarySupervisorFailureClass(result.err); got != test.class {
					t.Fatalf("wire class = %q, want %q", got, test.class)
				}
				if !result.leaderStopped || result.leaderTerminationInitiated || result.forcedMembers != forced {
					t.Fatalf("diagnostic changed termination facts: %+v", result)
				}
				if !containment.supervisorRead || containment.supervisorStatus != nil || containment.supervisorRequest != nil {
					t.Fatal("status receiver retained descriptor authority")
				}
			}
		})
	}
}

func TestOrdinarySupervisorFailureStageCannotAdmitSuccess(t *testing.T) {
	for _, stage := range []string{ordinarySupervisorPreflight, "future_stage"} {
		body, err := json.Marshal(linuxOrdinarySupervisorResult{FailureStage: stage})
		if err != nil {
			t.Fatal(err)
		}
		result := supervisorStatusFixture(t, body).terminateSupervisor()
		if result.err == nil || !strings.Contains(result.err.Error(), "failure stage without an error") {
			t.Fatalf("contradictory status admitted: %v", result.err)
		}
		if got := OrdinarySupervisorFailureClass(result.err); got != "" {
			t.Fatalf("contradiction invented failure class %q", got)
		}
	}
	for _, body := range []string{`{"Forced":false}`, `{"Forced":false,"Error":""}`, `{"Forced":false,"Error":"","FailureStage":""}`} {
		if result := supervisorStatusFixture(t, []byte(body)).terminateSupervisor(); result.err != nil {
			t.Fatalf("unchanged success status failed: %v", result.err)
		}
	}
}

func TestOrdinarySupervisorFailureStageRetainsStrictWireAdmission(t *testing.T) {
	for _, body := range []string{
		"",
		`{"Forced":false,"FailureStage":"preflight","Unexpected":true}`,
		`{"Forced":false,"Error":"original","FailureStage":"preflight","Unexpected":true}`,
		`{"Forced":false}{"Forced":false}`,
		`{"Forced":false,"Error":"original","FailureStage":"preflight"}{"Forced":false}`,
		`{"Forced":false,"FailureStage":42}`,
	} {
		containment := supervisorStatusFixture(t, []byte(body))
		result := containment.terminateSupervisor()
		if err := result.err; err == nil || errors.Is(err, io.EOF) {
			t.Fatalf("invalid status admitted or lost explicit cleanup failure: %v", err)
		}
		if got := OrdinarySupervisorFailureClass(result.err); got != "" {
			t.Fatalf("unadmitted status established class %q", got)
		}
	}
}
