package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
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

// rootFetchSource counts every remote read of a Git root and can fail one
// repository or hold the first reads until another one runs concurrently.
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
}

func (source *rootFetchSource) read(ctx context.Context, kind, repository string) error {
	source.mu.Lock()
	source.reads = append(source.reads, kind+" "+repository)
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
	if repository == source.fail {
		return errors.New("remote read failed for " + repository)
	}
	return ctx.Err()
}

func (source *rootFetchSource) ResolveGit(ctx context.Context, spec sourcespec.Git, selector string, exact bool) (sourceacquisition.RefResolution, error) {
	if err := source.read(ctx, "ref", spec.Repository); err != nil {
		return sourceacquisition.RefResolution{}, err
	}
	return source.Acquirer.ResolveGit(ctx, spec, selector, exact)
}

func (source *rootFetchSource) AcquireGit(ctx context.Context, spec sourcespec.Git, commit, digest string) (domain.PackageSnapshot, error) {
	if err := source.read(ctx, "tree", spec.Repository); err != nil {
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
	for _, failing := range []string{"acme/pb", "acme/skills//skills/beta", "other/pc"} {
		t.Run(failing, func(t *testing.T) {
			app, requirements, source := mixedRootsJourney(t)
			source.fail = failing
			_, serialErr := resolveMixedRoots(t, t.Context(), app, requirements, nil, true)
			if serialErr == nil {
				t.Fatal("expected the failing source to fail serial resolution")
			}
			_, parallelErr := resolveMixedRoots(t, t.Context(), app, requirements, nil, false)
			if parallelErr == nil || parallelErr.Error() != serialErr.Error() {
				t.Fatalf("prefetched error = %v, want the serial error %v", parallelErr, serialErr)
			}
			assertEmptyDir(t, source.tempRoot)
		})
	}
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
