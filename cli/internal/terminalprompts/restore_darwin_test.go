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
	// The exact queued LF must complete a canonical record after replay.
	// Do not inherit a newline-to-CR mapping from the PTY's initial flags.
	original.Iflag &^= unix.INLCR | unix.IGNCR
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
	// Keep an explicit reader knote attached across restoration. TIOCSETA's
	// wakeup can invoke ttnread before it installs the canonical attributes;
	// the test must exercise that path independently of Go's runtime poller.
	kq, err := unix.Kqueue()
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(kq)
	change := unix.Kevent_t{Ident: uint64(fd), Filter: unix.EVFILT_READ, Flags: unix.EV_ADD | unix.EV_CLEAR}
	if _, err := unix.Kevent(kq, []unix.Kevent_t{change}, nil, nil); err != nil {
		t.Fatal(err)
	}
	const queued = "next owner\n"
	if _, err := io.WriteString(master, queued); err != nil {
		t.Fatal(err)
	}
	// A kernel readiness event plus an exact raw byte count acknowledges the
	// whole write without reading it. Raw readiness does not yet prove that a
	// completed canonical record exists; the post-restore probes below do.
	const fionread = 0x4004667f
	deadline := time.Now().Add(time.Second)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatal("raw queue readiness deadline")
		}
		var events [1]unix.Kevent_t
		timeout := unix.NsecToTimespec(remaining.Nanoseconds())
		count, err := unix.Kevent(kq, nil, events[:], &timeout)
		if err == unix.EINTR {
			continue
		}
		if err != nil || count != 1 {
			t.Fatalf("raw queue readiness: %d %v", count, err)
		}
		if event := events[0]; event.Ident != uint64(fd) || event.Filter != unix.EVFILT_READ || event.Flags&(unix.EV_ERROR|unix.EV_EOF) != 0 {
			t.Fatalf("raw queue event: %+v", event)
		}
		n, err := unix.IoctlGetInt(fd, fionread)
		if err != nil {
			t.Fatal(err)
		}
		if n == len(queued) {
			break
		}
		if n > len(queued) {
			t.Fatalf("raw queue: got %d want %d", n, len(queued))
		}
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
				t.Fatalf("canonical queue probe %d: got %d want %d, error %v; attrs %+v", i, n, len(queued), err, get())
			}
			if got := get(); got != original {
				t.Fatalf("renewed replay: got %+v want %+v", got, original)
			}
		}
	}
	// With original PENDIN set, the exact-state assertion above requires that
	// replay remain pending; this read is the next owner's pending work.
	var buf [64]byte
	n, err := slave.Read(buf[:])
	if err != nil || string(buf[:n]) != queued {
		t.Fatalf("next owner: %q %v", buf[:n], err)
	}
	if n, err := unix.IoctlGetInt(fd, fionread); err != nil || n != 0 {
		t.Fatalf("remaining queue: %d %v", n, err)
	}
}
