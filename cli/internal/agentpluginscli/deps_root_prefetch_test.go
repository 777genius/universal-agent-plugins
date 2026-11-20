package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// rootFetchSource counts every remote read of a Git root, named "kind
// repository//path", and can fail one read or hold the first reads until
// another one runs concurrently.
type rootFetchSource struct {
	skillsource.Acquirer
	mu       sync.Mutex
	reads    []string
	fail     string
	overlap  bool
	onRead   func(int)
	active   atomic.Int32
	maxSeen  atomic.Int32
	tempRoot string
	// failed is set once the failing read returned; roots whose first read
	// starts after it are recorded in startedAfterFailure.
	failed              atomic.Bool
	roots               map[string]bool
	startedAfterFailure []string
	// holdRefs makes every other ref read finish only after the failure, and
	// after its own cancellation when that follows; such roots are recorded
	// in canceledRefs and their later reads in readsAfterCancel.
	holdRefs         bool
	canceledRefs     map[string]bool
	readsAfterCancel []string
}

func (source *rootFetchSource) read(ctx context.Context, kind, root string) error {
	read := kind + " " + root
	source.mu.Lock()
	if source.roots == nil {
		source.roots = map[string]bool{}
	}
	if !source.roots[root] && source.failed.Load() {
		source.startedAfterFailure = append(source.startedAfterFailure, root)
	}
	if kind != "ref" && source.canceledRefs[root] {
		source.readsAfterCancel = append(source.readsAfterCancel, read)
	}
	source.roots[root] = true
	source.reads = append(source.reads, read)
	n := len(source.reads)
	source.mu.Unlock()
	if source.onRead != nil {
		source.onRead(n)
	}
	now := source.active.Add(1)
	defer source.active.Add(-1)
	for seen := source.maxSeen.Load(); now > seen && !source.maxSeen.CompareAndSwap(seen, now); seen = source.maxSeen.Load() {
	}
	if source.overlap {
		deadline := time.Now().Add(2 * time.Second)
		for source.maxSeen.Load() < 2 && time.Now().Before(deadline) && ctx.Err() == nil {
			time.Sleep(time.Millisecond)
		}
	}
	if read == source.fail {
		source.failed.Store(true)
		return errors.New("remote read failed for " + root)
	}
	if source.holdRefs && kind == "ref" {
		deadline := time.Now().Add(5 * time.Second)
		for !source.failed.Load() && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		select {
		case <-ctx.Done():
			// The ref answer arrives after the cancellation.
			source.mu.Lock()
			if source.canceledRefs == nil {
				source.canceledRefs = map[string]bool{}
			}
			source.canceledRefs[root] = true
			source.mu.Unlock()
			return nil
		case <-time.After(500 * time.Millisecond):
		}
	}
	return ctx.Err()
}

func (source *rootFetchSource) ResolveGit(ctx context.Context, spec sourcespec.Git, selector string, exact bool) (sourceacquisition.RefResolution, error) {
	root := spec.Repository + "//" + spec.Path
	if err := source.read(ctx, "ref", root); err != nil {
		return sourceacquisition.RefResolution{}, err
	}
	source.mu.Lock()
	late := source.canceledRefs[root]
	source.mu.Unlock()
	if late {
		// A ref answer that arrived after cancellation, as a remote could.
		return sourceacquisition.RefResolution{Commit: strings.Repeat("0", 40)}, nil
	}
	return source.Acquirer.ResolveGit(ctx, spec, selector, exact)
}

func (source *rootFetchSource) AcquireGit(ctx context.Context, spec sourcespec.Git, commit, digest string) (domain.PackageSnapshot, error) {
	if err := source.read(ctx, "tree", spec.Repository+"//"+spec.Path); err != nil {
		return domain.PackageSnapshot{}, err
	}
	return source.Acquirer.AcquireGit(ctx, spec, commit, digest)
}

func (source *rootFetchSource) AcquireGitHubSkill(ctx context.Context, repository, commit, path string) (domain.PackageSnapshot, error) {
	if err := source.read(ctx, "skill", repository+"//"+path); err != nil {
		return domain.PackageSnapshot{}, err
	}
	return source.Acquirer.AcquireGitHubSkill(ctx, repository, commit, path)
}

func (source *rootFetchSource) readsOf(read string) int {
	source.mu.Lock()
	defer source.mu.Unlock()
	return len(slices.DeleteFunc(slices.Clone(source.reads), func(other string) bool { return other != read }))
}

func (source *rootFetchSource) reset() {
	source.mu.Lock()
	source.reads, source.roots, source.startedAfterFailure = nil, nil, nil
	source.mu.Unlock()
	source.failed.Store(false)
	source.maxSeen.Store(0)
}

func (source *rootFetchSource) sortedReads() []string {
	source.mu.Lock()
	defer source.mu.Unlock()
	reads := slices.Clone(source.reads)
	slices.Sort(reads)
	return reads
}

// mixedRootsJourney declares three Git plugins from different repositories
// with commit, ref and default selectors, plus three Skills from two
// repositories. All repositories are served by one local fixture.
func mixedRootsJourney(t *testing.T) (App, environment.Requirements, *rootFetchSource) {
	t.Helper()
	repo := t.TempDir()
	for _, name := range []string{"pa", "pb", "pc"} {
		dir := filepath.Join(repo, "plugins", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"` + name + `","version":"1.0.0","description":"Plugin ` + name + `"}`
		if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"alpha", "beta", "gamma"} {
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
	source := &rootFetchSource{Acquirer: skillsource.Acquirer{Acquirer: j.source.Acquirer}, tempRoot: j.source.TempRoot}
	j.cli.app.SourceAcquirer = source
	plugin := func(repository, name string, selector environment.Selector) environment.Requirement {
		return environment.Requirement{Alias: name, Git: &environment.GitSource{Repository: repository, Path: "plugins/" + name, Selector: selector}}
	}
	skill := func(repository, name string) environment.Requirement {
		return environment.Requirement{Alias: name, Git: &environment.GitSource{Kind: environment.SkillKind, Skill: name, Repository: repository, Path: "skills/" + name, Selector: environment.Selector{Kind: environment.SelectorRef, Value: "main"}}}
	}
	requirements := environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{
		"pa":    plugin("acme/pa", "pa", environment.Selector{Kind: environment.SelectorRef, Value: "main"}),
		"pb":    plugin("acme/pb", "pb", environment.Selector{Kind: environment.SelectorDefault}),
		"pc":    plugin("other/pc", "pc", environment.Selector{Kind: environment.SelectorCommit, Value: sha}),
		"alpha": skill("acme/skills", "alpha"),
		"beta":  skill("acme/skills", "beta"),
		"gamma": skill("other/skills", "gamma"),
	}, Targets: map[string][]string{"codex": {"alpha", "beta", "gamma", "pa", "pb", "pc"}}}
	return j.cli.app, requirements, source
}

func resolveMixedRoots(t *testing.T, ctx context.Context, app App, requirements environment.Requirements, previous []byte, serial bool) ([]byte, error) {
	t.Helper()
	preview, cleanup, err := dependencyPreviewApp(app, &options{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var locked environment.Lock
	if serial {
		// The resolution resolveDependencyLock performs, without the prefetch.
		var prior environment.Lock
		if previous != nil {
			if prior, err = decodeCompatibleDependencyLockBytes(previous, requirements); err != nil {
				t.Fatal(err)
			}
		}
		if err := canonicalizeRequirementsFromLock(&requirements, prior); err != nil {
			t.Fatal(err)
		}
		if requirements, err = preview.canonicalizeDependencyRequirements(ctx, requirements); err != nil {
			t.Fatal(err)
		}
		resolver := dependencyResolver{app: preview, requirements: requirements}
		locked, err = (environment.Coordinator{EnvironmentID: preview.environmentID(), Resolver: resolver}).Resolve(ctx, requirements, prior, "")
	} else {
		_, _, locked, err = resolveDependencyLock(ctx, preview, &options{}, requirements, previous, "")
	}
	if err != nil {
		return nil, err
	}
	return encodeDependencyLockBytes(locked)
}

func TestGitRootPrefetchProducesTheSerialLockWithOneReadPerRoot(t *testing.T) {
	app, requirements, source := mixedRootsJourney(t)
	serial, err := resolveMixedRoots(t, t.Context(), app, requirements, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	serialReads := source.sortedReads()
	source.reads, source.overlap = nil, true
	parallel, err := resolveMixedRoots(t, t.Context(), app, requirements, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(parallel) != string(serial) {
		t.Fatalf("prefetched lock differs from the serial lock:\n%s\n---\n%s", parallel, serial)
	}
	if got := source.sortedReads(); !slices.Equal(got, serialReads) {
		t.Fatalf("remote reads = %v, want exactly the serial reads %v", got, serialReads)
	}
	if source.maxSeen.Load() < 2 {
		t.Fatal("independent roots were not read concurrently")
	}
	// Roots pinned by the previous lock are not prefetched: the pinned
	// resolution reads exactly what the serial one reads.
	source.reads, source.overlap = nil, false
	if _, err := resolveMixedRoots(t, t.Context(), app, requirements, parallel, true); err != nil {
		t.Fatal(err)
	}
	pinnedReads := source.sortedReads()
	source.reads = nil
	again, err := resolveMixedRoots(t, t.Context(), app, requirements, parallel, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := source.sortedReads(); string(again) != string(serial) || !slices.Equal(got, pinnedReads) {
		t.Fatalf("pinned resolution changed the lock or reads: %v, want %v", got, pinnedReads)
	}
	assertEmptyDir(t, source.tempRoot)
}

// install and sync resolve a missing or stale lock through
// resolveDependencySyncCandidate; that path must use the same prefetch.
func TestGitRootPrefetchServesInstallAndSyncResolution(t *testing.T) {
	app, requirements, source := mixedRootsJourney(t)
	serial, err := resolveMixedRoots(t, t.Context(), app, requirements, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	serialReads := source.sortedReads()
	source.reads, source.overlap = nil, true
	preview, cleanup, err := dependencyPreviewApp(app, &options{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	_, locked, err := resolveDependencySyncCandidate(t.Context(), preview, &options{}, requirements, environment.Lock{}, false)
	if err != nil {
		t.Fatal(err)
	}
	body, err := encodeDependencyLockBytes(locked)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(serial) {
		t.Fatalf("install/sync lock differs from the serial lock:\n%s\n---\n%s", body, serial)
	}
	if got := source.sortedReads(); !slices.Equal(got, serialReads) {
		t.Fatalf("install/sync remote reads = %v, want %v", got, serialReads)
	}
	if source.maxSeen.Load() < 2 {
		t.Fatal("install/sync resolution did not read independent roots concurrently")
	}
	assertEmptyDir(t, source.tempRoot)
}

func TestGitRootPrefetchFailingSourceReturnsTheSerialError(t *testing.T) {
	for _, failing := range []string{"ref acme/pb//plugins/pb", "skill acme/skills//skills/beta", "tree other/pc//plugins/pc"} {
		t.Run(failing, func(t *testing.T) {
			app, requirements, source := mixedRootsJourney(t)
			source.fail = failing
			_, serialErr := resolveMixedRoots(t, t.Context(), app, requirements, nil, true)
			if serialErr == nil {
				t.Fatal("expected the failing source to fail serial resolution")
			}
			serialReads, serialFailing := source.sortedReads(), source.readsOf(failing)
			source.reset()
			source.overlap = true
			_, parallelErr := resolveMixedRoots(t, t.Context(), app, requirements, nil, false)
			if parallelErr == nil || parallelErr.Error() != serialErr.Error() {
				t.Fatalf("prefetched error = %v, want the serial error %v", parallelErr, serialErr)
			}
			// The failure is served, not repeated: the failing read happens once.
			if got := source.readsOf(failing); got != serialFailing || got != 1 {
				t.Fatalf("failing read happened %d times, serially %d times", got, serialFailing)
			}
			// No new root starts after the failure. Roots already started
			// beside it may finish the read they are in, so the extra reads
			// over serial are bounded by two per other in-flight root.
			if got := source.startedAfterFailure; len(got) != 0 {
				t.Fatalf("roots started after the failure: %v", got)
			}
			if extra := len(source.sortedReads()) - len(serialReads); extra > 2*(prefetchWindow-1) {
				t.Fatalf("%d reads beyond serial, want at most %d", extra, 2*(prefetchWindow-1))
			}
			for _, read := range serialReads {
				if !slices.Contains(source.sortedReads(), read) {
					t.Fatalf("prefetched resolution skipped serial read %q", read)
				}
			}
			assertEmptyDir(t, source.tempRoot)
		})
	}
}

// A root canceled by an earlier failure does not move on from its ref read to
// its tree read, which keeps the reads beyond serial minimal.
func TestGitRootPrefetchCanceledRootStopsAfterItsRef(t *testing.T) {
	app, requirements, source := mixedRootsJourney(t)
	source.fail, source.holdRefs = "ref acme/skills//skills/alpha", true // first root in solver order
	if _, err := resolveMixedRoots(t, t.Context(), app, requirements, nil, false); err == nil {
		t.Fatal("expected the failing source to fail resolution")
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	if len(source.canceledRefs) == 0 {
		t.Fatal("no root was canceled while reading its ref; the test proves nothing")
	}
	if len(source.readsAfterCancel) != 0 {
		t.Fatalf("canceled roots read on after their ref: %v", source.readsAfterCancel)
	}
	assertEmptyDir(t, source.tempRoot)
}

func TestGitRootPrefetchCancellationLeavesNoAcquisitions(t *testing.T) {
	app, requirements, source := mixedRootsJourney(t)
	ctx, cancel := context.WithCancel(t.Context())
	source.onRead = func(n int) {
		if n == 3 {
			cancel()
		}
	}
	if _, err := resolveMixedRoots(t, ctx, app, requirements, nil, false); err == nil {
		t.Fatal("expected cancellation to abort resolution")
	}
	assertEmptyDir(t, source.tempRoot)
}
