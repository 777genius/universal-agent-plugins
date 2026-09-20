//go:build darwin

package terminalprompts

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/muesli/cancelreader"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// Each production regression owns a fresh PTY and a bounded subprocess. A
// blocking kernel read must fail its case without leaking into another owner.
func TestDarwinInputHandoff(t *testing.T) {
	for _, scenario := range []string{"queued", "partial-eof", "canonical-queued", "record-boundary", "long-line", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDarwinInputHandoffChild$", "-test.v")
			home := t.TempDir()
			cmd.Env = []string{"HOME=" + home, "USERPROFILE=" + home,
				"XDG_CONFIG_HOME=" + home, "XDG_DATA_HOME=" + home, "XDG_CACHE_HOME=" + home,
				"TMPDIR=" + t.TempDir(), "DARWIN_HANDOFF_CASE=" + scenario}
			out, err := cmd.CombinedOutput()
			t.Logf("%s", out)
			if err != nil {
				t.Fatalf("native production regression: %v (deadline: %v)", err, ctx.Err())
			}
		})
	}
}

func TestDarwinInputHandoffChild(t *testing.T) {
	scenario := os.Getenv("DARWIN_HANDOFF_CASE")
	if scenario == "" {
		return
	}
	master, slave := darwinHandoffPTY(t)
	original, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TIOCGETA)
	if err != nil {
		t.Fatal(err)
	}
	write := func(s string) {
		t.Helper()
		if _, err := io.WriteString(master, s); err != nil {
			t.Fatal(err)
		}
	}
	want := "n"
	queuedNext := false
	switch scenario {
	case "queued":
		state, err := term.MakeRaw(int(slave.Fd()))
		if err != nil {
			t.Fatal(err)
		}
		owned, err := cancelreader.NewReader(slave)
		if err != nil {
			t.Fatal(err)
		}
		r := newSubmissionReader(context.Background(), owned)
		// The actual CLI batch: rich submission followed by the Plain answer.
		write(" \rn\nnext\n")
		for _, want := range []byte(" \r") {
			var b [1]byte
			if n, err := r.Read(b[:]); n != 1 || err != nil || b[0] != want {
				t.Fatalf("rich read: %d %q %v", n, b, err)
			}
		}
		r.finish()
		owned.Cancel()
		if err := owned.Close(); err != nil {
			t.Fatal(err)
		}
		if err := term.Restore(int(slave.Fd()), state); err != nil {
			t.Fatal(err)
		}
		queuedNext = true
	case "partial-eof":
		write("y\x04\x04") // Preserve the actual CLI fixture and first-read reuse assertion.
	case "canonical-queued":
		write("n\nnext\n")
		queuedNext = true
	case "record-boundary":
		// VEOF terminates a record, not an answer. Read through to the newline,
		// leaving the following owner's already queued record in the kernel.
		write("n\x04\nnext\n")
		queuedNext = true
	case "long-line":
		want = strings.Repeat("x", 1000) // Within Darwin's canonical queue limit.
		write(want + "\n")
	case "cancel":
	default:
		t.Fatalf("unknown case %q", scenario)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	line, err := promptio.ReadLine(ctx, slave)
	if scenario == "cancel" {
		if line != "" || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("blocked read cancellation: %q %v", line, err)
		}
	} else if scenario == "partial-eof" {
		if line != "" || !errors.Is(err, prompt.ErrPromptInputClosed) {
			t.Fatalf("partial EOF must fail closed: %q %v", line, err)
		}
	} else if line != want || err != nil {
		t.Fatalf("answer: %q %v; want %q", line, err, want)
	}
	attrs, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TIOCGETA)
	if err != nil {
		t.Fatal(err)
	}
	if *attrs != *original {
		t.Fatalf("terminal state changed: got %+v want %+v", attrs, original)
	}
	if !queuedNext {
		write("next\n")
	}
	// No retry past a residual EOF record. The next owner must succeed first try.
	var next [32]byte
	n, err := slave.Read(next[:])
	if string(next[:n]) != "next\n" || err != nil {
		t.Fatalf("same-terminal next-line reuse failed: %q %v", next[:n], err)
	}
}

// Fresh kernel PTYs only; no controlling terminal or real profile is opened.
// Darwin's ptmx ioctls avoid adding a test dependency to the module graph.
func darwinHandoffPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { master.Close() })
	var name [128]byte
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0])))
	if errno != 0 {
		t.Fatal(errno)
	}
	for _, op := range []uint{unix.TIOCPTYGRANT, unix.TIOCPTYUNLK} {
		if err := unix.IoctlSetInt(int(master.Fd()), op, 0); err != nil {
			t.Fatal(err)
		}
	}
	slave, err := os.OpenFile(strings.TrimRight(string(name[:]), "\x00"), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { slave.Close() })
	state, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TIOCGETA)
	if err != nil {
		t.Fatal(err)
	}
	state.Lflag |= unix.ICANON | unix.ECHO | unix.ISIG
	state.Iflag |= unix.ICRNL
	state.Cc[unix.VEOF] = 4
	if err := unix.IoctlSetTermios(int(slave.Fd()), unix.TIOCSETA, state); err != nil {
		t.Fatal(err)
	}
	return master, slave
}
