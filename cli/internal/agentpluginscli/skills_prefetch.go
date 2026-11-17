package agentpluginscli

import (
	"context"
	"sync"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// skillPrefetchLimit bounds concurrent raw Skill fetches against one host.
const skillPrefetchLimit = 4

type rawSkillKey struct{ repository, commit, path string }

type rawSkillFetch struct {
	done     chan struct{}
	snapshot domain.PackageSnapshot
	err      error
}

// rawSkillPrefetch starts the raw acquisitions that selection would perform one
// by one. It only moves the same AcquireGitHubSkill call with the same immutable
// arguments earlier; every result is still normalized, loaded and pinned by the
// sequential selection. A failed or unused fetch never reaches the caller: take
// falls back to its own acquisition, and close removes everything not taken.
type rawSkillPrefetch struct {
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	fetches map[rawSkillKey]*rawSkillFetch
}

func startRawSkillPrefetch(ctx context.Context, acquirer skillRawAcquirer, keys []rawSkillKey) *rawSkillPrefetch {
	ctx, cancel := context.WithCancel(ctx)
	prefetch := &rawSkillPrefetch{cancel: cancel, fetches: map[rawSkillKey]*rawSkillFetch{}}
	slots := make(chan struct{}, skillPrefetchLimit)
	for _, key := range keys {
		if _, ok := prefetch.fetches[key]; ok {
			continue
		}
		fetch := &rawSkillFetch{done: make(chan struct{})}
		prefetch.fetches[key] = fetch
		prefetch.wg.Add(1)
		go func() {
			defer prefetch.wg.Done()
			defer close(fetch.done)
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				fetch.err = ctx.Err()
				return
			}
			defer func() { <-slots }()
			fetch.snapshot, fetch.err = acquirer.AcquireGitHubSkill(ctx, key.repository, key.commit, key.path)
		}()
	}
	return prefetch
}

// take transfers ownership of a successful prefetched snapshot to the caller.
func (prefetch *rawSkillPrefetch) take(ctx context.Context, key rawSkillKey) (domain.PackageSnapshot, bool) {
	if prefetch == nil {
		return domain.PackageSnapshot{}, false
	}
	prefetch.mu.Lock()
	fetch, ok := prefetch.fetches[key]
	delete(prefetch.fetches, key)
	prefetch.mu.Unlock()
	if !ok {
		return domain.PackageSnapshot{}, false
	}
	select {
	case <-fetch.done:
	case <-ctx.Done():
		// The goroutine still owns the result; hand it back for close to remove.
		prefetch.mu.Lock()
		prefetch.fetches[key] = fetch
		prefetch.mu.Unlock()
		return domain.PackageSnapshot{}, false
	}
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
		if fetch.err == nil {
			_ = packagedigest.Remove(fetch.snapshot)
		}
		delete(prefetch.fetches, key)
	}
}
