//go:build linux

package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestDisplayDescriptorsHoldLockAndActualSocketUntilClose(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "uap-vsc-fd-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	lock, socket := filepath.Join(root, "lock"), filepath.Join(root, "socket")
	if err := os.WriteFile(lock, []byte("inert lock"), 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	lockInfo, socketInfo := inertAssetInfo(t, lock), inertAssetInfo(t, socket)
	assets, err := holdDisplayAssets(lock, socket, lockInfo, socketInfo)
	if err != nil || len(assets) != 2 {
		t.Fatalf("held display pair: %v %v", assets, err)
	}
	c := &controller{displayAssets: assets}
	defer func() { _ = c.closeDisplayAssets() }()
	for path, asset := range assets {
		if err := asset.verify(path); err != nil {
			t.Fatal(err)
		}
		flags, err := unix.FcntlInt(asset.file.Fd(), unix.F_GETFL, 0)
		if err != nil || flags&unix.O_PATH == 0 {
			t.Fatal("display handle is not path-only", err)
		}
		fdFlags, err := unix.FcntlInt(asset.file.Fd(), unix.F_GETFD, 0)
		if err != nil || fdFlags&unix.FD_CLOEXEC == 0 {
			t.Fatal("display handle can cross exec", err)
		}
	}
	if err := c.closeDisplayAssets(); err != nil || len(c.displayAssets) != 0 {
		t.Fatalf("close both display handles: %v", err)
	}
	for path, asset := range assets {
		if asset.verify(path) == nil {
			t.Fatal("closed descriptor retained display authority")
		}
	}
}

func TestDisplayDescriptorPairClosesFirstOnSecondOpenFailure(t *testing.T) {
	root := t.TempDir()
	lock, other := filepath.Join(root, "lock"), filepath.Join(root, "other")
	for _, path := range []string{lock, other} {
		if err := os.WriteFile(path, []byte("inert"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	lockInfo, otherInfo := inertAssetInfo(t, lock), inertAssetInfo(t, other)
	if err := os.Remove(other); err != nil {
		t.Fatal(err)
	}
	before := inertAssetFDCount(t, lock)
	assets, err := holdDisplayAssets(lock, other, lockInfo, otherInfo)
	if err == nil || assets != nil || inertAssetFDCount(t, lock) != before {
		t.Fatal("partial display admission retained a descriptor or authority", err)
	}
}

func TestDisplayDescriptorRejectsSymlinkAndMissingOrClosedAuthority(t *testing.T) {
	root := t.TempDir()
	path, link := filepath.Join(root, "asset"), filepath.Join(root, "link")
	if err := os.WriteFile(path, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	info := inertAssetInfo(t, path)
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := holdDisplayAsset(link, info); err == nil {
		t.Fatal("display descriptor followed a symlink")
	}
	c := &controller{displayAssets: map[string]heldDisplayAsset{path: {info: info}}}
	if c.cleanupDisplayAsset(path) == nil {
		t.Fatal("missing descriptor granted unlink authority")
	}
	asset, err := holdDisplayAsset(path, info)
	if err != nil {
		t.Fatal(err)
	}
	if err := asset.file.Close(); err != nil {
		t.Fatal(err)
	}
	c.displayAssets[path] = asset
	if c.cleanupDisplayAsset(path) == nil || c.closeDisplayAssets() == nil {
		t.Fatal("closed descriptor granted unlink or concealed close failure")
	}
	if string(mustReadInert(t, path)) != "unchanged" {
		t.Fatal("refused descriptor cleanup changed named bytes")
	}
}

func inertAssetInfo(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func inertAssetFDCount(t *testing.T, path string) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err == nil && target == path {
			count++
		}
	}
	return count
}
