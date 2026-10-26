//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Scanner cache entries are authored before any fixture child. All privileged
// writes use a held ordinary parent and a new exclusive directory/file.
func scannerDirectory(path string, uid, gid int) (resultErr error) {
	parent, err := trustedParent(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parent.Close()) }()
	if err := unix.Mkdirat(int(parent.Fd()), filepath.Base(path), 0700); err != nil {
		return err
	}
	return scannerSetDirectory(path, uid, gid, 0700)
}

func scannerSetDirectory(path string, uid, gid int, mode os.FileMode) (resultErr error) {
	parent, err := trustedParent(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parent.Close()) }()
	info, err := anchoredInfo(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("ordinary scanner directory required")
	}
	held, err := trustedOpenHeld(parent, filepath.Base(path), info, true)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, held.Close()) }()
	if err := held.Chown(uid, gid); err != nil {
		return err
	}
	if err := held.Chmod(mode); err != nil {
		return err
	}
	after, err := held.Stat()
	linked, linkErr := anchoredInfo(path)
	if err != nil || linkErr != nil || !sameFile(after, linked) {
		return fmt.Errorf("scanner directory link changed")
	}
	return nil
}

func (c *controller) prepareTrustedScanner() error {
	if c.scannerCache != nil || c.request.Input.Trusted.ScannerArchiveSHA256 != trustedScannerArchive {
		return fmt.Errorf("fresh pinned scanner cache required")
	}
	f := c.trustedState
	state := filepath.Join(f.Root, "installer-state")
	security := filepath.Join(state, "security")
	chain := []string{security, filepath.Join(security, "lintai"), filepath.Join(security, "lintai", "0.1.3"), filepath.Join(security, "lintai", "0.1.3", "linux-amd64-musl")}
	s := &trustedScannerCache{security: security, binary: filepath.Join(chain[3], "lintai"), digest: c.request.Input.Trusted.ScannerSHA256}
	c.scannerCache = s
	if err := c.seedTrustedScanner(chain); err != nil {
		return err
	}
	if err := c.holdTrustedScanner(chain); err != nil {
		return err
	}
	return s.bind()
}

func (c *controller) seedTrustedScanner(chain []string) error {
	f, s := c.trustedState, c.scannerCache
	for _, dir := range chain {
		if err := scannerDirectory(dir, 0, 0); err != nil {
			return err
		}
	}
	assessments := filepath.Join(s.security, "assessments")
	if err := scannerDirectory(assessments, c.request.Input.UID, c.request.Input.GID); err != nil {
		return err
	}
	if err := c.scannerCopy(s.binary); err != nil {
		return err
	}
	for _, scope := range []string{"a", "b"} {
		path := filepath.Join(f.Root, scope+"-events.jsonl")
		if _, err := anchoredInfo(path); !os.IsNotExist(err) {
			return fmt.Errorf("fresh empty event log required")
		}
		if err := c.trustedWrite(path, nil, 0600); err != nil {
			return err
		}
	}
	for _, dir := range chain {
		if err := scannerSetDirectory(dir, 0, 0, 0555); err != nil {
			return err
		}
	}
	if err := scannerSetDirectory(f.Root, 0, 0, 0755); err != nil {
		return err
	}
	return nil
}

func (c *controller) holdTrustedScanner(chain []string) error {
	f, s := c.trustedState, c.scannerCache
	state, assessments := filepath.Dir(s.security), filepath.Join(s.security, "assessments")
	for _, dir := range append([]string{c.request.Base, f.Root}, chain...) {
		mode := os.FileMode(0555)
		if dir == c.request.Base || dir == f.Root {
			mode = 0755
		}
		if err := s.hold(dir, 0, 0, os.ModeDir|mode); err != nil {
			return err
		}
	}
	for _, dir := range []string{state, assessments} {
		if err := s.hold(dir, c.request.Input.UID, c.request.Input.GID, os.ModeDir|0700); err != nil {
			return err
		}
	}
	if err := s.hold(s.binary, 0, 0, 0555); err != nil {
		return err
	}
	var err error
	s.inventory, err = trustedInventory(chain[1])
	if err != nil {
		return err
	}
	return nil
}

func (s *trustedScannerCache) bind() error {
	state, security := filepath.Dir(s.security), s.security
	var err error
	s.parentMount, err = scannerMountID(state)
	if err != nil {
		return err
	}
	before, err := scannerMountID(security)
	if err != nil || before != s.parentMount {
		return fmt.Errorf("scanner directory is already a mountpoint")
	}
	for _, n := range s.nodes {
		mount, err := scannerMountID(n.path)
		if err != nil || mount != s.parentMount {
			return fmt.Errorf("unexpected original scanner ancestor mount")
		}
	}
	// Resolve both operands through the already held task inode. The native
	// state owner cannot race the privileged mount onto a substituted symlink.
	anchor := s.node(security)
	if err := anchor.verify(); err != nil {
		return err
	}
	operand := fmt.Sprintf("/proc/self/fd/%d", anchor.file.Fd())
	if err := unix.Mount(operand, operand, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("exact task scanner bind failed: %w", err)
	}
	s.bound = true
	s.mount, err = scannerMountID(security)
	if err != nil || s.mount == before {
		return fmt.Errorf("new exact scanner mount identity missing")
	}
	for _, n := range s.nodes {
		n.mount = s.parentMount
		if n.path == security || trustedContained(security, n.path) {
			n.mount = s.mount
		}
	}
	s.sealed = true
	return s.verify()
}
