package agentpluginscli

import (
	"context"
	"fmt"
	"sync"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// skillPrefetchWindow bounds how far one selection pass may run ahead of its
// sequential consumer: fetches in flight plus fetched snapshots not yet taken.
// It applies per selectSkills call; concurrent prepares each get their own.
const skillPrefetchWindow = 4

type rawSkillKey struct{ repository, commit, path string }

type rawSkillFetch struct {
	key      rawSkillKey
	started  bool // guarded by rawSkillPrefetch.mu
	done     chan struct{}
	snapshot domain.PackageSnapshot
	err      error
}

// rawSkillPrefetch starts the raw acquisitions that selection would perform one
// by one. It only moves the same AcquireGitHubSkill call with the same immutable
// arguments earlier; every result is still normalized, loaded and pinned by the
// sequential selection. Fetches start strictly in selection order, and at most
// skillPrefetchWindow of them are in flight or waiting to be taken; each take
// opens the next one. A failed, unstarted or mismatched fetch never reaches the
// caller: take falls back to its own acquisition, and close removes everything
// not taken.
type rawSkillPrefetch struct {
	ctx         context.Context
	cancel      context.CancelFunc
	acquirer    skillRawAcquirer
	wg          sync.WaitGroup
	mu          sync.Mutex
	ordered     []*rawSkillFetch
	next        int
	outstanding int
	fetches     map[rawSkillKey]*rawSkillFetch
}

func startRawSkillPrefetch(ctx context.Context, acquirer skillRawAcquirer, keys []rawSkillKey) *rawSkillPrefetch {
	ctx, cancel := context.WithCancel(ctx)
	prefetch := &rawSkillPrefetch{ctx: ctx, cancel: cancel, acquirer: acquirer, fetches: map[rawSkillKey]*rawSkillFetch{}}
	for _, key := range keys {
		if _, ok := prefetch.fetches[key]; ok {
			continue
		}
		fetch := &rawSkillFetch{key: key, done: make(chan struct{})}
		prefetch.fetches[key] = fetch
		prefetch.ordered = append(prefetch.ordered, fetch)
	}
	prefetch.mu.Lock()
	prefetch.advanceLocked()
	prefetch.mu.Unlock()
	return prefetch
}

// advanceLocked starts the next fetches in selection order while the window
// has room. Fetches already claimed by take are skipped.
func (prefetch *rawSkillPrefetch) advanceLocked() {
	for prefetch.outstanding < skillPrefetchWindow && prefetch.next < len(prefetch.ordered) && prefetch.ctx.Err() == nil {
		fetch := prefetch.ordered[prefetch.next]
		prefetch.next++
		if prefetch.fetches[fetch.key] != fetch {
			continue
		}
		fetch.started = true
		prefetch.outstanding++
		prefetch.wg.Add(1)
		go prefetch.run(fetch)
	}
}

func (prefetch *rawSkillPrefetch) run(fetch *rawSkillFetch) {
	defer prefetch.wg.Done()
	defer close(fetch.done)
	defer func() {
		// A panicking acquirer must not kill a long-lived server process; the
		// consumer then performs its own acquisition exactly as before.
		if recovered := recover(); recovered != nil {
			fetch.snapshot, fetch.err = domain.PackageSnapshot{}, fmt.Errorf("skill prefetch panicked: %v", recovered)
		}
	}()
	fetch.snapshot, fetch.err = prefetch.acquirer.AcquireGitHubSkill(prefetch.ctx, fetch.key.repository, fetch.key.commit, fetch.key.path)
}

// take transfers ownership of a successful prefetched snapshot to the caller.
func (prefetch *rawSkillPrefetch) take(ctx context.Context, key rawSkillKey) (domain.PackageSnapshot, bool) {
	if prefetch == nil {
		return domain.PackageSnapshot{}, false
	}
	prefetch.mu.Lock()
	fetch, ok := prefetch.fetches[key]
	if !ok {
		prefetch.mu.Unlock()
		return domain.PackageSnapshot{}, false
	}
	delete(prefetch.fetches, key)
	if !fetch.started {
		// Not reached yet: the caller fetches itself and the window moves on.
		prefetch.advanceLocked()
		prefetch.mu.Unlock()
		return domain.PackageSnapshot{}, false
	}
	prefetch.mu.Unlock()
	select {
	case <-fetch.done:
	case <-ctx.Done():
		// The goroutine still owns the result; hand it back for close to remove.
		prefetch.mu.Lock()
		prefetch.fetches[key] = fetch
		prefetch.mu.Unlock()
		return domain.PackageSnapshot{}, false
	}
	prefetch.mu.Lock()
	prefetch.outstanding--
	prefetch.advanceLocked()
	prefetch.mu.Unlock()
	if fetch.err != nil {
		return domain.PackageSnapshot{}, false
	}
	return fetch.snapshot, true
}

func (prefetch *rawSkillPrefetch) close() {
	if prefetch == nil {
		return
	}
	prefetch.cancel()
	prefetch.wg.Wait()
	prefetch.mu.Lock()
	defer prefetch.mu.Unlock()
	for key, fetch := range prefetch.fetches {
		if fetch.started && fetch.err == nil {
			_ = packagedigest.Remove(fetch.snapshot)
		}
		delete(prefetch.fetches, key)
	}
}
