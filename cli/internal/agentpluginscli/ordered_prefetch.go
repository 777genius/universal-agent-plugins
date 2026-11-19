package agentpluginscli

import (
	"context"
	"fmt"
	"sync"
)

// prefetchWindow bounds how far one prefetch may run ahead of its sequential
// consumer: acquisitions in flight plus results not yet taken. It applies per
// prefetch; concurrent prepares each get their own.
const prefetchWindow = 4

type prefetchEntry[K comparable, V any] struct {
	key     K
	started bool // guarded by orderedPrefetch.mu
	done    chan struct{}
	value   V
	err     error
}

// orderedPrefetch starts acquisitions that a sequential consumer would perform
// one by one. It only moves the same call with the same immutable arguments
// earlier; the consumer still validates and records every result itself.
// Acquisitions start strictly in key order, and at most prefetchWindow of
// them are in flight or waiting to be taken; each take opens the next one. A
// failed, unstarted or mismatched acquisition never reaches the consumer: take
// reports a miss so the consumer performs its own acquisition and produces the
// same result or error as without prefetch, and close releases everything not
// taken.
type orderedPrefetch[K comparable, V any] struct {
	ctx         context.Context
	cancel      context.CancelFunc
	acquire     func(context.Context, K) (V, error)
	release     func(V)
	wg          sync.WaitGroup
	mu          sync.Mutex
	ordered     []*prefetchEntry[K, V]
	next        int
	outstanding int
	entries     map[K]*prefetchEntry[K, V]
}

func startOrderedPrefetch[K comparable, V any](ctx context.Context, keys []K, acquire func(context.Context, K) (V, error), release func(V)) *orderedPrefetch[K, V] {
	ctx, cancel := context.WithCancel(ctx)
	prefetch := &orderedPrefetch[K, V]{ctx: ctx, cancel: cancel, acquire: acquire, release: release, entries: map[K]*prefetchEntry[K, V]{}}
	for _, key := range keys {
		if _, ok := prefetch.entries[key]; ok {
			continue
		}
		entry := &prefetchEntry[K, V]{key: key, done: make(chan struct{})}
		prefetch.entries[key] = entry
		prefetch.ordered = append(prefetch.ordered, entry)
	}
	prefetch.mu.Lock()
	prefetch.advanceLocked()
	prefetch.mu.Unlock()
	return prefetch
}

// advanceLocked starts the next acquisitions in key order while the window has
// room. Entries already claimed by take are skipped.
func (prefetch *orderedPrefetch[K, V]) advanceLocked() {
	for prefetch.outstanding < prefetchWindow && prefetch.next < len(prefetch.ordered) && prefetch.ctx.Err() == nil {
		entry := prefetch.ordered[prefetch.next]
		prefetch.next++
		if prefetch.entries[entry.key] != entry {
			continue
		}
		entry.started = true
		prefetch.outstanding++
		prefetch.wg.Add(1)
		go prefetch.run(entry)
	}
}

func (prefetch *orderedPrefetch[K, V]) run(entry *prefetchEntry[K, V]) {
	defer prefetch.wg.Done()
	defer close(entry.done)
	defer func() {
		// A panicking acquirer must not kill a long-lived server process; the
		// consumer then performs its own acquisition exactly as before.
		if recovered := recover(); recovered != nil {
			var zero V
			entry.value, entry.err = zero, fmt.Errorf("prefetch panicked: %v", recovered)
		}
	}()
	entry.value, entry.err = prefetch.acquire(prefetch.ctx, entry.key)
}

// take transfers ownership of a successful prefetched result to the caller.
func (prefetch *orderedPrefetch[K, V]) take(ctx context.Context, key K) (V, bool) {
	var zero V
	if prefetch == nil {
		return zero, false
	}
	prefetch.mu.Lock()
	entry, ok := prefetch.entries[key]
	if !ok {
		prefetch.mu.Unlock()
		return zero, false
	}
	delete(prefetch.entries, key)
	if !entry.started {
		// Not reached yet: the caller acquires itself and the window moves on.
		prefetch.advanceLocked()
		prefetch.mu.Unlock()
		return zero, false
	}
	prefetch.mu.Unlock()
	select {
	case <-entry.done:
	case <-ctx.Done():
		// The goroutine still owns the result; hand it back for close to release.
		prefetch.mu.Lock()
		prefetch.entries[key] = entry
		prefetch.mu.Unlock()
		return zero, false
	}
	prefetch.mu.Lock()
	prefetch.outstanding--
	prefetch.advanceLocked()
	prefetch.mu.Unlock()
	if entry.err != nil {
		return zero, false
	}
	return entry.value, true
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
