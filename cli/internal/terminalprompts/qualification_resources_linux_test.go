//go:build linux

package terminalprompts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"golang.org/x/sys/unix"
)

// The forms AND their PTY controller live in this process. Every session opens
// a fresh kernel PTY; no subprocess census can hide controller descriptor leaks.
func TestQualificationPTYResources(t *testing.T) {
	oldGC := debug.SetGCPercent(-1) // Finalizers must not repair leaked descriptors.
	defer debug.SetGCPercent(oldGC)
	t.Setenv("TERM", "xterm-256color")
	for i := 0; i < 6; i++ {
		qualificationPTYSession(t, i)
	}
	baseline := qualificationPTYStable(t, nil)
	t.Logf("warmup=6 fd=%d goroutines=%d", baseline[0], baseline[1])
	for i := 6; i < 30; i++ {
		qualificationPTYSession(t, i)
		if (i+1)%6 == 0 {
			got := qualificationPTYStable(t, &baseline)
			t.Logf("measured=%d fd=%d goroutines=%d", i-5, got[0], got[1])
		}
	}
}

func qualificationPTYStable(t *testing.T, ceiling *[2]int) [2]int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last [2]int
	stable := 0
	for {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		got := [2]int{len(entries), runtime.NumGoroutine()}
		if got == last && (ceiling == nil || (got[0] <= ceiling[0] && got[1] <= ceiling[1])) {
			stable++
		} else {
			stable = 0
		}
		if stable == 3 {
			return got
		}
		if time.Now().After(deadline) {
			stacks := make([]byte, 1<<20)
			n := runtime.Stack(stacks, true)
			t.Fatalf("PTY resources did not settle: baseline=%v actual=%v\n%s", ceiling, got, stacks[:n])
		}
		last = got
		time.Sleep(20 * time.Millisecond)
	}
}

func qualificationPTYSession(t *testing.T, iteration int) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 30, Col: 100}); err != nil {
		t.Fatal(err)
	}
	before, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}

	// Read the actual renderer stream, with an explicit joined controller lifetime.
	ready := make(chan struct{}, 1)
	drained := make(chan []byte, 1)
	go func() {
		var output bytes.Buffer
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			output.Write(buf[:n])
			if bytes.Contains(output.Bytes(), []byte("Yes")) {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
			if err != nil {
				drained <- output.Bytes()
				return
			}
		}
	}()
	defer func() {
		slave.Close()
		master.Close()
		select {
		case <-drained:
		case <-time.After(time.Second):
			t.Error("PTY controller failed to join")
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	type outcome struct {
		result prompt.ConfirmationResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := (HuhPrompter{Input: slave, Output: slave, NoColor: true}).Confirm(ctx, prompt.ConfirmationRequest{Title: "Resource qualification?"})
		done <- outcome{result, err}
	}()
	select {
	case <-ready:
	case result := <-done:
		t.Fatalf("iteration %d form returned before rendering: %+v", iteration, result)
	case <-ctx.Done():
		t.Fatal("PTY form did not render within three seconds")
	}
	reuse := fmt.Sprintf("next-owner-%d", iteration)
	if iteration%2 == 0 {
		// Queue the next owner's answer with the submission, before cleanup.
		if _, err := io.WriteString(master, " \r"+reuse+"\n"); err != nil {
			t.Fatal(err)
		}
	} else {
		cancel()
	}
	var result outcome
	select {
	case result = <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("PTY form did not join")
	}
	if iteration%2 == 0 {
		if result.err != nil || !result.result.Accepted {
			t.Fatalf("submission: %+v %v", result.result, result.err)
		}
	} else if !errors.Is(result.err, context.Canceled) || result.result.Accepted {
		t.Fatalf("cancellation: %+v %v", result.result, result.err)
	}
	after, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil || *before != *after {
		t.Fatalf("PTY mode changed/handle closed: before=%+v after=%+v err=%v", before, after, err)
	}
	if iteration%2 != 0 {
		if _, err := io.WriteString(master, reuse+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	reuseCtx, reuseCancel := context.WithTimeout(context.Background(), time.Second)
	defer reuseCancel()
	line, err := promptio.ReadLine(reuseCtx, slave)
	if err != nil || line != reuse {
		t.Fatalf("next-owner input: %q %v", line, err)
	}
	// Closing the slave delivers EIO only after the controller drains queued output.
	slave.Close()
	select {
	case output := <-drained:
		hide := bytes.LastIndex(output, []byte("\x1b[?25l"))
		show := bytes.LastIndex(output, []byte("\x1b[?25h"))
		if hide < 0 || show <= hide {
			t.Fatalf("cursor was not hidden then restored: %q", output)
		}
		drained <- output // deferred join also covers early failure paths
	case <-time.After(time.Second):
		t.Fatal("PTY output failed to drain")
	}
}
