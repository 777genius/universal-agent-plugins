package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
)

// countingAcquire records each key it is asked for and fails or panics on
// request.
type countingAcquire struct {
	mu     sync.Mutex
	calls  []int
	fail   map[int]error
	panics map[int]bool
	// gate holds every acquisition until it is closed.
	gate chan struct{}
}

func (acquire *countingAcquire) run(ctx context.Context, key int) (string, error) {
	acquire.mu.Lock()
	acquire.calls = append(acquire.calls, key)
	acquire.mu.Unlock()
	if acquire.gate != nil {
		select {
		case <-acquire.gate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if acquire.panics[key] {
		panic("acquirer bug")
	}
	if err := acquire.fail[key]; err != nil {
		return "", err
	}
	return fmt.Sprint("value-", key), nil
}

func (acquire *countingAcquire) started() []int {
	acquire.mu.Lock()
	defer acquire.mu.Unlock()
	return slices.Clone(acquire.calls)
}

type traceRecorder struct {
	mu     sync.Mutex
	events []prefetchEvent
}

func (recorder *traceRecorder) record(event prefetchEvent) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.events = append(recorder.events, event)
}

func (recorder *traceRecorder) outcomes() map[string]string {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	out := map[string]string{}
	for _, event := range recorder.events {
		if event.Key != "" {
			out[event.Key] = event.Outcome
		}
	}
	return out
}

func TestOrderedPrefetchServesFailureOnceAndStopsStartingNewWork(t *testing.T) {
	acquire := &countingAcquire{fail: map[int]error{1: errors.New("remote failed")}, gate: make(chan struct{})}
	keys := []int{0, 1, 2, 3, 4, 5, 6, 7}
	prefetch := startOrderedPrefetch(t.Context(), keys, acquire.run, func(string) {}, prefetchGuard{})
	defer prefetch.close()
	waitFor(t, "first window", func() bool { return len(acquire.started()) == prefetchWindow })
	close(acquire.gate)
	waitFor(t, "failure to stop prefetching", func() bool {
		prefetch.mu.Lock()
		defer prefetch.mu.Unlock()
		return prefetch.stopped
	})
	if value, served, err := prefetch.take(t.Context(), 0); !served || err != nil || value != "value-0" {
		t.Fatalf("take 0 = %q %v %v", value, served, err)
	}
	if _, served, err := prefetch.take(t.Context(), 1); !served || err == nil || err.Error() != "remote failed" {
		t.Fatalf("failure must be served as is: served=%v err=%v", served, err)
	}
	// After the failure nothing new starts: later keys are misses for the
	// consumer, which would stop at the failure anyway.
	if _, served, _ := prefetch.take(t.Context(), 5); served {
		t.Fatal("key started after a failure")
	}
	got := acquire.started()
	slices.Sort(got)
	if !slices.Equal(got, []int{0, 1, 2, 3}) {
		t.Fatalf("acquisitions after failure = %v, want only the first window", got)
	}
}

func TestOrderedPrefetchFailureCancelsOnlyLaterKeys(t *testing.T) {
	release := make(chan struct{})
	var canceled sync.Map
	run := func(ctx context.Context, key int) (string, error) {
		if key == 1 {
			return "", errors.New("remote failed")
		}
		select {
		case <-ctx.Done():
			canceled.Store(key, true)
			return "", ctx.Err()
		case <-release:
			return fmt.Sprint("value-", key), nil
		}
	}
	prefetch := startOrderedPrefetch(t.Context(), []int{0, 1, 2, 3}, run, func(string) {}, prefetchGuard{})
	defer prefetch.close()
	waitFor(t, "later keys to be canceled by the failure", func() bool {
		_, two := canceled.Load(2)
		_, three := canceled.Load(3)
		return two && three
	})
	if _, earlier := canceled.Load(0); earlier {
		t.Fatal("a failure canceled an earlier key the consumer still needs")
	}
	close(release)
	if value, served, err := prefetch.take(t.Context(), 0); !served || err != nil || value != "value-0" {
		t.Fatalf("earlier key = %q %v %v", value, served, err)
	}
	if _, served, err := prefetch.take(t.Context(), 1); !served || err == nil {
		t.Fatalf("failure = %v %v", served, err)
	}
	if _, served, err := prefetch.take(t.Context(), 2); served || err != nil {
		t.Fatalf("abandoned later key must be repeated by the consumer: %v %v", served, err)
	}
}

func TestOrderedPrefetchRedoesPanicAndEnvironmentalFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		acquire *countingAcquire
		admit   func(*atomic.Bool) func() bool
		outcome string
	}{
		{name: "panic", acquire: &countingAcquire{panics: map[int]bool{0: true}}, outcome: prefetchPanic},
		{name: "no space", acquire: &countingAcquire{fail: map[int]error{0: fmt.Errorf("write: %w", syscall.ENOSPC)}}, outcome: prefetchRedo},
		{name: "low disk at failure", acquire: &countingAcquire{fail: map[int]error{0: errors.New("opaque git failure")}}, outcome: prefetchRedo,
			admit: func(failing *atomic.Bool) func() bool { return func() bool { return !failing.Load() } }},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := &traceRecorder{}
			var lowDisk atomic.Bool
			guard := prefetchGuard{trace: recorder.record}
			run := test.acquire.run
			if test.admit != nil {
				guard.admit = test.admit(&lowDisk)
				run = func(ctx context.Context, key int) (string, error) {
					value, err := test.acquire.run(ctx, key)
					if err != nil {
						lowDisk.Store(true)
					}
					return value, err
				}
			}
			prefetch := startOrderedPrefetch(t.Context(), []int{0, 1, 2}, run, func(string) {}, guard)
			defer prefetch.close()
			if _, served, err := prefetch.take(t.Context(), 0); served || err != nil {
				t.Fatalf("%s must be repeated by the consumer: served=%v err=%v", test.name, served, err)
			}
			if got := recorder.outcomes()["0"]; got != test.outcome {
				t.Fatalf("trace outcome = %q, want %q", got, test.outcome)
			}
		})
	}
}

func TestOrderedPrefetchStartsNothingWhileNotAdmitted(t *testing.T) {
	acquire := &countingAcquire{}
	recorder := &traceRecorder{}
	prefetch := startOrderedPrefetch(t.Context(), []int{0, 1, 2}, acquire.run, func(string) {}, prefetchGuard{admit: func() bool { return false }, trace: recorder.record})
	defer prefetch.close()
	if _, served, _ := prefetch.take(t.Context(), 0); served {
		t.Fatal("served a key that was never admitted")
	}
	if got := acquire.started(); len(got) != 0 {
		t.Fatalf("acquisitions without admission = %v", got)
	}
	if got := recorder.outcomes()["0"]; got != prefetchMiss {
		t.Fatalf("trace outcome = %q, want miss", got)
	}
}

func TestOrderedPrefetchTracesEveryOutcome(t *testing.T) {
	acquire := &countingAcquire{fail: map[int]error{1: context.DeadlineExceeded}}
	recorder := &traceRecorder{}
	prefetch := startOrderedPrefetch(t.Context(), []int{0, 1}, acquire.run, func(string) {}, prefetchGuard{trace: recorder.record})
	defer prefetch.close()
	_, _, _ = prefetch.take(t.Context(), 0)
	_, _, _ = prefetch.take(t.Context(), 1)
	if got := recorder.outcomes(); got["0"] != prefetchHit || got["1"] != prefetchFail {
		t.Fatalf("trace outcomes = %v", got)
	}
	for _, event := range recorder.events {
		if event.Key == "1" && (event.Error != "deadline" || event.InFlight < 1) {
			t.Fatalf("failure event lacks class or in-flight count: %+v", event)
		}
	}
}

func TestSkillDirectoryMismatch(t *testing.T) {
	for _, test := range []struct {
		skill, directory string
		mismatch         bool
	}{
		{"review", "", false},
		{"review", "review", false},
		{"review", "skills/review", false},
		{"review", "skills/other", true},
		{"review", "skills/review/nested", true},
	} {
		if got := skillDirectoryMismatch(test.skill, test.directory); got != test.mismatch {
			t.Errorf("skillDirectoryMismatch(%q, %q) = %v", test.skill, test.directory, got)
		}
	}
}
