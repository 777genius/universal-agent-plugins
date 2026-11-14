//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// SIGKILL does not run Xvfb's own lock/socket cleanup. Remove only the two
// exact stable assets captured from its verified startup, after strict success
// proved its tree reaped. A failed/unproved tree never grants unlink authority.
func (c *controller) cleanupDisplay() error {
	for _, path := range []string{"/tmp/.X11-unix/X93", "/tmp/.X93-lock"} {
		if err := c.cleanupDisplayAsset(path); err != nil {
			return err
		}
	}
	return c.cleanupCreatedDisplayParent()
}
func (c *controller) cleanupDisplayAsset(path string) error {
	asset, recorded := c.displayAssets[path]
	current, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !recorded || asset.verify(path) != nil || !sameFile(asset.info, current) {
		return fmt.Errorf("display asset identity changed; not removed")
	}
	parent := filepath.Dir(path)
	info, err := exactDirectory(parent, 0)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0002 != 0 && info.Mode()&os.ModeSticky == 0 {
		return fmt.Errorf("global display parent lacks sticky ownership")
	}
	fd, err := unix.Open(parent, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	var held unix.Stat_t
	err = unix.Fstatat(fd, filepath.Base(path), &held, unix.AT_SYMLINK_NOFOLLOW)
	if err == nil && !sameDisplayStat(held, current) {
		err = fmt.Errorf("display unlink identity changed")
	}
	if err == nil {
		err = unix.Unlinkat(fd, filepath.Base(path), 0)
	}
	closeErr := unix.Close(fd)
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}
func (c *controller) unprovedDisplayCleanup() error {
	for _, path := range []string{"/tmp/.X93-lock", "/tmp/.X11-unix/X93"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return fmt.Errorf("display tree/asset cleanup unproved; no global unlink authorized")
		}
	}
	return c.cleanupCreatedDisplayParent()
}

// The shared X11 parent is removed only if this task created that exact inode
// and it is empty. A preexisting directory or another display is never removed.
func (c *controller) cleanupCreatedDisplayParent() error {
	if c.createdDisplayParent == nil {
		return nil
	}
	path := "/tmp/.X11-unix"
	current, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !current.IsDir() || !os.SameFile(c.createdDisplayParent, current) {
		return fmt.Errorf("created display parent changed; retained")
	}
	owner, err := statUID(current)
	if err != nil || owner != 0 || current.Mode() != c.createdDisplayParent.Mode() {
		return fmt.Errorf("created display parent authority changed")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("created display parent has foreign members; retained")
	}
	return os.Remove(path)
}

func sameDisplayStat(held unix.Stat_t, current os.FileInfo) bool {
	stat, ok := current.Sys().(*syscall.Stat_t)
	return ok && held.Dev == stat.Dev && held.Ino == stat.Ino && held.Mode == stat.Mode && held.Nlink == stat.Nlink && held.Uid == stat.Uid && held.Size == stat.Size && held.Ctim.Sec == stat.Ctim.Sec && held.Ctim.Nsec == stat.Ctim.Nsec
}
