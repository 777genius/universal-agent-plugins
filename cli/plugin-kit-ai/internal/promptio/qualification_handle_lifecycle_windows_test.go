//go:build windows

package promptio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

type qualificationHandleRecord struct {
	generation int
	cancelWindowsEvent
}
type qualificationHandleTrace struct {
	mu                sync.Mutex
	records           [8192]qualificationHandleRecord
	count, generation int
	overflow          bool
}

func (q *qualificationHandleTrace) context(ctx context.Context) context.Context {
	q.generation++ // The qualification caller starts requests sequentially.
	generation := q.generation
	return context.WithValue(ctx, cancelWindowsKey{}, &cancelWindowsOps{observe: func(e cancelWindowsEvent) {
		q.mu.Lock()
		defer q.mu.Unlock()
		if q.count == len(q.records) {
			q.overflow = true
			return
		}
		q.records[q.count] = qualificationHandleRecord{generation, e}
		q.count++
	}})
}

// Each registered generation must traverse a complete branch, including failed
// acquisitions. Numeric handle reuse across generations never transfers ownership.
func (q *qualificationHandleTrace) verdict() error {
	if q.overflow {
		return fmt.Errorf("lifecycle recording overflow")
	}
	if q.generation == 0 || q.count == 0 {
		return fmt.Errorf("empty lifecycle trace")
	}
	type request struct {
		next, branch                         string
		borrowed, console, thread            windows.Handle
		worker, readDone, cancel, cancelDone bool
		workerID                             uint32
		attempts                             int
	}
	requests := make([]request, q.generation+1)
	valid := func(h windows.Handle) bool { return h != 0 && h != windows.InvalidHandle }
	for _, e := range q.records[:q.count] {
		if e.generation < 1 || e.generation > q.generation {
			return fmt.Errorf("unknown generation %d", e.generation)
		}
		r := &requests[e.generation]
		bad := func() error { return fmt.Errorf("generation=%d invalid %s in %s", e.generation, e.operation, r.next) }
		switch e.operation {
		case "request-start", "request-end", "branch-reader", "branch-file", "branch-console", "branch-stream", "worker-start", "join", "cancel-start", "cancel-complete", "read-enter":
			if e.err != nil {
				return bad()
			}
		}
		switch e.operation {
		case "request-start", "request-end", "branch-reader", "worker-start", "join", "cancel-start", "cancel-complete", "pre-cancel":
			if e.handle != 0 {
				return bad()
			}
		}
		switch e.operation {
		case "thread-acquire", "thread-close", "native-read-complete":
			if e.threadID == 0 || e.threadID != r.workerID {
				return bad()
			}
		case "read-enter", "read-complete":
			if r.worker && (e.threadID == 0 || e.threadID != r.workerID) {
				return bad()
			}
		}
		switch e.operation {
		case "cancel-start":
			if !r.worker || r.cancel || (r.next != "read-enter" && r.next != "read-complete" && r.next != "thread-close" && r.next != "join") {
				return bad()
			}
			r.cancel = true
			continue
		case "console-cancel", "thread-cancel":
			if !r.cancel || r.cancelDone {
				return bad()
			}
			h := r.thread
			if r.console != 0 {
				h = r.console
				if e.operation != "console-cancel" {
					return bad()
				}
			} else if e.operation != "thread-cancel" {
				return bad()
			}
			r.attempts++
			if !valid(h) || h != e.handle {
				return bad()
			}
			continue
		case "cancel-complete":
			if !r.cancel || r.cancelDone || (r.next != "thread-close" && r.next != "join") {
				return bad()
			}
			if (r.console != 0 || r.thread != 0) && r.attempts == 0 {
				return bad()
			}
			r.cancelDone = true
			continue
		case "native-read-complete":
			if r.branch != "console" || r.next != "read-complete" || e.handle != r.console || e.units > 1 {
				return bad()
			}
			continue
		}
		if e.operation == "request-start" {
			if r.next != "" || e.handle != 0 || e.err != nil {
				return bad()
			}
			r.next = "branch"
			continue
		}
		if r.next == "branch" {
			switch e.operation {
			case "branch-reader":
				r.branch, r.next = "reader", "read-enter"
			case "branch-file":
				r.borrowed, r.next = e.handle, "classify"
			default:
				return bad()
			}
			continue
		}
		if r.next == "classify" {
			switch e.operation {
			case "pre-cancel":
				if !errors.Is(e.err, context.Canceled) && !errors.Is(e.err, context.DeadlineExceeded) {
					return bad()
				}
				r.next = "request-end"
			case "branch-console", "branch-stream":
				if e.handle != r.borrowed {
					return bad()
				}
				if e.operation == "branch-console" {
					r.branch, r.next = "console", "input-validate"
				} else {
					r.branch, r.next = "stream", "worker-start"
				}
			default:
				return bad()
			}
			continue
		}
		if e.operation != r.next {
			return bad()
		}
		switch e.operation {
		case "input-validate":
			if e.handle != r.borrowed {
				return bad()
			}
			r.next = "console-acquire"
			if e.err != nil {
				r.next = "request-end"
			}
		case "console-acquire", "thread-acquire":
			if e.err != nil {
				if valid(e.handle) {
					return bad()
				}
				if e.operation == "console-acquire" {
					r.next = "request-end"
				} else {
					r.next = "join"
				}
				break
			}
			if !valid(e.handle) || e.handle == r.borrowed {
				return bad()
			}
			if e.operation == "console-acquire" {
				r.console, r.next = e.handle, "console-validate"
			} else {
				if e.handle == r.console {
					return bad()
				}
				r.thread, r.next = e.handle, "read-enter"
			}
		case "console-validate":
			if e.handle != r.console {
				return bad()
			}
			r.next = "worker-start"
			if e.err != nil {
				r.next = "console-close"
			}
		case "worker-start":
			if e.threadID == 0 {
				return bad()
			}
			r.workerID = e.threadID
			r.worker, r.next = true, "thread-acquire"
		case "read-enter":
			if e.handle != r.console {
				return bad()
			}
			r.next = "read-complete"
		case "read-complete":
			if e.handle != r.console {
				return bad()
			}
			r.readDone = true
			if r.worker {
				r.next = "thread-close"
			} else {
				r.next = "request-end"
			}
		case "thread-close", "console-close":
			if e.err != nil {
				return fmt.Errorf("generation=%d %s failed: %v", e.generation, e.operation, e.err)
			}
			if r.cancel && !r.cancelDone {
				return bad()
			}
			if e.operation == "thread-close" {
				if !r.readDone || e.handle != r.thread {
					return bad()
				}
				r.thread, r.next = 0, "join"
			} else {
				if e.handle != r.console {
					return bad()
				}
				r.console, r.next = 0, "request-end"
			}
		case "join":
			if !r.worker || r.thread != 0 || (r.cancel && !r.cancelDone) {
				return bad()
			}
			r.next = "request-end"
			if r.console != 0 {
				r.next = "console-close"
			}
		case "request-end":
			if r.console != 0 || r.thread != 0 {
				return bad()
			}
			r.next = "ended"
		default:
			return bad()
		}
	}
	for generation := 1; generation <= q.generation; generation++ {
		if requests[generation].next != "ended" {
			return fmt.Errorf("missing %s generation=%d", requests[generation].next, generation)
		}
	}
	return nil
}

func (q *qualificationHandleTrace) report(t *testing.T) {
	// Every ReadLine has returned and joined; no printing in cancellation windows.
	for _, e := range q.records[:q.count] {
		t.Logf("owned generation=%d op=%s handle=%#x native_tid=%d units=%d result=%v", e.generation, e.operation, e.handle, e.threadID, e.units, e.err)
	}
	if err := q.verdict(); err != nil {
		t.Errorf("owned lifecycle: %v", err)
	}
}

func TestQualificationHandleDiagnosticFailures(t *testing.T) {
	for _, failure := range []string{"acquisition", "close"} {
		t.Run(failure, func(t *testing.T) {
			f, err := os.CreateTemp(t.TempDir(), "input")
			if err != nil {
				t.Fatal(err)
			}
			qualificationCloseFile(t, f)
			q := new(qualificationHandleTrace)
			ctx := q.context(context.Background())
			ops := ctx.Value(cancelWindowsKey{}).(*cancelWindowsOps)
			if failure == "acquisition" {
				ops.duplicate = func(windows.Handle, windows.Handle, windows.Handle, *windows.Handle, uint32, bool, uint32) error {
					return windows.ERROR_ACCESS_DENIED
				}
			} else {
				ops.close = func(h windows.Handle) error {
					// Release the real test handle, but inject the native failure result.
					if err := windows.CloseHandle(h); err != nil {
						return err
					}
					return windows.ERROR_ACCESS_DENIED
				}
			}
			_, _ = readCancelable(ctx, f)
			if err := q.verdict(); (failure == "close" && (err == nil || !strings.Contains(err.Error(), failure))) || (failure == "acquisition" && err != nil) {
				t.Fatalf("missed %s: %v", failure, err)
			}
			if q.records[q.count-2].operation != "join" {
				t.Fatal("missing join on failure path")
			}
		})
	}
}

// Hold entry until a real cancellation attempt has completed. No sleeps select
// the race; the normal read then observes the canceled context and must join.
func TestQualificationHandleCancellationBeforeRead(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "input")
	if err != nil {
		t.Fatal(err)
	}
	qualificationCloseFile(t, f)
	qualificationCancellationBeforeRead(t, f)
}

func qualificationCancellationBeforeRead(t *testing.T, f *os.File) {
	q := new(qualificationHandleTrace)
	ctx, cancel := context.WithCancel(q.context(context.Background()))
	defer cancel()
	ops := ctx.Value(cancelWindowsKey{}).(*cancelWindowsOps)
	record := ops.observe
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	ops.observe = func(e cancelWindowsEvent) {
		record(e)
		if e.operation == "read-enter" {
			close(entered)
			<-release
		}
		if e.operation == "thread-cancel" || e.operation == "console-cancel" {
			once.Do(func() { close(release) })
		}
	}
	done := make(chan error, 1)
	go func() { _, err := readCancelable(ctx, f); done <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("read entry not observed")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not join")
	}
	if err := q.verdict(); err != nil {
		t.Fatal(err)
	}
	qualificationRequireOps(t, q, "worker-start", "read-complete", "cancel-start", "cancel-complete", "join")
	qualificationMutations(t, q)
}
