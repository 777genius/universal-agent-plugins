//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

type nativeLogLeaf struct {
	parent, file *os.File
	original     os.FileInfo
}

func holdNativeLogLeaf(root *os.Root, name string, owner, group uint32) (*nativeLogLeaf, nativeLogCheck) {
	info, err := root.Lstat(name)
	if err != nil || !regular(info, 64<<10) {
		return nil, "leaf-stat-type-link-size"
	}
	stat := info.Sys().(*syscall.Stat_t)
	if stat.Uid != owner || stat.Gid != group {
		return nil, "leaf-owner-device"
	}
	parent, err := root.Open(".")
	if err != nil {
		return nil, "leaf-mount"
	}
	file, err := root.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		_ = parent.Close()
		return nil, "leaf-open"
	}
	return &nativeLogLeaf{parent: parent, file: file, original: info}, "none"
}

func (leaf *nativeLogLeaf) close() {
	_ = leaf.file.Close()
	_ = leaf.parent.Close()
}

func nativeLogMountID(file *os.File) (uint64, error) {
	var st unix.Statx_t
	if err := unix.Statx(int(file.Fd()), "", unix.AT_EMPTY_PATH, unix.STATX_MNT_ID, &st); err != nil {
		return 0, err
	}
	if st.Mask&unix.STATX_MNT_ID == 0 || st.Mnt_id == 0 {
		return 0, fmt.Errorf("native log mount identity unavailable")
	}
	return st.Mnt_id, nil
}

func nativeLogRootMount(root *os.Root) (uint64, error) {
	file, err := root.Open(".")
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()
	return nativeLogMountID(file)
}

// Overlayfs can report different st_dev values for a directory and its ordinary
// child. Compare the actual held mount IDs instead; each file's own dev/ino and
// the remaining immutable metadata still have to survive the separate joins.
func (b *nativeLogBaseline) sameNativeLogMounts(parent, leaf *os.File) bool {
	parentMount, parentErr := nativeLogMountID(parent)
	leafMount, leafErr := nativeLogMountID(leaf)
	profileMount, profileErr := scannerMountID(b.profile)
	return parentErr == nil && leafErr == nil && profileErr == nil && b.mount != 0 &&
		parentMount == b.mount && leafMount == b.mount && profileMount == b.mount
}
