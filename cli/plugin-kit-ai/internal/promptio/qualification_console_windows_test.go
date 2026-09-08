//go:build windows

package promptio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Runs with inherited ConPTY console input; ReadLine owns each fresh input open.
// A timing sweep exercises cancellation around ReadConsole entry; the delayed
// samples exercise a read held open without an input record from the driver.
func TestQualificationConsoleCancellation(t *testing.T) {
	if os.Getenv("UAP_QUALIFICATION_CASE") != "windows-console-cancel" {
		t.Skip("requires native ConPTY runner")
	}
	h := windows.Handle(os.Stdin.Fd())
	var before uint32
	if err := windows.GetConsoleMode(h, &before); err != nil {
		t.Fatalf("requires real inherited console input: %v", err)
	}
	oldGC := debug.SetGCPercent(-1) // Do not let finalizers hide handle leaks.
	defer debug.SetGCPercent(oldGC)
	var baseline [2]uint32
	for i := 0; i < 30; i++ {
		delay := []time.Duration{0, time.Microsecond, 100 * time.Microsecond, time.Millisecond, 10 * time.Millisecond, 80 * time.Millisecond}[i%6]
		fmt.Fprintf(os.Stdout, "QUALIFICATION_CONSOLE_READ_START %d delay=%s\n", i, delay)
		ctx, cancel := context.WithCancel(context.Background())
		timer := time.AfterFunc(delay, cancel)
		started := time.Now()
		line, err := ReadLine(ctx, os.Stdin)
		timer.Stop()
		cancel()
		if line != "" || !errors.Is(err, context.Canceled) {
			t.Fatalf("iteration %d: %q %v", i, line, err)
		}
		if time.Since(started) > time.Second {
			t.Fatalf("iteration %d cancellation exceeded one second", i)
		}
		var after uint32
		if err := windows.GetConsoleMode(h, &after); err != nil || before != after {
			t.Fatalf("console mode changed/handle closed: %d %d %v", before, after, err)
		}
		fmt.Fprintf(os.Stdout, "QUALIFICATION_CONSOLE_REUSE_READY %d\n", i)
		reuseCtx, reuseCancel := context.WithTimeout(context.Background(), 2*time.Second)
		line, err = ReadLine(reuseCtx, os.Stdin)
		reuseCancel()
		expected := fmt.Sprintf("qualification-reuse-%d", i)
		if err != nil || line != expected {
			t.Fatalf("iteration %d inherited console reuse: %q %v", i, line, err)
		}
		if err := windows.GetConsoleMode(h, &after); err != nil || before != after {
			t.Fatalf("iteration %d console mode/handle after reuse: %d %d %v", i, before, after, err)
		}
		// Keep the runner's 30 exchanges and cancellation bounds unchanged.
		// One complete timing sweep warms up before four fixed-baseline batches.
		if i == 5 {
			baseline = qualificationConsoleResources(t, nil)
			t.Logf("console warmup=6 handles=%d goroutines=%d", baseline[0], baseline[1])
		} else if i > 5 && (i+1)%6 == 0 {
			got := qualificationConsoleResources(t, &baseline)
			t.Logf("console measured=%d handles=%d goroutines=%d", i-5, got[0], got[1])
		}
	}
	qualificationConsoleQueuedAnswers(t, h, before)
	qualificationConsoleInputClassification(t, h)
	qualificationConsoleResources(t, &baseline)
	fmt.Fprintln(os.Stdout, "QUALIFICATION_CONSOLE_OK")
}
