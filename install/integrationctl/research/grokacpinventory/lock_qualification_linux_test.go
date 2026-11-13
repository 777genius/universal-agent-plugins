//go:build linux

package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestLockQualificationIsFixedScopeAndHistoricalMode(t *testing.T) {
	r := inertRequest()
	r.Mode, r.Phase = lockMode, "locked-writer"
	if e := validateResearchMode(r); e != nil {
		t.Fatal(e)
	}
	for _, phase := range []string{"released-writer", "", "append", "PRIVATE"} {
		r.Phase = phase
		e := validateResearchMode(r)
		if (e == nil) != (phase == "released-writer") {
			t.Fatal("closed phase contract")
		}
	}
	r.Phase, r.Scope = "locked-writer", "b"
	if validateResearchMode(r) == nil {
		t.Fatal("different scope admitted")
	}
	r.Mode, r.Scope = "", "a"
	if validateResearchMode(r) == nil {
		t.Fatal("historical mode accepted phase")
	}
}

func TestLockQualificationResultPhaseAndAcquisitionProof(t *testing.T) {
	r := inertRequest()
	r.Mode, r.Phase = lockMode, "locked-writer"
	held := true
	got := result{Mode: lockMode, Phase: r.Phase, LockHeld: &held, Complete: true,
		Commands: make([]json.RawMessage, 4)}
	if e := validateLockResult(got, r); e != nil {
		t.Fatal(e)
	}
	for _, change := range []string{"phase", "missing", "unheld", "commands"} {
		bad := got
		switch change {
		case "phase":
			bad.Phase = "released-writer"
		case "missing":
			bad.LockHeld = nil
		case "unheld":
			value := false
			bad.LockHeld = &value
		case "commands":
			bad.Commands = nil
		}
		if validateLockResult(bad, r) == nil {
			t.Fatal("unproved locked completion admitted")
		}
	}
	got.Complete = false
	held = false
	if e := validateLockResult(got, r); e != nil {
		t.Fatal("honest pre-acquisition failure refused")
	}
}

func TestLockQualificationUsesUnchangedUnprivilegedCollectorArgv(t *testing.T) {
	systems := make(map[string]systemFact)
	for _, name := range []string{"unshare", "setpriv", "env", "python3"} {
		systems[name] = systemFact{"/usr/bin/" + name, strings.Repeat("a", 64)}
	}
	r := inertRequest()
	ordinary, e := fixedCommand("/tmp/sealed", "/tmp/input", r, systems)
	if e != nil {
		t.Fatal(e)
	}
	r.Mode, r.Phase = lockMode, "locked-writer"
	locked, e := fixedCommand("/tmp/sealed", "/tmp/input", r, systems)
	if e != nil || !reflect.DeepEqual(ordinary, locked) {
		t.Fatal("lock mode changed privilege drop, environment or collector command")
	}
}
