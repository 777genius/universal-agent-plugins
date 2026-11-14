//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"

	"golang.org/x/sys/unix"
)

type trustedScannerNode struct {
	path  string
	file  *os.File
	info  os.FileInfo
	mount uint64
}

// Directory entries and timestamps may change during ordinary state/cache
// writes. Their original inode, ownership and boundary mode may not change.
func (n *trustedScannerNode) verify() error {
	held, err := n.file.Stat()
	linked, linkErr := anchoredInfo(n.path)
	if err != nil || linkErr != nil || !scannerNodeIdentity(n.info, held) || !scannerNodeIdentity(n.info, linked) {
		return fmt.Errorf("original scanner ancestor/file identity differs: %s", n.path)
	}
	if n.mount != 0 {
		mount, err := scannerMountID(n.path)
		if err != nil || mount != n.mount {
			return fmt.Errorf("scanner ancestor mount changed: %s", n.path)
		}
	}
	return nil
}

func scannerNodeIdentity(want, got os.FileInfo) bool {
	if want == nil || got == nil || !os.SameFile(want, got) || want.Mode() != got.Mode() {
		return false
	}
	w, wok := want.Sys().(*syscall.Stat_t)
	g, gok := got.Sys().(*syscall.Stat_t)
	if !wok || !gok || w.Uid != g.Uid || w.Gid != g.Gid {
		return false
	}
	return want.IsDir() && got.IsDir() || regular(got, 128<<20) && sameFile(want, got)
}

type trustedScannerCache struct {
	security, binary, digest string
	nodes                    []*trustedScannerNode
	mount, parentMount       uint64
	bound, sealed            bool
	inventory                map[string]string
}

func scannerMountID(path string) (uint64, error) {
	var st unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_MNT_ID, &st); err != nil {
		return 0, err
	}
	if st.Mask&unix.STATX_MNT_ID == 0 || st.Mnt_id == 0 {
		return 0, fmt.Errorf("scanner mount identity unavailable")
	}
	return st.Mnt_id, nil
}

func (s *trustedScannerCache) node(path string) *trustedScannerNode {
	for _, n := range s.nodes {
		if n.path == path {
			return n
		}
	}
	return nil
}

func (s *trustedScannerCache) protects(path string) bool {
	return s.node(path) != nil || path == s.security || trustedContained(filepath.Join(s.security, "lintai"), path)
}

func (s *trustedScannerCache) hold(path string, uid, gid int, mode os.FileMode) error {
	info, err := anchoredInfo(path)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != uid || int(st.Gid) != gid || info.Mode() != mode {
		return fmt.Errorf("declared scanner ownership/mode differs: %s", path)
	}
	fd, err := unix.Open(path, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	n := &trustedScannerNode{path: path, info: info, file: os.NewFile(uintptr(fd), path)}
	if err := n.verify(); err != nil {
		return errors.Join(err, n.file.Close())
	}
	s.nodes = append(s.nodes, n)
	return nil
}

func (s *trustedScannerCache) verify() error {
	if !s.sealed || !s.bound || s.mount == 0 {
		return fmt.Errorf("scanner cache is not sealed and bound")
	}
	for _, n := range s.nodes {
		if err := n.verify(); err != nil {
			return err
		}
	}
	mounted, err := scannerMountID(s.security)
	parent, parentErr := scannerMountID(filepath.Dir(s.security))
	if err != nil || parentErr != nil || mounted != s.mount || parent != s.parentMount || mounted == parent {
		return fmt.Errorf("original scanner mount binding differs")
	}
	return s.verifyContents()
}

func (s *trustedScannerCache) verifyContents() error {
	digest, err := hashFile(s.binary, 128<<20)
	if err != nil || digest != s.digest {
		return fmt.Errorf("pinned cached scanner bytes differ")
	}
	tree, err := trustedInventory(filepath.Join(s.security, "lintai"))
	if err != nil || !reflect.DeepEqual(tree, s.inventory) {
		return fmt.Errorf("sealed scanner subtree differs")
	}
	return nil
}

// Only the exact mount created for this task is released. A failed native tree
// remains failed even when this separate resource cleanup succeeds.
func (c *controller) closeTrustedScanner() (resultErr error) {
	s := c.scannerCache
	if s == nil {
		return nil
	}
	if s.bound {
		// Refusal keeps descriptors held until process exit. No replacement
		// inode may borrow this failed cleanup authority.
		if err := s.verify(); err != nil {
			return err
		}
		if err := unix.Unmount(s.security, unix.UMOUNT_NOFOLLOW); err != nil {
			return fmt.Errorf("exact scanner cache unmount failed: %w", err)
		}
		s.bound = false
		after, err := scannerMountID(s.security)
		if err != nil || after != s.parentMount {
			resultErr = fmt.Errorf("scanner cache original mount not restored")
		}
	}
	for _, n := range s.nodes {
		resultErr = errors.Join(resultErr, n.file.Close())
	}
	s.nodes = nil
	return resultErr
}
