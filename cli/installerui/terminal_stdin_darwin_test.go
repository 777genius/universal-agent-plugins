//go:build darwin

package installerui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"golang.org/x/sys/unix"
)

// The public installer redirects /dev/tty onto stdin, whose Go filename remains
// /dev/stdin. Escape must join its reader, restore the TTY and permit reuse.
func TestDarwinInheritedTTYCancel(t *testing.T) {
	master, slave := consentPTY(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDarwinInheritedTTYCancelChild$", "-test.v")
	home := t.TempDir()
	cmd.Env = []string{"TEST_DARWIN_STDIN=1", "HOME=" + home, "TMPDIR=" + t.TempDir(), "TERM=xterm-256color"}
	cmd.Stdin = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	var diagnostics bytes.Buffer
	cmd.Stdout = &diagnostics
	cmd.Stderr = &diagnostics
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	joined := false
	defer func() {
		if !joined {
			_ = cmd.Process.Kill()
			<-done
		}
	}()
	var screen strings.Builder
	sent := 0
	for {
		select {
		case err := <-done:
			joined = true
			if err != nil || sent != 4 {
				t.Fatalf("inherited TTY: %v, sent=%d, deadline=%v\n%s\n%s", err, sent, ctx.Err(), diagnostics.String(), screen.String())
			}
			return
		default:
		}
		fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
		if _, err := unix.Poll(fds, 20); err != nil {
			if err == unix.EINTR {
				continue
			}
			t.Fatal(err)
		}
		if fds[0].Revents&unix.POLLIN == 0 {
			continue
		}
		var buf [8192]byte
		n, err := master.Read(buf[:])
		if err != nil {
			_ = cmd.Process.Kill()
			waitErr := <-done
			joined = true
			if waitErr == nil && sent == 4 {
				return
			}
			t.Fatalf("inherited TTY: %v, read=%v, sent=%d, deadline=%v\n%s\n%s", waitErr, err, sent, ctx.Err(), diagnostics.String(), screen.String())
		}
		screen.Write(buf[:n])
		marker := fmt.Sprintf("Choose fixture %d", sent)
		keys := "\x1b"
		if sent == 3 {
			marker = "NEXT OWNER"
			keys = "next owner\n"
		}
		if sent < 4 && strings.Contains(screen.String(), marker) {
			if _, err := io.WriteString(master, keys); err != nil {
				t.Fatal(err)
			}
			sent++
		}
	}
}

func TestDarwinInheritedTTYCancelChild(t *testing.T) {
	if os.Getenv("TEST_DARWIN_STDIN") != "1" {
		return
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer tty.Close()
	// Match shell's <&3 duplication; do not rename or close os.Stdin.
	if err := unix.Dup2(int(tty.Fd()), 0); err != nil {
		t.Fatal(err)
	}
	original, err := unix.IoctlGetTermios(0, unix.TIOCGETA)
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewTerminal(TerminalConfig{Input: os.Stdin, Output: tty, Mode: ModeRich, NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		result, err := p.SelectMany(ctx, MultiSelectRequest{SelectRequest: SelectRequest{Title: fmt.Sprintf("Choose fixture %d", i), Options: []Option{{ID: "alpha", Label: "Alpha"}}}})
		cancel()
		if err != nil || !result.Cancelled || result.Accepted || len(result.IDs) != 0 {
			t.Fatalf("Escape: %+v %v", result, err)
		} //nolint:misspell // Public cancellation field.
		attrs, err := unix.IoctlGetTermios(0, unix.TIOCGETA)
		if err != nil || *attrs != *original {
			t.Fatalf("terminal restore: %v", err)
		}
	}
	if _, err := io.WriteString(tty, "NEXT OWNER\n"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	line, err := promptio.ReadLine(ctx, os.Stdin)
	if err != nil || line != "next owner" {
		t.Fatalf("next owner: %q %v", line, err)
	}
}
