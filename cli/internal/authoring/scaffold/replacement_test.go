package scaffold

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"syscall"
	"testing"
)

// Establish rename permission on this disposable fixture before Apply retains
// directory handles. A permission error by itself is never evidence of a pin.
func requireRenameRoundTrip(t *testing.T, from, to string) {
	t.Helper()
	before := skillTree(t, from)
	if err := os.Rename(from, to); err != nil {
		t.Fatalf("unpinned rename calibration: %v", err)
	}
	if err := os.Rename(to, from); err != nil {
		t.Fatalf("unpinned rename restoration: %v", err)
	}
	if !reflect.DeepEqual(before, skillTree(t, from)) {
		t.Fatal("rename calibration changed fixture")
	}
}

// Stage names are chosen inside Apply. Calibrate with a fresh sibling using the
// same private DACL, payload layout and planned bytes, closing all child handles
// before attempting the same pathname rename. No ACL or privilege bypass.
func calibrateStageRename(t *testing.T, parentPath string, files []File) {
	t.Helper()
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	const name = ".authoring-rename-probe"
	if err := makePrivateStage(parent, name); err != nil {
		t.Fatal(err)
	}
	stagePath := filepath.Join(parentPath, name)
	t.Cleanup(func() { os.RemoveAll(stagePath) })
	stage, err := parent.OpenRoot(name)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	if err := stage.Mkdir("payload", 0700); err != nil {
		t.Fatal(err)
	}
	payload, err := stage.OpenRoot("payload")
	if err != nil {
		t.Fatal(err)
	}
	err = errors.Join(writeTree(context.Background(), payload, files), payload.Close(), stage.Close(), parent.Close())
	if err != nil {
		t.Fatal(err)
	}
	requireRenameRoundTrip(t, stagePath, stagePath+"-moved")
	if err := os.RemoveAll(stagePath); err != nil {
		t.Fatal(err)
	}
}

// Linux must still perform the successful replacement attack. Windows may
// instead deny moving a directory with retained descendant handles. Only the
// exact native ERROR_ACCESS_DENIED from this rename is accepted, after the
// caller's successful unpinned calibration; generic permission errors fail.
func replacementRenameBlocked(t *testing.T, from, to string) bool {
	t.Helper()
	before := skillTree(t, from)
	info, err := os.Stat(from)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Rename(from, to)
	if err == nil {
		return false
	}
	var link *os.LinkError
	if runtime.GOOS != "windows" || !errors.As(err, &link) || link.Op != "rename" || link.Old != from || link.New != to || link.Err != syscall.Errno(5) {
		t.Fatalf("unexpected replacement rename error (pin unproven): %v", err)
	}
	now, statErr := os.Stat(from)
	if statErr != nil || !os.SameFile(info, now) || !reflect.DeepEqual(before, skillTree(t, from)) {
		t.Fatalf("denied rename changed owned tree: %v", statErr)
	}
	if _, err := os.Lstat(to); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("denied rename created destination: %v", err)
	}
	t.Log("native Windows rename denied with ERROR_ACCESS_DENIED; calibrated rename and intact pinned tree verified")
	return true
}
