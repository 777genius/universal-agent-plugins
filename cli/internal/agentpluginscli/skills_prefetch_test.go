package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type prefetchSkillAcquirer struct {
	dir     string
	fail    map[string]bool
	block   chan struct{}
	calls   atomic.Int32
	active  atomic.Int32
	maxSeen atomic.Int32
	mu      sync.Mutex
	roots   []string
}

func (fake *prefetchSkillAcquirer) AcquireGitHubSkill(ctx context.Context, repository, commit, path string) (domain.PackageSnapshot, error) {
	fake.calls.Add(1)
	now := fake.active.Add(1)
	defer fake.active.Add(-1)
	for {
		seen := fake.maxSeen.Load()
		if now <= seen || fake.maxSeen.CompareAndSwap(seen, now) {
			break
		}
	}
	if fake.block != nil {
		select {
		case <-fake.block:
		case <-ctx.Done():
			return domain.PackageSnapshot{}, ctx.Err()
		}
	}
	if fake.fail[path] {
		return domain.PackageSnapshot{}, errors.New("fetch failed")
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	root := filepath.Join(fake.dir, fmt.Sprintf("agentplugins-package-%d", len(fake.roots)), "root")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return domain.PackageSnapshot{}, err
	}
	fake.roots = append(fake.roots, root)
	return domain.PackageSnapshot{Root: root, TreeDigest: repository + "@" + commit + "//" + path}, nil
}

func (fake *prefetchSkillAcquirer) AcquireGitHubSkillVerified(context.Context, string, string, string, string) (domain.PackageSnapshot, error) {
	return domain.PackageSnapshot{}, errors.New("not used")
}

func (fake *prefetchSkillAcquirer) remaining(t *testing.T) int {
	t.Helper()
	fake.mu.Lock()
	defer fake.mu.Unlock()
	count := 0
	for _, root := range fake.roots {
		if _, err := os.Stat(root); err == nil {
			count++
		}
	}
	return count
}

func prefetchKeys(n int) []rawSkillKey {
	keys := []rawSkillKey{}
	for i := range n {
		keys = append(keys, rawSkillKey{repository: "owner/repo", commit: "c", path: fmt.Sprintf("skills/s%d", i)})
	}
	return keys
}

func TestRawSkillPrefetchConcurrentTakeIsBoundedAndExact(t *testing.T) {
	fake := &prefetchSkillAcquirer{dir: t.TempDir()}
	keys := prefetchKeys(9)
	prefetch := startRawSkillPrefetch(t.Context(), fake, append(keys, keys[0]))
	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Go(func() {
			snapshot, ok := prefetch.take(t.Context(), key)
			if !ok || snapshot.TreeDigest != key.repository+"@"+key.commit+"//"+key.path {
				t.Errorf("take %v = %v, %v", key, snapshot.TreeDigest, ok)
			}
			if _, ok := prefetch.take(t.Context(), key); ok {
				t.Errorf("snapshot %v handed out twice", key)
			}
		})
	}
	wg.Wait()
	prefetch.close()
	if got := fake.calls.Load(); got != int32(len(keys)) {
		t.Fatalf("acquisitions = %d, want %d (duplicates fetched once)", got, len(keys))
	}
	if got := fake.maxSeen.Load(); got > skillPrefetchLimit {
		t.Fatalf("concurrent acquisitions = %d, limit %d", got, skillPrefetchLimit)
	}
	// Taken snapshots belong to the caller and survive close.
	if got := fake.remaining(t); got != len(keys) {
		t.Fatalf("remaining taken snapshots = %d, want %d", got, len(keys))
	}
}

func TestRawSkillPrefetchNeverServesAnotherSourceOrFailedFetch(t *testing.T) {
	fake := &prefetchSkillAcquirer{dir: t.TempDir(), fail: map[string]bool{"skills/s1": true}}
	keys := prefetchKeys(2)
	prefetch := startRawSkillPrefetch(t.Context(), fake, keys)
	defer prefetch.close()
	for _, other := range []rawSkillKey{
		{repository: "owner/other", commit: "c", path: "skills/s0"},
		{repository: "owner/repo", commit: "d", path: "skills/s0"},
		{repository: "owner/repo", commit: "c", path: "skills/s0/nested"},
	} {
		if _, ok := prefetch.take(t.Context(), other); ok {
			t.Fatalf("prefetch served %v for a different source", other)
		}
	}
	if _, ok := prefetch.take(t.Context(), keys[1]); ok {
		t.Fatal("failed prefetch must fall back to the caller's own acquisition")
	}
	var nilPrefetch *rawSkillPrefetch
	if _, ok := nilPrefetch.take(t.Context(), keys[0]); ok {
		t.Fatal("nil prefetch served a snapshot")
	}
	nilPrefetch.close()
}

func TestRawSkillPrefetchCloseRemovesUntakenOnErrorPath(t *testing.T) {
	fake := &prefetchSkillAcquirer{dir: t.TempDir()}
	keys := prefetchKeys(5)
	prefetch := startRawSkillPrefetch(t.Context(), fake, keys)
	taken, ok := prefetch.take(t.Context(), keys[0])
	if !ok {
		t.Fatal("expected prefetched snapshot")
	}
	// Selection fails here; the deferred close must remove every untaken fetch.
	prefetch.close()
	if got := fake.remaining(t); got != 1 {
		t.Fatalf("remaining snapshots after close = %d, want only the taken one", got)
	}
	if _, err := os.Stat(taken.Root); err != nil {
		t.Fatalf("taken snapshot removed by close: %v", err)
	}
}

func TestRawSkillPrefetchCancelStopsFetchesAndCleansUp(t *testing.T) {
	fake := &prefetchSkillAcquirer{dir: t.TempDir(), block: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	keys := prefetchKeys(6)
	prefetch := startRawSkillPrefetch(ctx, fake, keys)
	cancel()
	if _, ok := prefetch.take(ctx, keys[0]); ok {
		t.Fatal("cancelled take served a snapshot")
	}
	prefetch.close()
	if got := fake.remaining(t); got != 0 {
		t.Fatalf("remaining snapshots after cancel = %d", got)
	}
}
