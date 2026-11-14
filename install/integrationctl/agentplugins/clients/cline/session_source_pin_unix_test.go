//go:build linux || darwin

package cline

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

func TestSessionParentAcquisitionRefusesSubstitutedFIFO(t *testing.T) {
	base := t.TempDir()
	path, moved := filepath.Join(base, "selected"), filepath.Join(base, "original")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(base + string(filepath.Separator) + ".")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	calls := 0
	err = boundedSessionFIFOCall(t, path, func() error {
		directory, err := captureSessionSourceDirectory(t.Context(), root, path, "selected", true,
			func(anchor *os.Root, relative string) (*os.Root, error) {
				calls++
				if err := os.Rename(path, moved); err != nil {
					return nil, err
				}
				if err := syscall.Mkfifo(path, 0o600); err != nil {
					return nil, err
				}
				return openSessionSourceDirectory(anchor, relative)
			})
		if err == nil {
			return directory.close()
		}
		return err
	})
	if !errors.Is(err, syscall.ENOTDIR) || calls != 1 {
		t.Fatalf("substituted FIFO directory was not refused: calls=%d err=%v", calls, err)
	}
	after, err := os.Lstat(moved)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
		t.Fatalf("acquisition changed the original moved directory: %v", err)
	}
	fifo, err := os.Lstat(path)
	if err != nil || fifo.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("foreign FIFO was removed or changed: %v", err)
	}
}

func TestSessionRootedLeafAcquisitionRefusesFIFO(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "mcp.json")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(base + string(filepath.Separator) + ".")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	err = boundedSessionFIFOCall(t, path, func() error {
		file, err := atomicfile.OpenRootReadFile(root, filepath.Base(path))
		if err == nil {
			return file.Close()
		}
		return err
	})
	if !errors.Is(err, atomicfile.ErrUnsafeFile) {
		t.Fatalf("rooted FIFO was not refused before bytes: %v", err)
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
		t.Fatalf("acquisition changed the foreign FIFO: %v", err)
	}
}

// A blocking regression is released through its own FIFO before the test fails;
// the production call has no writer, fallback, retry or deadline extension.
func boundedSessionFIFOCall(t *testing.T, path string, call func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- call() }()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
		if err == nil {
			_ = syscall.Close(fd)
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		t.Fatal("source acquisition blocked on a substituted FIFO")
		return nil
	}
}
