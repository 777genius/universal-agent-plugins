//go:build darwin

package terminalprompts

import (
	"context"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func TestDarwinRestoreQueuedFlags(t *testing.T) {
	for _, scenario := range []string{"flusho", "flusho-no-echo", "flusho-pendin"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDarwinRestoreQueuedFlagsChild$", "-test.v")
			cmd.Env = []string{"DARWIN_RESTORE_CASE=" + scenario, "TMPDIR=" + t.TempDir()}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("PTY restore: %v (deadline: %v)\n%s", err, ctx.Err(), out)
			}
		})
	}
}

func TestDarwinRestoreQueuedFlagsChild(t *testing.T) {
	scenario := os.Getenv("DARWIN_RESTORE_CASE")
	if scenario == "" {
		return
	}
	master, slave := darwinHandoffPTY(t)
	fd := int(slave.Fd())
	get := func() unix.Termios {
		t.Helper()
		attrs, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
		if err != nil {
			t.Fatal(err)
		}
		return *attrs
	}
	original := get()
	original.Lflag &^= unix.PENDIN
	original.Lflag |= unix.FLUSHO
	if scenario == "flusho-pendin" {
		original.Lflag |= unix.PENDIN
	}
	if scenario == "flusho-no-echo" {
		original.Lflag &^= unix.ECHO | unix.ECHONL
	}
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &original); err != nil {
		t.Fatal(err)
	}
	if got := get(); got != original {
		t.Fatalf("snapshot setup: got %+v want %+v", got, original)
	}
	restore, err := promptio.SnapshotTerminal(fd)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := term.MakeRaw(fd); err != nil {
		t.Fatal(err)
	}
	const queued = "next owner\n"
	if _, err := io.WriteString(master, queued); err != nil {
		t.Fatal(err)
	}
	// Observe the raw queue before restoring canonical mode. This probe cannot
	// replay it: MakeRaw cleared PENDIN when it disabled ICANON.
	const fionread = 0x4004667f
	deadline := time.Now().Add(time.Second)
	for {
		n, err := unix.IoctlGetInt(fd, fionread)
		if err != nil {
			t.Fatal(err)
		}
		if n == len(queued) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("raw queue: got %d want %d", n, len(queued))
		}
		time.Sleep(time.Millisecond)
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}
	if got := get(); got != original {
		t.Fatalf("restore: got %+v want %+v", got, original)
	}
	if original.Lflag&unix.PENDIN == 0 {
		// Subsequent next-owner probes must neither replay input nor clear FLUSHO.
		for i := 0; i < 2; i++ {
			n, err := unix.IoctlGetInt(fd, fionread)
			if err != nil || n != len(queued) {
				t.Fatalf("queue probe: %d %v", n, err)
			}
			if got := get(); got != original {
				t.Fatalf("renewed replay: got %+v want %+v", got, original)
			}
		}
	}
	// With original PENDIN set, the exact-state assertion above also proves
	// restore skipped replay; this read is the next owner's pending work.
	var buf [64]byte
	n, err := slave.Read(buf[:])
	if err != nil || string(buf[:n]) != queued {
		t.Fatalf("next owner: %q %v", buf[:n], err)
	}
	if n, err := unix.IoctlGetInt(fd, fionread); err != nil || n != 0 {
		t.Fatalf("remaining queue: %d %v", n, err)
	}
}
