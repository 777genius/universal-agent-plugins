//go:build linux || darwin

package hermes

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestProjectSessionOpenCannotBlockOnFIFOReplacement(t *testing.T) {
	path := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(path, "member"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	file, err := openSessionFile(root, "member")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	flags, err := unix.FcntlInt(file.Fd(), unix.F_GETFL, 0)
	if err != nil || flags&syscall.O_NONBLOCK == 0 {
		t.Fatal("FIFO open was not nonblocking", err)
	}
	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatal("fixture was not held FIFO", err)
	}
	if _, err := readSessionMember(root, "member", 64); err == nil {
		t.Fatal("accepted FIFO source")
	}
}

func TestProjectSessionRefusesNonPrivateRootBeforeConfigRead(t *testing.T) {
	req, owner := newProjectSessionFixture(t)
	if err := os.Chmod(projectSessionRoot(req.ProjectRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProjectSession(t.Context(), req, owner); err == nil {
		t.Fatal("session accepted a nonprivate selected root")
	}
}
