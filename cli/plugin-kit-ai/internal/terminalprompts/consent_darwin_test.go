//go:build darwin

package terminalprompts

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const consentGetTermios = unix.TIOCGETA

func consentPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, slave := darwinHandoffPTY(t)
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 30, Col: 100}); err != nil {
		t.Fatal(err)
	}
	return master, slave
}

// Unlike os.OpenFile, a blocking inherited descriptor wrapped by os.NewFile
// does not register the slave with Go's kqueue poller. Snapshot the untouched
// kernel defaults, as the native Python openpty CLI harness does, and also
// exercise both values of Darwin's transient PENDIN flag. Never reset attributes
// between selection, confirmation and the plain owner.
func TestDarwinConsentInheritedPTY(t *testing.T) {
	for _, flags := range []string{"kernel-defaults", "pendin-set", "pendin-clear"} {
		t.Run(flags, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDarwinConsentInheritedPTYChild$", "-test.v")
			root := t.TempDir()
			cmd.Env = []string{"HOME=" + root, "USERPROFILE=" + root,
				"XDG_CONFIG_HOME=" + root, "XDG_DATA_HOME=" + root, "XDG_CACHE_HOME=" + root,
				"TMPDIR=" + root, "DARWIN_CONSENT_FLAGS=" + flags}
			out, err := cmd.CombinedOutput()
			t.Logf("%s", out)
			if err != nil {
				t.Fatalf("native inherited PTY: %v (deadline: %v)", err, ctx.Err())
			}
		})
	}
}

func TestDarwinConsentInheritedPTYChild(t *testing.T) {
	flags := os.Getenv("DARWIN_CONSENT_FLAGS")
	if flags == "" {
		return
	}
	testConsentPTYBoundary(t, func(t *testing.T) (*os.File, *os.File) {
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
		path := strings.TrimRight(string(name[:]), "\x00")
		fd, err := unix.Open(path, unix.O_RDWR|unix.O_NOCTTY, 0)
		if err != nil {
			t.Fatal(err)
		}
		slave := os.NewFile(uintptr(fd), path)
		t.Cleanup(func() { slave.Close() })
		if flags != "kernel-defaults" {
			attrs, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
			if err != nil {
				t.Fatal(err)
			}
			switch flags {
			case "pendin-set":
				attrs.Lflag |= unix.PENDIN
			case "pendin-clear":
				attrs.Lflag &^= unix.PENDIN
			default:
				t.Fatalf("unknown initial flags %q", flags)
			}
			if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, attrs); err != nil {
				t.Fatal(err)
			}
			actual, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
			if err != nil {
				t.Fatal(err)
			}
			if actual.Lflag&unix.PENDIN != attrs.Lflag&unix.PENDIN {
				t.Fatalf("initial PENDIN not established: got %+v want %+v", actual, attrs)
			}
		}
		if err := unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, &unix.Winsize{Row: 30, Col: 100}); err != nil {
			t.Fatal(err)
		}
		return master, slave
	})
}
