//go:build darwin

package terminalprompts

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The public shell duplicates its virtual /dev/tty onto stdin. Qualify the
// actual consent boundary and suffix handoff using that device, not a PTY slave.
func TestDarwinVirtualTTYConsent(t *testing.T) {
	for _, name := range []string{"queued-alt-enter-next-owner", "queued-escape", "fresh-space-enter"} {
		t.Run(name, func(t *testing.T) {
			master, slave := consentPTY(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			root := t.TempDir()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDarwinVirtualTTYConsentChild$", "-test.v")
			cmd.Env = []string{"HOME=" + root, "TMPDIR=" + root, "TERM=xterm-256color", "TEST_VIRTUAL_CONSENT=" + name}
			var diagnostics bytes.Buffer
			cmd.Stdout = &diagnostics
			cmd.Stderr = &diagnostics
			cmd.Stdin = slave
			cmd.ExtraFiles = []*os.File{master}
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
			// Closing this parent's slave lets the child observe EOF after closing its
			// own last terminal handle; inherited fd3 belongs only to the controller.
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			if err := slave.Close(); err != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				t.Fatal(err)
			}
			out, err := cmd.Wait(), ctx.Err()
			if out != nil {
				t.Fatalf("virtual consent: %v (deadline: %v)\n%s", out, err, diagnostics.String())
			}
		})
	}
}

func TestDarwinVirtualTTYConsentChild(t *testing.T) {
	name := os.Getenv("TEST_VIRTUAL_CONSENT")
	if name == "" {
		return
	}
	switch name {
	case "queued-alt-enter-next-owner", "queued-escape", "fresh-space-enter":
	default:
		t.Fatal("unknown virtual consent scenario")
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Dup2(int(tty.Fd()), 0); err != nil {
		tty.Close()
		t.Fatal(err)
	}
	if err := tty.Close(); err != nil {
		t.Fatal(err)
	}
	// os.Stdin keeps its original /dev/stdin name. No physical-slave descriptor
	// remains in this fresh subprocess to hide the virtual backend's behavior.
	master := os.NewFile(3, "TEST-virtual-consent-controller")
	defer master.Close()
	testConsentPTYCase(t, func(*testing.T) (*os.File, *os.File) { return master, os.Stdin }, name)
}
