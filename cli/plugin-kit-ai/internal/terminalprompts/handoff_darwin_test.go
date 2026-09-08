//go:build darwin && terminaldiagnostic

package terminalprompts

import (
	"context"
	"errors"
	"fmt"
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

// Run each diagnostic in a disposable process: cancelreader's kqueue readiness
// wait can succeed before a blocking terminal Read, which Cancel cannot wake.
// A parent deadline preserves the last read phase without leaking that reader.
// These controls intentionally retain the production acceptance assertions.
// Temporary investigation only: remove or convert after the root-cause fix.
// The explicit terminaldiagnostic tag keeps default maintained tests unchanged.
func TestDarwinInputHandoff(t *testing.T) {
	for _, scenario := range []string{"queued", "partial-eof", "cancel"} {
		for _, reader := range []string{"direct-byte", "direct-record", "kqueue", "select", "production"} {
			if scenario == "cancel" && reader != "production" {
				continue
			}
			t.Run(scenario+"/"+reader, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDarwinInputHandoffChild$", "-test.v")
				cmd.Env = []string{"HOME=" + t.TempDir(), "TMPDIR=" + t.TempDir(),
					"DARWIN_HANDOFF_CASE=" + scenario, "DARWIN_HANDOFF_READER=" + reader}
				out, err := cmd.CombinedOutput()
				t.Logf("%s", out)
				if err != nil {
					t.Fatalf("native diagnostic: %v (deadline: %v)", err, ctx.Err())
				}
			})
		}
	}
}

func TestDarwinInputHandoffChild(t *testing.T) {
	scenario := os.Getenv("DARWIN_HANDOFF_CASE")
	if scenario == "" {
		return // Helper process only; the parent executes every case.
	}
	mode := os.Getenv("DARWIN_HANDOFF_READER")
	master, slave := darwinHandoffPTY(t)
	fmt.Printf("scenario=%s reader=%s slave=%s\n", scenario, mode, slave.Name())
	write := func(s string) {
		t.Helper()
		fmt.Printf("write master %q\n", s)
		if _, err := io.WriteString(master, s); err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "queued" {
		state, err := term.MakeRaw(int(slave.Fd()))
		if err != nil {
			t.Fatal(err)
		}
		owned, err := cancelreader.NewReader(slave)
		if err != nil {
			t.Fatal(err)
		}
		r := newSubmissionReader(context.Background(), owned)
		// Exactly the native queued-lifecycle confirmation batch. No input is
		// added at the Plain marker and no queue is flushed during restore.
		write(" \rn\n")
		for _, want := range []byte(" \r") {
			var b [1]byte
			fmt.Println("rich read begin")
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
		fmt.Println("rich reader closed; canonical mode restored")
	} else if scenario == "partial-eof" {
		write("y\x04\x04") // Exactly the native partial-EOF fixture.
	}

	var reader io.Reader = slave
	if mode == "kqueue" || mode == "select" {
		var source cancelreader.File = slave
		if mode == "select" {
			// Diagnostic only: the pinned library selects its select backend
			// by this name. The descriptor still names the SAME fresh slave.
			source = darwinSelectControl{slave}
		}
		cr, err := cancelreader.NewReader(source)
		if err != nil {
			t.Fatal(err)
		}
		defer cr.Close()
		reader = cr
	}
	if mode == "production" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		fmt.Println("promptio.ReadLine begin")
		line, err := promptio.ReadLine(ctx, slave)
		fmt.Printf("promptio.ReadLine end line=%q err=%v\n", line, err)
		if scenario == "queued" {
			if line != "n" || err != nil {
				t.Fatalf("queued answer: %q %v", line, err)
			}
		} else if scenario == "cancel" {
			if line != "" || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("blocked read cancellation: %q %v", line, err)
			}
		} else if line != "" || !errors.Is(err, prompt.ErrPromptInputClosed) {
			t.Fatalf("partial EOF must fail closed: %q %v", line, err)
		}
	} else {
		size := 1
		if mode == "direct-record" {
			size = 4096
		}
		var got strings.Builder
		for i := 0; i < 4; i++ {
			buf := make([]byte, size)
			fmt.Printf("plain read begin size=%d\n", size)
			n, err := reader.Read(buf)
			fmt.Printf("plain read end n=%d data=%q err=%v\n", n, buf[:n], err)
			got.Write(buf[:n])
			if err != nil && err != io.EOF {
				t.Fatal(err)
			}
			if n == 0 || err == io.EOF || strings.HasSuffix(got.String(), "\n") {
				break
			}
		}
		want := "n\n"
		if scenario == "partial-eof" {
			want = "y"
		}
		if got.String() != want {
			t.Fatalf("input changed: got %q want %q", got.String(), want)
		}
	}
	// A single direct read exposes a residual EOF record instead of silently
	// retrying past it. This is the same next-line contract as the shell probe.
	write("next\n")
	fmt.Println("same-slave next-line read begin")
	var next [32]byte
	n, err := slave.Read(next[:])
	fmt.Printf("same-slave next-line read end n=%d data=%q err=%v\n", n, next[:n], err)
	if string(next[:n]) != "next\n" || err != nil {
		t.Fatalf("same-terminal next-line reuse failed: %q %v", next[:n], err)
	}
}

type darwinSelectControl struct{ *os.File }

func (darwinSelectControl) Name() string { return "/dev/tty" }

// Fresh kernel PTYs only; no controlling terminal or real profile is opened.
// Darwin's ptmx ioctls avoid adding a test dependency to the module graph.
func darwinHandoffPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
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
