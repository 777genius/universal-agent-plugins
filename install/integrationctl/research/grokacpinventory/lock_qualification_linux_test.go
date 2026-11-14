//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
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

func supervisorLockFixture(t *testing.T, phase string) (*scopeState, request, string) {
	t.Helper()
	// Hosted overlay filesystems can report different devices for newly created
	// regular files and their directories. Exercise the real same-device lock
	// authority on a private Linux tmpfs root; never relax the production guard.
	root, err := os.MkdirTemp("/dev/shm", "uap-grok-lock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(root, "home", ".grok", ".config-init.lock")
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, nil, 0600); e != nil {
		t.Fatal(e)
	}
	r := inertRequest()
	r.UID, r.GID, r.Mode, r.Phase = os.Getuid(), os.Getgid(), lockMode, phase
	input, e := holdDirectory(root, r.UID, r.GID)
	if e != nil {
		t.Fatal(e)
	}
	s := &scopeState{input: input}
	t.Cleanup(func() {
		if s.input != nil {
			if e := s.close(); e != nil {
				t.Error(e)
			}
		}
	})
	return s, r, path
}

func assertIndependentFlock(t *testing.T, path string, blocked bool) {
	t.Helper()
	file, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = file.Close() }()
	e = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if blocked {
		if !errors.Is(e, unix.EWOULDBLOCK) {
			t.Fatalf("independent lock was not blocked: %v", e)
		}
	} else if e != nil {
		t.Fatal(e)
	}
}

func TestSupervisorLockOutlivesCollectorExchangeAndCleanup(t *testing.T) {
	s, r, path := supervisorLockFixture(t, "locked-writer")
	if e := s.prepareLock(r); e != nil {
		logSupervisorLockAuthority(t, s, r, path)
		t.Fatal(e)
	}
	var result map[string]any
	if e := json.Unmarshal(inertResult(t, r), &result); e != nil {
		t.Fatal(e)
	}
	result["mode"], result["phase"], result["lock_held"] = lockMode, r.Phase, true
	body, e := json.Marshal(result)
	if e != nil {
		t.Fatal(e)
	}
	body = append(body, '\n')
	cleanupReturned := false
	raw, e := supervisedCapture(context.Background(), ports.Command{}, r, 1,
		func(_ context.Context, _ ports.Command, exchange func(io.Writer, io.Reader) error) error {
			if e := exchange(io.Discard, bytes.NewReader(body)); e != nil {
				return e
			}
			// Collector output has reached EOF; family cleanup has not returned.
			assertIndependentFlock(t, path, true)
			cleanupReturned = true
			return nil
		})
	if e != nil || !cleanupReturned || !bytes.Equal(raw, body) {
		t.Fatalf("capture did not reach the cleanup boundary: %v", e)
	}
	if e = s.lock.verify(); e != nil {
		t.Fatal(e)
	}
	assertIndependentFlock(t, path, true)
	if e = s.close(); e != nil {
		t.Fatal(e)
	}
	s.input, s.lock = nil, nil
	assertIndependentFlock(t, path, false)
}

func TestSupervisorLockPhaseAndReplacedParents(t *testing.T) {
	s, r, path := supervisorLockFixture(t, "released-writer")
	if e := s.prepareLock(r); e != nil {
		logSupervisorLockAuthority(t, s, r, path)
		t.Fatal(e)
	}
	assertIndependentFlock(t, path, false)
	locked, request, original := supervisorLockFixture(t, "locked-writer")
	if e := locked.prepareLock(request); e != nil {
		logSupervisorLockAuthority(t, locked, request, original)
		t.Fatal(e)
	}
	parent := filepath.Dir(original)
	if e := os.Rename(parent, parent+"-retained"); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(parent, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(original, nil, 0600); e != nil {
		t.Fatal(e)
	}
	if locked.lock.verify() == nil {
		t.Fatal("replacement parent granted lock authority")
	}
	assertIndependentFlock(t, filepath.Join(parent+"-retained", ".config-init.lock"), true)
	assertIndependentFlock(t, original, false)
}

func TestSupervisorLockRefusesForeignAndMalformedLeaves(t *testing.T) {
	for _, fault := range []string{"busy", "symlink", "hardlink", "bytes", "mode", "owner"} {
		t.Run(fault, func(t *testing.T) {
			s, r, path := supervisorLockFixture(t, "locked-writer")
			var e error
			switch fault {
			case "busy":
				var file *os.File
				file, e = os.Open(path)
				if e == nil {
					defer func() { _ = file.Close() }()
					e = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
				}
			case "symlink":
				e = os.Rename(path, path+"-retained")
				if e == nil {
					e = os.Symlink(path+"-retained", path)
				}
			case "hardlink":
				e = os.Link(path, path+"-alias")
			case "bytes":
				e = os.WriteFile(path, []byte("foreign"), 0600)
			case "mode":
				e = os.Chmod(path, 0640)
			case "owner":
				r.UID++
			}
			if e != nil {
				t.Fatal(e)
			}
			if s.prepareLock(r) == nil {
				t.Fatal("unowned lock admitted")
			}
		})
	}
}

func logSupervisorLockAuthority(t *testing.T, s *scopeState, r request, path string) {
	t.Helper()
	t.Logf("diagnostic request UID=%d GID=%d phase=%s expectedMode=%#o", r.UID, r.GID, r.Phase, unix.S_IFREG|0600)
	named, err := os.Lstat(path)
	if err != nil {
		t.Logf("diagnostic named stat error=%v", err)
	} else {
		id, identityErr := identity(named)
		t.Logf("diagnostic named sys=%T mode=%v identity=%+v identityError=%v", named.Sys(), named.Mode(), id, identityErr)
	}
	if s.lock == nil {
		return
	}
	if s.lock.file.file != nil {
		held, statErr := s.lock.file.file.Stat()
		if statErr != nil {
			t.Logf("diagnostic held leaf stat error=%v", statErr)
		} else {
			id, identityErr := identity(held)
			t.Logf("diagnostic held leaf sys=%T mode=%v identity=%+v identityError=%v", held.Sys(), held.Mode(), id, identityErr)
		}
	}
	for index, parent := range s.lock.parents {
		held, statErr := parent.file.Stat()
		if statErr != nil {
			t.Logf("diagnostic parent%d stat error=%v", index, statErr)
		} else {
			id, identityErr := identity(held)
			t.Logf("diagnostic parent%d expectedDev=%d expectedUID=%d expectedGID=%d expectedMode=%v sys=%T identity=%+v identityError=%v", index, parent.dev, parent.uid, parent.gid, parent.mode, held.Sys(), id, identityErr)
		}
	}
}
