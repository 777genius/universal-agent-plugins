//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const lockMode = "lock-qualification-v1"

func validateResearchMode(r request) error {
	if r.Mode == cliMode {
		if r.Scope != "a" || cliArguments(r) == nil {
			return errors.New("cli-phase-refused")
		}
		return nil
	}
	if r.Mode == lockMode {
		if r.Scope != "a" || r.Phase != "locked-writer" && r.Phase != "released-writer" {
			return errors.New("lock-phase-refused")
		}
		return nil
	}
	if r.Phase != "" {
		return errors.New("unexpected-phase-refused")
	}
	return validateSessionRequest(r)
}

func lockResultFields(mode, original string) string {
	if mode == cliMode {
		return original + " mode phase"
	}
	if mode == lockMode {
		return original + " mode phase lock_held"
	}
	return original
}

func validateLockResult(got result, r request) error {
	if r.Mode == cliMode {
		return validateCLIResult(got, r)
	}
	if r.Mode != lockMode {
		if got.Phase != "" || got.LockHeld != nil {
			return errors.New("unexpected-lock-proof")
		}
		return nil
	}
	if got.Mode != lockMode || got.Phase != r.Phase || got.LockHeld == nil ||
		got.Complete && (*got.LockHeld != (r.Phase == "locked-writer") || len(got.Commands) != 4) {
		return errors.New("lock-proof-refused")
	}
	return nil
}

// Only the supervisor owns this lock; killing the collector cannot release it.
type lockAuthority struct {
	root    *directoryAuthority
	parents []*directoryAuthority
	file    heldFile
}

func (s *scopeState) prepareLock(r request) error {
	if r.Mode != lockMode {
		return nil
	}
	if e := validateResearchMode(r); e != nil {
		return e
	}
	s.lock = &lockAuthority{root: s.input}
	parent := s.input
	for _, name := range []string{"home", ".grok"} {
		child, e := holdLockDirectory(parent, name, r.UID, r.GID)
		if e != nil {
			return e
		}
		s.lock.parents = append(s.lock.parents, child)
		parent = child
	}
	fd, e := unix.Openat(int(parent.file.Fd()), ".config-init.lock", unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if e != nil {
		return e
	}
	s.lock.file = heldFile{file: os.NewFile(uintptr(fd), "grok-config-lock"), path: filepath.Join(parent.path, ".config-init.lock")}
	info, e := s.lock.file.file.Stat()
	if e != nil {
		return e
	}
	id, e := identity(info)
	if e != nil || id.Mode != unix.S_IFREG|0600 || int64(id.UID) != int64(r.UID) || int64(id.GID) != int64(r.GID) || id.Links != 1 || id.Size != 0 || id.Dev != parent.dev {
		if e != nil {
			return errors.New("lock-file-authority-refused")
		}
		return fmt.Errorf("lock-file-authority-refused: regular=%t mode=%#o uidgidmatch=%t nlink=%d size=%d deviceMatch=%t",
			id.Mode&unix.S_IFMT == unix.S_IFREG, id.Mode, int64(id.UID) == int64(r.UID) && int64(id.GID) == int64(r.GID), id.Links, id.Size, id.Dev == parent.dev)
	}
	s.lock.file.id = id
	if e = s.lock.verify(); e != nil {
		return e
	}
	if r.Phase == "locked-writer" {
		if e = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); e != nil {
			return e
		}
	}
	return s.lock.verify()
}

func holdLockDirectory(parent *directoryAuthority, name string, uid, gid int) (*directoryAuthority, error) {
	if parent == nil {
		return nil, errors.New("lock-parent-missing")
	}
	if e := parent.verify(); e != nil {
		return nil, e
	}
	fd, e := unix.Openat(int(parent.file.Fd()), name, unix.O_PATH|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	file := os.NewFile(uintptr(fd), name)
	fail := func(e error) (*directoryAuthority, error) { return nil, errors.Join(e, file.Close()) }
	info, e := file.Stat()
	if e != nil {
		return fail(e)
	}
	id, e := identity(info)
	if e != nil || id.Dev != parent.dev || info.Mode().Perm()&022 != 0 {
		return fail(errors.New("lock-parent-authority-refused"))
	}
	mount, e := mountID(fd, "")
	if e != nil || mount != parent.mount {
		return fail(errors.New("lock-parent-mount-refused"))
	}
	child := &directoryAuthority{file, filepath.Join(parent.path, name), id.Dev, id.Ino, mount, info.Mode(), uid, gid}
	if e = errors.Join(parent.verify(), child.verify()); e != nil {
		return fail(e)
	}
	return child, nil
}

func (lock *lockAuthority) verify() error {
	if lock == nil {
		return nil
	}
	if e := lock.root.verify(); e != nil {
		return e
	}
	for _, parent := range lock.parents {
		if e := parent.verify(); e != nil {
			return e
		}
	}
	return lock.file.unchanged()
}

func (s *scopeState) verifyRoots() error {
	if e := s.input.verify(); e != nil {
		return e
	}
	if e := s.directory.verify(); e != nil {
		return e
	}
	return s.lock.verify()
}

func (lock *lockAuthority) close() error {
	if lock == nil {
		return nil
	}
	var e error
	if lock.file.file != nil {
		e = lock.file.file.Close()
	}
	for _, parent := range lock.parents {
		e = errors.Join(e, parent.file.Close())
	}
	return e
}
