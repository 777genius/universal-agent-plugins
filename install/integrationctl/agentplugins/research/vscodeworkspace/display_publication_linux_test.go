//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func displayPublicationFixture(t *testing.T) (string, string, isolationProof, []byte) {
	t.Helper()
	root := t.TempDir()
	lock, temp := filepath.Join(root, ".X93-lock"), filepath.Join(root, ".tX93-lock")
	proof := isolationProof{UID: os.Getuid(), PID: os.Getpid()}
	body := []byte(fmt.Sprintf("%10d\n", proof.PID))
	if err := os.WriteFile(temp, body, 0444); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(temp, lock); err != nil {
		t.Fatal(err)
	}
	return lock, temp, proof, body
}
func TestDisplayPublicationWaitsForRealTwoLinkToSingleLink(t *testing.T) {
	lock, temp, proof, body := displayPublicationFixture(t)
	publication := &displayLockPublication{}
	defer func() {
		if err := publication.close(); err != nil {
			t.Error(err)
		}
	}()
	ready, err := publication.ready(lock, temp, proof)
	if err != nil || ready || publication.file == nil {
		t.Fatal("two-link object admitted or not held", ready, err)
	}
	flags, err := unix.FcntlInt(publication.file.Fd(), unix.F_GETFL, 0)
	if err != nil || flags&unix.O_PATH == 0 {
		t.Fatal("publication handle can read aliased bytes", err)
	}
	if err = os.Remove(temp); err != nil {
		t.Fatal(err)
	}
	if ready, err = publication.ready(lock, temp, proof); err != nil || !ready {
		t.Fatal("completed publication refused", ready, err)
	}
	got, err := readOrdinary(lock, 64)
	if err != nil || string(got) != string(body) {
		t.Fatal("final strict read differs", err)
	}
}
func TestDisplayPublicationRefusesUnknownLinksMetadataAndReplacement(t *testing.T) {
	for _, kind := range []string{"owner", "mode", "size", "third-alias", "missing-temp", "foreign-temp", "symlink-temp", "replacement"} {
		t.Run(kind, func(t *testing.T) {
			lock, temp, proof, _ := displayPublicationFixture(t)
			publication := &displayLockPublication{}
			defer func() {
				if err := publication.close(); err != nil {
					t.Error(err)
				}
			}()
			var err error
			switch kind {
			case "owner":
				proof.UID++
			case "mode":
				err = os.Chmod(temp, 0600)
			case "size":
				if err = os.Chmod(temp, 0600); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(temp, []byte("short"), 0600); err != nil {
					t.Fatal(err)
				}
				err = os.Chmod(temp, 0444)
			case "third-alias":
				err = os.Link(lock, filepath.Join(filepath.Dir(lock), "foreign"))
			case "missing-temp":
				err = os.Rename(temp, temp+"-foreign")
			case "foreign-temp", "symlink-temp":
				if err = os.Rename(temp, temp+"-foreign"); err != nil {
					t.Fatal(err)
				}
				if kind == "foreign-temp" {
					err = os.WriteFile(temp, []byte("foreign"), 0444)
				} else {
					err = os.Symlink(lock, temp)
				}
			case "replacement":
				if ready, e := publication.ready(lock, temp, proof); e != nil || ready {
					t.Fatal("initial pair differs", e)
				}
				if err = os.Remove(lock); err != nil {
					t.Fatal(err)
				}
				err = os.WriteFile(lock, []byte(fmt.Sprintf("%10d\n", proof.PID)), 0444)
			}
			if err != nil {
				t.Fatal(err)
			}
			if ready, err := publication.ready(lock, temp, proof); err == nil || ready {
				t.Fatal("unknown publication admitted", kind, ready, err)
			}
		})
	}
}
func TestDisplayPublicationPersistentPairRefusesOriginalDeadlineAndCancellation(t *testing.T) {
	for _, expired := range []bool{false, true} {
		lock, temp, proof, _ := displayPublicationFixture(t)
		publication := &displayLockPublication{}
		ctx, cancel := context.WithCancel(t.Context())
		deadline := make(chan time.Time)
		if expired {
			close(deadline)
		} else {
			cancel()
		}
		called := false
		err := waitDisplayReadiness(ctx, deadline, nil, func() (bool, error) { called = true; return publication.ready(lock, temp, proof) })
		cancel()
		if err == nil || called {
			t.Fatal("expired original readiness budget performed observation", expired, err)
		}
		if err := publication.close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDisplayPublicationDeadlineDuringFinalObservationStillRefuses(t *testing.T) {
	lock, temp, proof, _ := displayPublicationFixture(t)
	publication := &displayLockPublication{}
	defer func() {
		if err := publication.close(); err != nil {
			t.Error(err)
		}
	}()
	if err := os.Remove(temp); err != nil {
		t.Fatal(err)
	}
	deadline := make(chan time.Time)
	err := waitDisplayReadiness(t.Context(), deadline, nil, func() (bool, error) {
		ready, err := publication.ready(lock, temp, proof)
		close(deadline)
		return ready, err
	})
	if err == nil {
		t.Fatal("final observation renewed original readiness budget")
	}
}

func TestDisplayFreshnessRefusesEitherLockWithoutMutatingIt(t *testing.T) {
	root := t.TempDir()
	lock, temp, socket := filepath.Join(root, "lock"), filepath.Join(root, "temp"), filepath.Join(root, "socket")
	if err := requireFreshDisplayLocks(lock, temp, socket); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{lock, temp, socket} {
		if err := os.WriteFile(path, []byte("foreign"), 0600); err != nil {
			t.Fatal(err)
		}
		if requireFreshDisplayLocks(lock, temp, socket) == nil {
			t.Fatal("preexisting display asset admitted", path)
		}
		body, err := os.ReadFile(path)
		if err != nil || string(body) != "foreign" {
			t.Fatal("foreign asset changed", err)
		}
		if err = os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

type displayPublicationSpecialMode struct{ os.FileInfo }

func (info displayPublicationSpecialMode) Mode() os.FileMode {
	return info.FileInfo.Mode() | os.ModeSetuid
}
func TestDisplayPublicationRefusesSpecialModeBitsWithoutPermissionChanges(t *testing.T) {
	_, temp, proof, _ := displayPublicationFixture(t)
	info, err := os.Lstat(temp)
	if err != nil {
		t.Fatal(err)
	}
	if transientDisplayLock(displayPublicationSpecialMode{info}, proof) {
		t.Fatal("unknown special mode admitted")
	}
}

func TestDisplayPublicationFinalReadRetainsOriginalMetadata(t *testing.T) {
	lock, temp, proof, _ := displayPublicationFixture(t)
	publication := &displayLockPublication{}
	defer func() {
		if err := publication.close(); err != nil {
			t.Error(err)
		}
	}()
	if err := os.Remove(temp); err != nil {
		t.Fatal(err)
	}
	if ready, err := publication.ready(lock, temp, proof); err != nil || !ready {
		t.Fatal(ready, err)
	}
	original := publication.original
	changed := original.ModTime().Add(-time.Hour)
	if err := os.Chtimes(lock, changed, changed); err != nil {
		t.Fatal(err)
	}
	named, err := os.Lstat(lock)
	if err != nil {
		t.Fatal(err)
	}
	if named.ModTime().Equal(original.ModTime()) {
		t.Fatal("fixture mtime did not change")
	}
	if err := publication.finalInfo(named); err == nil {
		t.Fatal("post-ready metadata mutation admitted")
	}
}

func TestDisplayPublicationPendingUnlinkAcceptsOnlyMatchingInodeAsNotReady(t *testing.T) {
	lock, temp, proof, body := displayPublicationFixture(t)
	two, err := os.Lstat(lock)
	if err != nil || !pendingDisplayLock(two, two, proof) {
		t.Fatal("original two-link transition not recognized", err)
	}
	if err := os.Remove(temp); err != nil {
		t.Fatal(err)
	}
	one, err := os.Lstat(lock)
	if err != nil || !pendingDisplayLock(one, one, proof) {
		t.Fatal("same-inode single-link transitional snapshot rejected", err)
	}
	foreign := filepath.Join(filepath.Dir(lock), "foreign-lock")
	if err := os.WriteFile(foreign, body, 0444); err != nil {
		t.Fatal(err)
	}
	other, err := os.Lstat(foreign)
	if err != nil || pendingDisplayLock(one, other, proof) {
		t.Fatal("foreign same-size temporary lock admitted", err)
	}
}
