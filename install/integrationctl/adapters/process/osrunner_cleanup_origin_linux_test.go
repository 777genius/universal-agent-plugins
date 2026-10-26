//go:build linux

package process

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestOrdinaryCleanupOriginPreservesExactCauseAndFirstJoinedReason(t *testing.T) {
	original := errors.New("exact original cleanup failure")
	for _, reason := range cleanupReasonFixtures {
		failure := withOrdinaryCleanupOrigin(reason, original)
		if failure.Error() != original.Error() || reflect.ValueOf(errors.Unwrap(failure)) != reflect.ValueOf(original) || !errors.Is(failure, original) || ordinaryCleanupFailureReason(failure) != reason {
			t.Fatal("origin did not preserve exact message/cause/identity")
		}
	}
	first := withOrdinaryCleanupOrigin("parent_changed", supervisorDiagnosticTextTrap{})
	joined := errors.Join(errors.New("earlier unclassified cause"), first, withOrdinaryCleanupOrigin("pidfd_open", original))
	if got := withOrdinaryCleanupOrigin("task_children_observation", joined); reflect.ValueOf(got) != reflect.ValueOf(joined) || ordinaryCleanupFailureReason(got) != "parent_changed" {
		t.Fatal("broader annotation replaced first recognized joined origin")
	}
	for _, cause := range []error{nil, os.ErrNotExist, ErrOrdinaryContainmentUnsupported} {
		if got := withOrdinaryCleanupOrigin("task_directory_read", cause); reflect.ValueOf(got) != reflect.ValueOf(cause) || ordinaryCleanupFailureReason(got) != "" {
			t.Fatal("nil/gone/unsupported outcome changed")
		}
	}
	trap := &supervisorDiagnosticTextTrap{}
	if got := withOrdinaryCleanupOrigin("unknown secret reason", trap); reflect.ValueOf(got) != reflect.ValueOf(trap) || ordinaryCleanupFailureReason(got) != "" {
		t.Fatal("unknown origin gained metadata or read error text")
	}
}

func TestOrdinaryCleanupProducerReceiverPreservesEveryReasonAndFact(t *testing.T) {
	for _, reason := range cleanupReasonFixtures {
		t.Run(reason, func(t *testing.T) {
			const message = "original cleanup message\nsecond line"
			for _, forced := range []bool{false, true} {
				wire := linuxOrdinarySupervisorResult{Forced: forced}
				setOrdinarySupervisorCleanupFailure(&wire, withOrdinaryCleanupOrigin(reason, errors.New(message)))
				if wire.Error != message || wire.FailureStage != ordinarySupervisorDescendantCleanup || wire.FailureReason != reason {
					t.Fatal("producer changed message/stage/reason")
				}
				body, err := json.Marshal(wire)
				if err != nil {
					t.Fatal(err)
				}
				containment := supervisorStatusFixture(t, body)
				result := containment.terminateSupervisor()
				if result.err == nil || result.err.Error() != message || OrdinarySupervisorFailureClass(result.err) != "ordinary_supervisor_descendant_cleanup_"+reason {
					t.Fatal("strict wire lost exact cleanup cause/category")
				}
				if !result.leaderStopped || result.leaderTerminationInitiated || result.forcedMembers != forced || containment.supervisorStatus != nil || containment.supervisorRequest != nil {
					t.Fatal("metadata changed termination facts or descriptor authority")
				}
			}
		})
	}
	unchanged := linuxOrdinarySupervisorResult{Forced: true, Error: "retained", FailureStage: "retained"}
	before := unchanged
	setOrdinarySupervisorCleanupFailure(&unchanged, nil)
	if unchanged != before {
		t.Fatal("nil cleanup changed existing result")
	}
}

func TestOrdinaryCleanupReasonWireRejectsContradictionsAndUnadmittedRecords(t *testing.T) {
	for _, wire := range []linuxOrdinarySupervisorResult{
		{FailureReason: "parent_changed"},
		{FailureStage: ordinarySupervisorDescendantCleanup, FailureReason: "parent_changed"},
		{Error: "original", FailureReason: "parent_changed"},
		{Error: "original", FailureStage: ordinarySupervisorPreflight, FailureReason: "parent_changed"},
		{Error: "original", FailureStage: "unknown stage", FailureReason: "parent_changed"},
	} {
		body, err := json.Marshal(wire)
		if err != nil {
			t.Fatal(err)
		}
		result := supervisorStatusFixture(t, body).terminateSupervisor()
		if result.err == nil || OrdinarySupervisorFailureClass(result.err) != "" {
			t.Fatal("contradictory cleanup metadata established a class or success")
		}
	}
	for _, reason := range []string{"", "unknown secret reason"} {
		body, err := json.Marshal(linuxOrdinarySupervisorResult{Error: "original", FailureStage: ordinarySupervisorDescendantCleanup, FailureReason: reason})
		if err != nil {
			t.Fatal(err)
		}
		result := supervisorStatusFixture(t, body).terminateSupervisor()
		if result.err == nil || result.err.Error() != "original" || OrdinarySupervisorFailureClass(result.err) != "ordinary_supervisor_descendant_cleanup" {
			t.Fatal("legacy/unknown reason lost generic cleanup failure")
		}
	}
	for _, body := range []string{
		`{"Error":"original","FailureStage":"descendant_cleanup","FailureReason":"parent_changed","Unknown":true}`,
		`{"Error":"original","FailureStage":"descendant_cleanup","FailureReason":"parent_changed"}{}`,
		`{"Error":"original","FailureStage":"descendant_cleanup","FailureReason":42}`,
		`{"Error":"original","FailureStage":"descendant_cleanup","FailureReason":"parent_changed"`,
	} {
		result := supervisorStatusFixture(t, []byte(body)).terminateSupervisor()
		if result.err == nil || OrdinarySupervisorFailureClass(result.err) != "" {
			t.Fatal("unadmitted record established cleanup class/reason")
		}
	}
}
