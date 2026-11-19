package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"
)

// prefetchWindow bounds how far one prefetch may run ahead of its sequential
// consumer: acquisitions in flight plus results not yet taken. It applies per
// prefetch; concurrent prepares each get their own.
const prefetchWindow = 4

// prefetchGuard holds the environment checks shared by every prefetch. The
// zero value admits every acquisition and records nothing.
type prefetchGuard struct {
	// admit reports whether another acquisition may run concurrently now,
	// for example while enough free disk remains.
	admit func() bool
	// trace optionally records one event per key outcome.
	trace func(prefetchEvent)
}

// prefetchEvent describes how one prefetched key was resolved.
type prefetchEvent struct {
	Key      string  `json:"key"`
	Outcome  string  `json:"outcome"`
	Error    string  `json:"error_class,omitempty"`
	Seconds  float64 `json:"seconds,omitempty"`
	Waited   float64 `json:"waited_seconds,omitempty"`
	InFlight int     `json:"in_flight,omitempty"`
}

// Outcomes reported to prefetchGuard.trace.
const (
	prefetchHit      = "hit"      // served the prefetched result
	prefetchFail     = "fail"     // served the prefetched error
	prefetchMiss     = "miss"     // not started; the consumer acquires itself
	prefetchRedo     = "redo"     // environmental failure; the consumer acquires itself
	prefetchPanic    = "panic"    // acquirer panicked; the consumer acquires itself
	prefetchCanceled = "canceled" // the consumer stopped waiting
	prefetchStopped  = "stopped"  // further prefetching was stopped
)

type prefetchEntry[K comparable, V any] struct {
	key     K
	index   int
	started bool // guarded by orderedPrefetch.mu
	ctx     context.Context
	cancel  context.CancelFunc
	// abandoned marks an acquisition canceled by an earlier key's failure;
	// guarded by orderedPrefetch.mu.
	abandoned bool
	done      chan struct{}
	value     V
	err       error
	redo      bool
	panicked  bool
	inFlight  int
	seconds   float64
}

// orderedPrefetch starts acquisitions that a sequential consumer would perform
// one by one. It only moves the same call with the same immutable arguments
// earlier; the consumer still validates and records every result itself.
// Acquisitions start strictly in key order, and at most prefetchWindow of
// them are in flight or waiting to be taken; each take opens the next one.
//
// A failed acquisition is served as its error: it ran the consumer's own call
// with the same arguments, so repeating it would only double the cost of a
// failure. The first failure stops further prefetching and cancels the
// acquisitions after it in key order, which the consumer can no longer reach;
// it keeps those before it. A panic, or a failure while the guard no longer admits
// concurrent work (for example low free disk), is not served: the consumer
// repeats that acquisition alone and prefetching stops. An unstarted or
// mismatched key is a miss. close releases everything not taken.
type orderedPrefetch[K comparable, V any] struct {
	ctx         context.Context
	cancel      context.CancelFunc
	acquire     func(context.Context, K) (V, error)
	release     func(V)
	guard       prefetchGuard
	wg          sync.WaitGroup
	mu          sync.Mutex
	ordered     []*prefetchEntry[K, V]
	next        int
	outstanding int
	stopped     bool
	entries     map[K]*prefetchEntry[K, V]
}

func startOrderedPrefetch[K comparable, V any](ctx context.Context, keys []K, acquire func(context.Context, K) (V, error), release func(V), guard prefetchGuard) *orderedPrefetch[K, V] {
	ctx, cancel := context.WithCancel(ctx)
	prefetch := &orderedPrefetch[K, V]{ctx: ctx, cancel: cancel, acquire: acquire, release: release, guard: guard, entries: map[K]*prefetchEntry[K, V]{}}
	for _, key := range keys {
		if _, ok := prefetch.entries[key]; ok {
			continue
		}
		entry := &prefetchEntry[K, V]{key: key, index: len(prefetch.ordered), done: make(chan struct{})}
		prefetch.entries[key] = entry
		prefetch.ordered = append(prefetch.ordered, entry)
	}
	prefetch.mu.Lock()
	prefetch.advanceLocked()
	prefetch.mu.Unlock()
	return prefetch
}

func (prefetch *orderedPrefetch[K, V]) admits() bool {
	return prefetch.guard.admit == nil || prefetch.guard.admit()
}

func (prefetch *orderedPrefetch[K, V]) record(event prefetchEvent) {
	if prefetch.guard.trace != nil {
		prefetch.guard.trace(event)
	}
}

// stopLocked prevents any further acquisition from starting.
func (prefetch *orderedPrefetch[K, V]) stopLocked(reason string) {
	if !prefetch.stopped {
		prefetch.stopped = true
		prefetch.record(prefetchEvent{Outcome: prefetchStopped, Error: reason})
	}
}

// advanceLocked starts the next acquisitions in key order while the window has
// room. Entries already claimed by take are skipped.
func (prefetch *orderedPrefetch[K, V]) advanceLocked() {
	for !prefetch.stopped && prefetch.outstanding < prefetchWindow && prefetch.next < len(prefetch.ordered) && prefetch.ctx.Err() == nil {
		entry := prefetch.ordered[prefetch.next]
		if prefetch.entries[entry.key] != entry {
			prefetch.next++
			continue
		}
		if !prefetch.admits() {
			prefetch.stopLocked("not_admitted")
			return
		}
		prefetch.next++
		entry.started = true
		entry.ctx, entry.cancel = context.WithCancel(prefetch.ctx)
		prefetch.outstanding++
		entry.inFlight = prefetch.outstanding
		prefetch.wg.Add(1)
		go prefetch.run(entry)
	}
}

func (prefetch *orderedPrefetch[K, V]) run(entry *prefetchEntry[K, V]) {
	begin := time.Now()
	defer prefetch.wg.Done()
	defer close(entry.done)
	defer func() {
		entry.cancel()
		entry.seconds = time.Since(begin).Seconds()
		recovered := recover()
		if recovered != nil {
			// A panicking acquirer must not kill a long-lived server process; the
			// consumer then performs its own acquisition exactly as before.
			var zero V
			entry.value, entry.err, entry.redo, entry.panicked = zero, fmt.Errorf("prefetch panicked: %v", recovered), true, true
		}
		if entry.err == nil {
			return
		}
		prefetch.mu.Lock()
		defer prefetch.mu.Unlock()
		if prefetch.ctx.Err() != nil {
			return // closed: nothing is taken any more
		}
		if recovered == nil && entry.abandoned {
			// Canceled by an earlier key's failure; never served.
			entry.redo = true
			return
		}
		if recovered == nil && (errors.Is(entry.err, syscall.ENOSPC) || !prefetch.admits()) {
			// The failure may come from concurrent pressure such as a full
			// disk; the consumer repeats it alone.
			entry.redo = true
		}
		prefetch.stopLocked(errorClass(entry.err))
		for _, later := range prefetch.ordered[entry.index+1:] {
			if later.started {
				later.abandoned = true
				later.cancel()
			}
		}
	}()
	entry.value, entry.err = prefetch.acquire(entry.ctx, entry.key)
}

// take transfers ownership of a prefetched result to the caller. served is
// false when the caller must acquire itself; when served, err is the error the
// caller's own acquisition reported in the prefetch.
func (prefetch *orderedPrefetch[K, V]) take(ctx context.Context, key K) (value V, served bool, err error) {
	var zero V
	if prefetch == nil {
		return zero, false, nil
	}
	prefetch.mu.Lock()
	entry, ok := prefetch.entries[key]
	if !ok {
		prefetch.mu.Unlock()
		return zero, false, nil
	}
	delete(prefetch.entries, key)
	if !entry.started {
		// Not reached yet: the caller acquires itself and the window moves on.
		prefetch.advanceLocked()
		prefetch.mu.Unlock()
		prefetch.record(prefetchEvent{Key: fmt.Sprint(key), Outcome: prefetchMiss})
		return zero, false, nil
	}
	prefetch.mu.Unlock()
	waitStart := time.Now()
	select {
	case <-entry.done:
	case <-ctx.Done():
		// The goroutine still owns the result; hand it back for close to release.
		prefetch.mu.Lock()
		prefetch.entries[key] = entry
		prefetch.mu.Unlock()
		prefetch.record(prefetchEvent{Key: fmt.Sprint(key), Outcome: prefetchCanceled, InFlight: entry.inFlight})
		return zero, false, nil
	}
	prefetch.mu.Lock()
	prefetch.outstanding--
	prefetch.advanceLocked()
	prefetch.mu.Unlock()
	event := prefetchEvent{Key: fmt.Sprint(key), Seconds: entry.seconds, Waited: time.Since(waitStart).Seconds(), InFlight: entry.inFlight}
	switch {
	case entry.err == nil:
		event.Outcome = prefetchHit
		prefetch.record(event)
		return entry.value, true, nil
	case entry.redo:
		event.Outcome, event.Error = prefetchRedo, errorClass(entry.err)
		if entry.panicked {
			event.Outcome, event.Error = prefetchPanic, "panic"
		}
		prefetch.record(event)
		return zero, false, nil
	default:
		event.Outcome, event.Error = prefetchFail, errorClass(entry.err)
		prefetch.record(event)
		return zero, true, entry.err
	}
}

func (prefetch *orderedPrefetch[K, V]) close() {
	if prefetch == nil {
		return
	}
	prefetch.cancel()
	prefetch.wg.Wait()
	prefetch.mu.Lock()
	defer prefetch.mu.Unlock()
	for key, entry := range prefetch.entries {
		if entry.started && entry.err == nil {
			prefetch.release(entry.value)
		}
		delete(prefetch.entries, key)
	}
}
