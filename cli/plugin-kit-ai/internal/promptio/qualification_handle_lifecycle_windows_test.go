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

// Identity is (request generation, kind, native handle), never a raw handle
// alone: Windows may reuse the numeric value immediately after successful close.
func (q *qualificationHandleTrace) verdict() error {
	if q.overflow {
		return fmt.Errorf("lifecycle recording overflow")
	}
	owned := map[string]windows.Handle{}
	joined := map[int]bool{}
	completed := map[int]bool{}
	threads := map[int]bool{}
	for _, e := range q.records[:q.count] {
		kind, _, _ := strings.Cut(e.operation, "-")
		key := fmt.Sprintf("%d/%s", e.generation, kind)
		switch {
		case strings.HasSuffix(e.operation, "-acquire"):
			if e.err != nil {
				return fmt.Errorf("%s acquisition failed: %v", key, e.err)
			}
			if _, exists := owned[key]; exists {
				return fmt.Errorf("duplicate acquisition %s", key)
			}
			owned[key] = e.handle
			if kind == "thread" {
				threads[e.generation] = true
			}
		case strings.HasSuffix(e.operation, "-close"):
			if e.err != nil {
				return fmt.Errorf("%s close failed: %v", key, e.err)
			}
			if h, exists := owned[key]; !exists || h != e.handle {
				return fmt.Errorf("unowned close %s", key)
			}
			if kind == "thread" && !completed[e.generation] {
				return fmt.Errorf("thread closed before read completion")
			}
			if kind == "console" && !joined[e.generation] {
				return fmt.Errorf("console closed before join")
			}
			delete(owned, key)
		case e.operation == "read-complete":
			completed[e.generation] = true
		case e.operation == "join":
			if joined[e.generation] {
				return fmt.Errorf("duplicate join")
			}
			if _, exists := owned[fmt.Sprintf("%d/thread", e.generation)]; exists {
				return fmt.Errorf("join before thread close")
			}
			joined[e.generation] = true
		}
	}
	for generation := range threads {
		if !joined[generation] {
			return fmt.Errorf("missing join generation=%d", generation)
		}
	}
	if len(owned) != 0 {
		return fmt.Errorf("unclosed prompt handles: %v", owned)
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
			defer f.Close()
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
			if err := q.verdict(); err == nil || !strings.Contains(err.Error(), failure) {
				t.Fatalf("missed %s: %v", failure, err)
			}
			if q.records[q.count-1].operation != "join" {
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
	defer f.Close()
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
		if e.operation == "thread-cancel" {
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
}
