package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type prefetchSkillAcquirer struct {
	dir     string
	fail    map[string]bool
	panics  map[string]bool
	block   chan struct{}
	ignore  bool // keep blocking after cancellation, then succeed
	calls   atomic.Int32
	active  atomic.Int32
	maxSeen atomic.Int32
	mu      sync.Mutex
	started []string
	roots   []string
}

func (fake *prefetchSkillAcquirer) AcquireGitHubSkill(ctx context.Context, repository, commit, path string) (domain.PackageSnapshot, error) {
	fake.calls.Add(1)
	fake.mu.Lock()
	fake.started = append(fake.started, path)
	fake.mu.Unlock()
	now := fake.active.Add(1)
	defer fake.active.Add(-1)
	for {
		seen := fake.maxSeen.Load()
		if now <= seen || fake.maxSeen.CompareAndSwap(seen, now) {
			break
		}
	}
	if fake.panics[path] {
		panic("acquirer bug")
	}
	if fake.block != nil {
		if fake.ignore {
			<-fake.block
		} else {
			select {
			case <-fake.block:
			case <-ctx.Done():
				return domain.PackageSnapshot{}, ctx.Err()
			}
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

func (fake *prefetchSkillAcquirer) remaining() int {
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

func (fake *prefetchSkillAcquirer) startedPaths() []string {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return append([]string{}, fake.started...)
}

func prefetchKeys(n int) []rawSkillKey {
	keys := make([]rawSkillKey, 0, n)
	for i := range n {
		keys = append(keys, rawSkillKey{repository: "owner/repo", commit: "c", path: fmt.Sprintf("skills/s%d", i)})
	}
	return keys
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func closeWithin(t *testing.T, prefetch *rawSkillPrefetch) {
	t.Helper()
	closed := make(chan struct{})
	go func() { prefetch.close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("close did not stop in-flight prefetches")
	}
}

func TestRawSkillPrefetchStartsInOrderWithinWindow(t *testing.T) {
	fake := &prefetchSkillAcquirer{dir: t.TempDir()}
	keys := prefetchKeys(12)
	prefetch := startRawSkillPrefetch(t.Context(), fake, append(keys, keys[0]))
	defer prefetch.close()
	index := func(path string) int {
		return slices.IndexFunc(keys, func(key rawSkillKey) bool { return key.path == path })
	}
	for taken, key := range keys {
		if taken == 0 {
			waitFor(t, "first window", func() bool { return fake.calls.Load() == prefetchWindow })
		}
		for _, path := range fake.startedPaths() {
			if index(path) >= taken+prefetchWindow {
				t.Fatalf("fetch %s started before %d earlier snapshots were taken", path, index(path)-prefetchWindow+1)
			}
		}
		snapshot, ok := prefetch.take(t.Context(), key)
		if !ok || snapshot.TreeDigest != key.repository+"@"+key.commit+"//"+key.path {
			t.Fatalf("take %v = %v, %v", key, snapshot.TreeDigest, ok)
		}
		if _, ok := prefetch.take(t.Context(), key); ok {
			t.Fatalf("snapshot %v handed out twice", key)
		}
	}
	if got := fake.calls.Load(); got != int32(len(keys)) {
		t.Fatalf("acquisitions = %d, want %d (duplicates fetched once)", got, len(keys))
	}
	if got := fake.remaining(); got != len(keys) {
		t.Fatalf("taken snapshots must belong to the caller: remaining %d", got)
	}
}

func TestRawSkillPrefetchFillsButNeverExceedsWindow(t *testing.T) {
	fake := &prefetchSkillAcquirer{dir: t.TempDir(), block: make(chan struct{})}
	prefetch := startRawSkillPrefetch(t.Context(), fake, prefetchKeys(9))
	waitFor(t, "window to fill", func() bool { return fake.active.Load() == prefetchWindow })
	time.Sleep(20 * time.Millisecond)
	if got := fake.calls.Load(); got != prefetchWindow || fake.maxSeen.Load() != prefetchWindow || prefetchWindow < 2 {
		t.Fatalf("calls=%d peak=%d, want exactly the window %d", got, fake.maxSeen.Load(), prefetchWindow)
	}
	close(fake.block)
	closeWithin(t, prefetch)
	if got := fake.remaining(); got != 0 {
		t.Fatalf("untaken snapshots after close = %d", got)
	}
}

func TestRawSkillPrefetchNeverServesAnotherSourceOrFailedFetch(t *testing.T) {
	fake := &prefetchSkillAcquirer{dir: t.TempDir(), fail: map[string]bool{"skills/s1": true}, panics: map[string]bool{"skills/s2": true}}
	keys := prefetchKeys(3)
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
	if _, ok := prefetch.take(t.Context(), keys[2]); ok {
		t.Fatal("panicking prefetch must fall back to the caller's own acquisition")
	}
	var nilPrefetch *rawSkillPrefetch
	if _, ok := nilPrefetch.take(t.Context(), keys[0]); ok {
		t.Fatal("nil prefetch served a snapshot")
	}
	nilPrefetch.close()
}

func TestRawSkillPrefetchCloseRemovesUntakenOnErrorPath(t *testing.T) {
	fake := &prefetchSkillAcquirer{dir: t.TempDir()}
	keys := prefetchKeys(3)
	prefetch := startRawSkillPrefetch(t.Context(), fake, keys)
	taken, ok := prefetch.take(t.Context(), keys[0])
	if !ok {
		t.Fatal("expected prefetched snapshot")
	}
	waitFor(t, "remaining fetches", func() bool { return fake.remaining() == len(keys) })
	// Selection fails here; the deferred close must remove every untaken fetch.
	prefetch.close()
	if got := fake.remaining(); got != 1 {
		t.Fatalf("remaining snapshots after close = %d, want only the taken one", got)
	}
	if _, err := os.Stat(taken.Root); err != nil {
		t.Fatalf("taken snapshot removed by close: %v", err)
	}
}

func TestRawSkillPrefetchCloseStopsBlockedFetchesWithLiveParent(t *testing.T) {
	fake := &prefetchSkillAcquirer{dir: t.TempDir(), block: make(chan struct{})}
	prefetch := startRawSkillPrefetch(t.Context(), fake, prefetchKeys(6))
	waitFor(t, "fetches in flight", func() bool { return fake.active.Load() == prefetchWindow })
	closeWithin(t, prefetch)
	if got := fake.remaining(); got != 0 {
		t.Fatalf("remaining snapshots after close = %d", got)
	}
}

func TestRawSkillPrefetchRemovesSuccessArrivingAfterCancel(t *testing.T) {
	fake := &prefetchSkillAcquirer{dir: t.TempDir(), block: make(chan struct{}), ignore: true}
	keys := prefetchKeys(2)
	prefetch := startRawSkillPrefetch(t.Context(), fake, keys)
	waitFor(t, "fetches in flight", func() bool { return fake.active.Load() == 2 })
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, ok := prefetch.take(canceled, keys[0]); ok {
		t.Fatal("canceled take served a snapshot")
	}
	closed := make(chan struct{})
	go func() { prefetch.close(); close(closed) }()
	close(fake.block)
	<-closed
	if got, roots := fake.remaining(), len(fake.roots); got != 0 || roots != 2 {
		t.Fatalf("late successful fetch leaked: remaining=%d acquired=%d", got, roots)
	}
}

// countingSkillSource counts raw Skill fetches made through the real acquirer.
type countingSkillSource struct {
	skillsource.Acquirer
	mu       sync.Mutex
	calls    []rawSkillKey
	fail     string
	onSecond func()
	// failAfter makes the failing fetch wait until this many fetches succeeded,
	// so close must remove completed, untaken prefetches.
	failAfter int32
	succeeded atomic.Int32
}

func (source *countingSkillSource) AcquireGitHubSkill(ctx context.Context, repository, commit, path string) (domain.PackageSnapshot, error) {
	source.mu.Lock()
	source.calls = append(source.calls, rawSkillKey{repository, commit, path})
	n := len(source.calls)
	source.mu.Unlock()
	if n == 2 && source.onSecond != nil {
		source.onSecond()
	}
	if path == source.fail {
		deadline := time.Now().Add(30 * time.Second)
		for source.succeeded.Load() < source.failAfter && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		return domain.PackageSnapshot{}, errors.New("fetch failed")
	}
	snapshot, err := source.Acquirer.AcquireGitHubSkill(ctx, repository, commit, path)
	if err == nil {
		source.succeeded.Add(1)
	}
	return snapshot, err
}

func (source *countingSkillSource) keys() []rawSkillKey {
	source.mu.Lock()
	defer source.mu.Unlock()
	return append([]rawSkillKey{}, source.calls...)
}

func multiSkillJourney(t *testing.T, names []string) (savedJourney, *countingSkillSource, string) {
	t.Helper()
	repo := t.TempDir()
	for _, name := range names {
		dir := filepath.Join(repo, "skills", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: Handle " + name + " tasks\n---\nUse " + name + ".\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	savedJourneyGit(t, repo, "init", "-b", "main")
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "fixture")
	sha := savedJourneyGit(t, repo, "rev-parse", "HEAD")
	j := newSavedJourney(t, repo)
	source := &countingSkillSource{Acquirer: skillsource.Acquirer{Acquirer: j.source.Acquirer}}
	j.cli.app.SourceAcquirer = source
	return j, source, sha
}

func skillSelections(repository, sha string, names ...string) []SkillsSelection {
	selections := make([]SkillsSelection, 0, len(names))
	for _, name := range names {
		selections = append(selections, SkillsSelection{Repository: repository, Commit: sha, Path: "skills/" + name, Name: name})
	}
	return selections
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary acquisitions left behind in %s: %d entries", dir, len(entries))
	}
}

func TestPrepareSkillsPrefetchFetchesEachSelectedSkillOnce(t *testing.T) {
	names := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota", "kappa", "lambda"}
	j, source, sha := multiSkillJourney(t, names)
	saved, err := j.cli.app.PrepareSkills(t.Context(), SkillsBatchRequest{Selections: skillSelections("acme/tools", sha, "alpha"), TargetIDs: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := saved.Apply(t.Context()); err != nil {
		t.Fatal(err)
	}
	saved.Close()
	source.calls = nil
	// Mixed-case repository, a second repository and a re-add of the saved Skill.
	selections := append(skillSelections("Acme/Tools", sha, names...), skillSelections("other/repo", sha, "beta")...)
	selections[len(selections)-1].Alias = "beta-other"
	plan, err := j.cli.app.PrepareSkills(t.Context(), SkillsBatchRequest{Selections: selections, TargetIDs: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	want := make([]rawSkillKey, 0, len(names))
	for _, name := range names[1:] {
		want = append(want, rawSkillKey{"acme/tools", sha, "skills/" + name})
	}
	want = append(want, rawSkillKey{"other/repo", sha, "skills/beta"})
	got := source.keys()
	slices.SortFunc(got, func(a, b rawSkillKey) int { return strings.Compare(a.repository+a.path, b.repository+b.path) })
	slices.SortFunc(want, func(a, b rawSkillKey) int { return strings.Compare(a.repository+a.path, b.repository+b.path) })
	if !slices.Equal(got, want) {
		t.Fatalf("raw Skill fetches = %v, want exactly one per new selection %v", got, want)
	}
	if len(plan.actions) != len(selections) {
		t.Fatalf("planned %d actions for %d selections", len(plan.actions), len(selections))
	}
}

func TestPrepareSkillsPrefetchCleansUpWhenFirstSkillFails(t *testing.T) {
	names := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	j, source, sha := multiSkillJourney(t, names)
	source.fail, source.failAfter = "skills/alpha", prefetchWindow-1
	if _, err := j.cli.app.PrepareSkills(t.Context(), SkillsBatchRequest{Selections: skillSelections("acme/tools", sha, names...), TargetIDs: []string{"codex"}}); err == nil {
		t.Fatal("expected the failing first Skill to abort prepare")
	}
	if got := source.succeeded.Load(); got != prefetchWindow-1 {
		t.Fatalf("completed prefetches = %d, want %d to be removed by close", got, prefetchWindow-1)
	}
	assertEmptyDir(t, j.source.TempRoot)
}

func TestPrepareSkillsPrefetchCleansUpOnCancel(t *testing.T) {
	names := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"}
	j, source, sha := multiSkillJourney(t, names)
	ctx, cancel := context.WithCancel(t.Context())
	source.onSecond = cancel
	if _, err := j.cli.app.PrepareSkills(ctx, SkillsBatchRequest{Selections: skillSelections("acme/tools", sha, names...), TargetIDs: []string{"codex"}}); err == nil {
		t.Fatal("expected cancellation to abort prepare")
	}
	assertEmptyDir(t, j.source.TempRoot)
}

func TestPrepareSkillsPrefetchSkipsSelectionRejectedBeforeDownload(t *testing.T) {
	j, source, sha := multiSkillJourney(t, []string{"alpha", "beta", "gamma", "delta"})
	selections := skillSelections("acme/tools", sha, "alpha", "gamma", "delta")
	selections[0].Path = "skills/beta"
	if _, err := j.cli.app.PrepareSkills(t.Context(), SkillsBatchRequest{Selections: selections, TargetIDs: []string{"codex"}}); err == nil {
		t.Fatal("expected a Skill name that differs from its directory to be rejected")
	}
	for _, key := range source.keys() {
		if key.path == "skills/beta" {
			t.Fatal("prefetch downloaded a selection that is rejected before download")
		}
	}
	assertEmptyDir(t, j.source.TempRoot)
}
