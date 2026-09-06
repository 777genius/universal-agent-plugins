package commands_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

// Match the actual Root.Remove cause, never a rename/access/sharing denial.
func reviewCleanupRequireNonempty(t *testing.T, err error, name string) {
	t.Helper()
	want := syscall.ENOTEMPTY
	if runtime.GOOS == "windows" {
		want = syscall.Errno(145) // ERROR_DIR_NOT_EMPTY
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "removeat" || pathErr.Path != name || !errors.Is(pathErr.Err, want) {
		t.Fatalf("expected nonempty container removal for %q: %v", name, err)
	}
}

func reviewCleanupVerifyRetainedEntry(t *testing.T, stage string, before os.FileInfo) {
	t.Helper()
	after, err := os.Stat(stage)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("retained container identity changed: %v", err)
	}
	entries, err := os.ReadDir(stage)
	if err != nil || len(entries) != 1 || entries[0].Name() != "preserve" || !entries[0].Type().IsRegular() {
		t.Fatalf("owned payload not cleaned or unrelated entry changed: %v %v", entries, err)
	}
	if _, err := os.Lstat(stage + "-displaced"); !os.IsNotExist(err) {
		t.Fatalf("retained-entry fixture unexpectedly displaced stage: %v", err)
	}

	// Independently reproduce nonempty removal on this exact disposable stage
	// after Apply closed its handles. Then remove only our sentinel and prove
	// empty removal succeeds. This distinguishes the intended cleanup fault
	// from ACL or handle denial; no permission or security changes are needed.
	parent, err := os.OpenRoot(filepath.Dir(stage))
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	name := filepath.Base(stage)
	reviewCleanupRequireNonempty(t, parent.Remove(name), name)
	body, err := os.ReadFile(filepath.Join(stage, "preserve"))
	if err != nil || string(body) != "ordinary-review-value" {
		t.Fatal("nonempty-removal calibration changed protected content")
	}
	if err := parent.Remove(filepath.Join(name, "preserve")); err != nil {
		t.Fatal(err)
	}
	if err := parent.Remove(name); err != nil {
		t.Fatalf("empty-container removal calibration: %v", err)
	}
	t.Log("retained-entry fault: owned payload removed, container identity and protected bytes preserved; nonempty/empty removal calibrated")
}
