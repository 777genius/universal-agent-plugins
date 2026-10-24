package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type dependencyStagingSource struct {
	dependencyFixtureSource
	snapshots []domain.PackageSnapshot
}

func (source *dependencyStagingSource) AcquireGitHubVerified(ctx context.Context, repo, commit, path, digest string) (domain.PackageSnapshot, error) {
	snapshot, err := source.dependencyFixtureSource.AcquireGitHubVerified(ctx, repo, commit, path, digest)
	if err == nil {
		source.snapshots = append(source.snapshots, snapshot)
	}
	return snapshot, err
}

type dependencyStagingScanner struct {
	calls int
	fail  bool
}

func (scanner *dependencyStagingScanner) Evaluate(context.Context, domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	scanner.calls++
	if scanner.fail {
		return domain.SecurityAssessment{}, errors.New("fresh scanner refusal")
	}
	return domain.SecurityAssessment{}, nil
}

func dependencyStagingFixture(t *testing.T) (App, environment.LockedPackage, *dependencyStagingSource) {
	t.Helper()
	f := newCLIFixture(t, nil)
	root := writeCLIPlugin(t)
	loaded, err := f.app.acquireLocal(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = loaded.cleanup() }()
	pin := environment.LockedPackage{Alias: "demo", DigestAlgorithm: domain.TreeDigestAlgorithm,
		TreeDigest: loaded.envelope.TreeDigest, ManifestDigest: loaded.envelope.ManifestDigest,
		ExecutableFiles: append([]string{}, loaded.envelope.ExecutableFiles...),
		Source:          environment.LockedSource{Git: &environment.LockedGitSource{Repository: "acme/demo", Commit: strings.Repeat("1", 40)}}}
	source := &dependencyStagingSource{dependencyFixtureSource: dependencyFixtureSource{root: root}}
	f.app.SourceAcquirer = source
	f.app.EnvironmentStoreRoot = filepath.Join(t.TempDir(), "store")
	return f.app, pin, source
}

// Reusing bytes must not reuse a prior scanner result, persist unconfirmed
// content, retain consumer snapshots, or retain acquisitions across commands.
func TestDependencyStagingRechecksAndCleansSnapshots(t *testing.T) {
	base, pin, source := dependencyStagingFixture(t)
	scanner := &dependencyStagingScanner{}
	base.SecurityEvaluator = scanner
	app, cleanup, err := dependencyPreviewApp(base, &options{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for range 2 {
		loaded, err := materializeLockedPackage(context.Background(), app, pin, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		root := loaded.envelope.SnapshotRoot
		if err := loaded.cleanup(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("consumer snapshot survived its cleanup")
		}
	}
	if len(source.snapshots) != 1 || scanner.calls != 2 {
		t.Fatalf("reuse did not separate acquisition and scanning: acquisitions=%d scans=%d", len(source.snapshots), scanner.calls)
	}
	retained := source.snapshots[0].Root
	if _, err := os.Stat(retained); err != nil {
		t.Fatal("consumer cleanup removed command-owned raw bytes", err)
	}
	scanner.fail = true
	if _, err := materializeLockedPackage(context.Background(), app, pin, false, nil); err == nil || !strings.Contains(err.Error(), "fresh scanner refusal") {
		t.Fatalf("cached bytes bypassed current scanner refusal: %v", err)
	}
	if len(source.snapshots) != 1 || scanner.calls != 3 {
		t.Fatal("scanner refusal fetched again or reused assessment")
	}
	if _, err := os.Stat(base.EnvironmentStoreRoot); !os.IsNotExist(err) {
		t.Fatal("unconfirmed staging published the persistent store")
	}
	cleanup()
	if _, err := os.Stat(retained); !os.IsNotExist(err) {
		t.Fatal("command cleanup retained acquired bytes")
	}
	next, nextCleanup, err := dependencyPreviewApp(base, &options{})
	if err != nil {
		t.Fatal(err)
	}
	defer nextCleanup()
	snapshot, err := next.acquireLockedSnapshot(context.Background(), pin)
	if err != nil {
		t.Fatal(err)
	}
	if len(source.snapshots) != 2 {
		t.Fatal("next command reused the preceding command's staging")
	}
	_ = packagedigest.Remove(snapshot)
}

func TestDependencyStagingRejectsChangedBytesWithoutRefetch(t *testing.T) {
	base, pin, source := dependencyStagingFixture(t)
	app, cleanup, err := dependencyPreviewApp(base, &options{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	snapshot, err := app.acquireLockedSnapshot(context.Background(), pin)
	if err != nil {
		t.Fatal(err)
	}
	_ = packagedigest.Remove(snapshot)
	manifest := filepath.Join(source.snapshots[0].Root, "plugin.json")
	if err := os.Chmod(manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`{"name":"changed"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := app.acquireLockedSnapshot(context.Background(), pin); err == nil || !strings.Contains(err.Error(), "staged dependency no longer matches") {
		t.Fatalf("reused changed staging: %v", err)
	}
	if len(source.snapshots) != 1 {
		t.Fatal("changed staging silently refetched")
	}
}

func TestDependencyStagingKeepsExactSourceCoordinates(t *testing.T) {
	base, pin, source := dependencyStagingFixture(t)
	app, cleanup, err := dependencyPreviewApp(base, &options{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	// Different repositories, revisions and subpaths may contain identical
	// package bytes. Their immutable source acquisitions are still distinct.
	for _, coordinates := range []environment.LockedGitSource{
		{Repository: "acme/demo", Commit: strings.Repeat("1", 40)},
		{Repository: "other/demo", Commit: strings.Repeat("1", 40)},
		{Repository: "acme/demo", Commit: strings.Repeat("2", 40)},
		{Repository: "acme/demo", Commit: strings.Repeat("1", 40), Path: "nested"},
		{Repository: "acme/demo", Commit: strings.Repeat("1", 40)},
	} {
		pin.Source.Git = &coordinates
		snapshot, err := app.acquireLockedSnapshot(context.Background(), pin)
		if err != nil {
			t.Fatal(err)
		}
		_ = packagedigest.Remove(snapshot)
	}
	if len(source.snapshots) != 4 {
		t.Fatalf("expected one acquisition per exact source tuple, got %d", len(source.snapshots))
	}
}

func TestDependencyCorruptStoreNeverDownloadsReplacement(t *testing.T) {
	app, pin, source := dependencyStagingFixture(t)
	snapshot, err := source.AcquireGitHubVerified(context.Background(), "", "", "", pin.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	pkg, err := app.dependencyStore().Put(context.Background(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg.Root, "plugin.json"), []byte(`{"name":"changed"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, publish := range []bool{false, true} {
		if _, err := materializeLockedPackage(context.Background(), app, pin, publish, nil); !errors.Is(err, environmentstore.ErrMismatch) {
			t.Fatalf("corrupt store did not fail closed, publish=%v: %v", publish, err)
		}
	}
	if err := ensureDependencyStore(context.Background(), app, environment.Lock{Packages: []environment.LockedPackage{pin}}); !errors.Is(err, environmentstore.ErrMismatch) {
		t.Fatalf("store publication replaced corrupt content: %v", err)
	}
	if len(source.snapshots) != 1 {
		t.Fatal("corrupt store triggered a replacement source fetch")
	}
}

// A retained normalized artifact must not bypass a different raw-source contract.
func TestDependencyStagingSeparatesSkillProvenance(t *testing.T) {
	j, _, sha := skillJourney(t)
	ctx := context.Background()
	plan, err := j.cli.app.PrepareSkills(ctx, SkillsBatchRequest{Selections: []SkillsSelection{{Repository: "acme/tools", Commit: sha, Path: "skills/pdf", Name: "pdf"}}, TargetIDs: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	pin := plan.request.lock.Packages[0]
	for _, change := range []struct {
		name string
		edit func(*environment.LockedGitSource)
	}{
		{"raw digest", func(source *environment.LockedGitSource) { source.RawTreeDigest = strings.Repeat("0", 64) }},
		{"normalizer", func(source *environment.LockedGitSource) { source.Normalizer = "unsupported" }},
		{"name", func(source *environment.LockedGitSource) { source.Skill = "other" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			changed := pin
			source := *pin.Source.Git
			changed.Source.Git = &source
			change.edit(&source)
			if snapshot, acquireErr := plan.preview.acquireLockedSnapshot(ctx, changed); acquireErr == nil {
				_ = packagedigest.Remove(snapshot)
				t.Fatal("cached artifact bypassed changed Skill verification")
			}
		})
	}
	changed := pin
	source := *pin.Source.Git
	source.Path = "previous/pdf"
	changed.Source.Git = &source
	snapshot, err := plan.preview.acquireLockedSnapshot(ctx, changed)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	if snapshot.Source.Skill.RequestedPath != source.Path {
		t.Fatal("cache returned a previous requested source anchor")
	}
}

func TestSkillRootRequiresRawAcquisitionCapability(t *testing.T) {
	app, pin, source := dependencyStagingFixture(t)
	ctx := context.Background()
	requirement := environment.Requirement{Alias: "pdf", Git: &environment.GitSource{Kind: environment.SkillKind, Skill: "pdf", Repository: "acme/pdf", Selector: environment.Selector{Kind: environment.SelectorCommit, Value: pin.Source.Git.Commit}}}
	if _, err := (dependencyResolver{app: app}).resolveSkill(ctx, requirement, []string{"codex"}, nil, pin.Source.Git.Commit, "", ""); err == nil || !strings.Contains(err.Error(), "raw Skill acquisition unavailable") {
		t.Fatalf("root resolution used plugin acquisition: %v", err)
	}
	pin.Source.Git.Kind = environment.SkillKind
	pin.Source.Git.Skill = "pdf"
	pin.Source.Git.Normalizer = environment.SkillNormalizerV1
	if _, err := app.acquireLockedSkill(ctx, pin); err == nil || !strings.Contains(err.Error(), "raw Skill acquisition unavailable") {
		t.Fatalf("root replay used plugin acquisition: %v", err)
	}
	if len(source.snapshots) != 0 {
		t.Fatal("raw Skill fell back to plugin source acquisition")
	}
}
