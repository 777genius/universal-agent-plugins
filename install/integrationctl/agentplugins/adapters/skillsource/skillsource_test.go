package skillsource

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=iliya", "GIT_AUTHOR_EMAIL=iliyazelenkog@gmail.com", "GIT_COMMITTER_NAME=iliya", "GIT_COMMITTER_EMAIL=iliyazelenkog@gmail.com")
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %s %v", args, out, e)
	}
	return strings.TrimSpace(string(out))
}
func put(t *testing.T, root, p, body string, mode os.FileMode) {
	t.Helper()
	p = filepath.Join(root, filepath.FromSlash(p))
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(body), mode); e != nil {
		t.Fatal(e)
	}
}
func document(name string) string {
	return "---\nname: " + name + "\ndescription: Exact fixture\nfuture: {opaque: true}\nmetadata: {version: 'not-semver'}\n---\n# Original\r\n"
}
func fixture(t *testing.T) (string, Acquirer) {
	t.Helper()
	if _, e := exec.LookPath("git"); e != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	git(t, root, "init", "--quiet", "--initial-branch=main")
	return root, Acquirer{Acquirer: sourceacquisition.Acquirer{TempRoot: t.TempDir(), URLForRepo: func(string) string { return root }}}
}
func commitFixture(t *testing.T, root string) string {
	git(t, root, "add", ".")
	git(t, root, "commit", "--quiet", "-m", "test: fixture")
	return git(t, root, "rev-parse", "HEAD")
}

// This boundary catches omitted nested/dot candidates, traversal through links,
// false root completeness, and applying root budgets before narrowing scope.
func TestDiscoveryScopeCompletenessAndCancellation(t *testing.T) {
	root, a := fixture(t)
	for i := range 1000 {
		name := fmt.Sprintf("skill-%04d", i)
		put(t, root, "catalog/"+name+"/SKILL.md", document(name), 0600)
	}
	put(t, root, ".agents/outer/SKILL.md", document("outer"), 0600)
	put(t, root, ".agents/outer/inner/SKILL.md", document("inner"), 0600)
	put(t, root, ".agents/outer/scripts/run", "#!/bin/sh\nexit 0\n", 0700)
	put(t, root, "bad/SKILL.md", "---\nname: different\ndescription: x\n---\n", 0600)
	if e := os.Mkdir(filepath.Join(root, "linked"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("../.agents/outer/SKILL.md", filepath.Join(root, "linked", "SKILL.md")); e != nil {
		t.Fatal(e)
	}
	sha := commitFixture(t, root)
	s, e := a.Open(context.Background(), Source{Repository: "example/fixture", Commit: sha})
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	catalog, e := s.Scan(context.Background(), "")
	if e != nil {
		t.Fatal(e)
	}
	if !catalog.Complete || len(catalog.Candidates) != 1004 {
		t.Fatalf("catalog complete=%v count=%d reasons=%v", catalog.Complete, len(catalog.Candidates), catalog.IncompleteReasons)
	}
	if catalog.Discovered != 1004 || catalog.Validated != 1004 || catalog.Bytes < 1000 {
		t.Fatal("discovery facts missing", catalog.Discovered, catalog.Validated, catalog.Bytes)
	}
	available := 0
	for _, c := range catalog.Candidates {
		if c.Status == "available" {
			available++
		}
	}
	if available != 1002 {
		t.Fatalf("available=%d", available)
	}
	exact, e := s.ExactCatalog(context.Background(), ".agents/outer")
	if e != nil || exact.Complete || len(exact.Candidates) != 1 || exact.Candidates[0].Status != "available" {
		t.Fatalf("exact selection catalog=%+v %v", exact, e)
	}
	s.policy.MaxEntries = 4
	partial, e := s.Scan(context.Background(), "")
	if e != nil || partial.Complete {
		t.Fatalf("partial=%+v %v", partial, e)
	}
	scoped, e := s.Scan(context.Background(), ".agents")
	if e != nil || !scoped.Complete || len(scoped.Candidates) != 2 {
		t.Fatalf("scoped=%+v %v", scoped, e)
	}
	repeat, e := s.Scan(context.Background(), ".agents")
	if e != nil || repeat.Digest != scoped.Digest {
		t.Fatal("catalog is not deterministic", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = s.Scan(ctx, ""); e == nil {
		t.Fatal("canceled discovery succeeded")
	}
}

func TestNormalizationPreservesRawTreeAndStableManifestAcrossMove(t *testing.T) {
	root, a := fixture(t)
	put(t, root, "old/review/SKILL.md", document("review"), 0600)
	put(t, root, "old/review/scripts/run", "#!/bin/sh\nprintf test\n", 0700)
	put(t, root, "old/review/references/data", "original bytes\x00\xff", 0600)
	if e := os.Symlink("data", filepath.Join(root, "old/review/references/link")); e != nil {
		t.Fatal(e)
	}
	sha := commitFixture(t, root)
	raw, e := a.AcquireGitHub(context.Background(), "example/fixture", sha, "old/review")
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = packagedigest.Remove(raw) }()
	artifact, e := Normalize(context.Background(), raw, "review", "github-skill:example/fixture:review:old/review")
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = packagedigest.Remove(artifact) }()
	registry, e := specregistry.New()
	if e != nil {
		t.Fatal(e)
	}
	envelope, e := (loader.Loader{Registry: registry}).LoadSnapshot(context.Background(), artifact)
	if e != nil || len(envelope.Skills) != 1 {
		t.Fatalf("ordinary loader: %+v %v", envelope, e)
	}
	content, e := os.ReadFile(filepath.Join(artifact.Root, "skills/review/SKILL.md"))
	if e != nil || string(content) != document("review") {
		t.Fatal("raw document rewritten", e)
	}
	if len(artifact.ExecutableFiles) != 1 || artifact.ExecutableFiles[0] != "skills/review/scripts/run" {
		t.Fatal(artifact.ExecutableFiles)
	}
	if target, e := os.Readlink(filepath.Join(artifact.Root, "skills/review/references/link")); e != nil || target != "data" {
		t.Fatal("internal resource link changed", target, e)
	}
	git(t, root, "mv", "old", "new")
	sha2 := commitFixture(t, root)
	resolved, e := a.ResolveGitHubSkill(context.Background(), "example/fixture", sha2, "review", "old/review", "old/review")
	if e != nil || resolved != "new/review" {
		t.Fatal(resolved, e)
	}
	moved, e := a.AcquireGitHub(context.Background(), "example/fixture", sha2, resolved)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = packagedigest.Remove(moved) }()
	normalized, e := Normalize(context.Background(), moved, "review", "github-skill:example/fixture:review:old/review")
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = packagedigest.Remove(normalized) }()
	if raw.TreeDigest != moved.TreeDigest || artifact.TreeDigest != normalized.TreeDigest {
		t.Fatal("movement changed immutable artifact identity")
	}
	put(t, root, "new/review/references/data", "changed resource", 0600)
	changedSHA := commitFixture(t, root)
	changed, e := a.AcquireGitHub(context.Background(), "example/fixture", changedSHA, "new/review")
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = packagedigest.Remove(changed) }()
	changedArtifact, e := Normalize(context.Background(), changed, "review", "github-skill:example/fixture:review:old/review")
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = packagedigest.Remove(changedArtifact) }()
	if changed.TreeDigest == moved.TreeDigest || changedArtifact.TreeDigest == artifact.TreeDigest {
		t.Fatal("resource bytes missing from digests")
	}
	put(t, root, "duplicate/review/SKILL.md", document("review"), 0600)
	sha3 := commitFixture(t, root)
	if _, e = a.ResolveGitHubSkill(context.Background(), "example/fixture", sha3, "review", "gone", "gone"); e == nil {
		t.Fatal("ambiguous relocation succeeded")
	}
	a.Policy.MaxEntries = 1
	if _, e = a.ResolveGitHubSkill(context.Background(), "example/fixture", sha2, "review", "gone", "gone"); e == nil {
		t.Fatal("partial catalog established relocation uniqueness")
	}
}

func TestURLResolutionUsesActualSlashRefsAndRejectsAmbiguity(t *testing.T) {
	root, a := fixture(t)
	put(t, root, "skills/review/SKILL.md", document("review"), 0600)
	sha := commitFixture(t, root)
	git(t, root, "branch", "feature/topic")
	git(t, root, "tag", "feature")
	git(t, root, "branch", "default")
	s, e := a.Resolve(context.Background(), "https://github.com/Example/Fixture/tree/feature/topic/skills/review", "")
	if e == nil {
		t.Fatal("ambiguous slash boundary accepted", s)
	}
	s, e = a.Resolve(context.Background(), "https://github.com/Example/Fixture/blob/feature/topic/skills/review/SKILL.md", "refs/heads/feature/topic")
	if e != nil || s.Commit != sha || s.RequestedPath != "skills/review" || s.Repository != "example/fixture" || !s.ExactPath {
		t.Fatal(s, e)
	}
	s, e = a.Resolve(context.Background(), "https://raw.githubusercontent.com/example/fixture/"+sha+"/skills/review/SKILL.md", "")
	if e != nil || s.Commit != sha || s.RequestedPath != "skills/review" {
		t.Fatal(s, e)
	}
	symbolic, e := a.Resolve(context.Background(), "example/fixture@HEAD", "")
	if e != nil || symbolic.RequestedSelector != "default" || symbolic.Commit != sha {
		t.Fatal("symbolic HEAD became a named ref", symbolic, e)
	}
	git(t, root, "branch", "refs/tags/topic")
	nested, e := a.Resolve(context.Background(), "https://github.com/example/fixture/tree/refs/tags/topic/skills/review", "refs/heads/refs/tags/topic")
	if e != nil || nested.RequestedSelector != "refs/heads/refs/tags/topic" || nested.RequestedPath != "skills/review" {
		t.Fatal("ref namespace inside branch name was stripped twice", nested, e)
	}
	named, e := a.Resolve(context.Background(), "example/fixture", "default")
	if e != nil || named.RequestedSelector != "refs/heads/default" {
		t.Fatal("explicit default ref became default selector", named, e)
	}
	for _, input := range []string{"example/fixture@", "https://github.com/example/fixture/tree/main/../review", "example/fixture//../review", "https://github.com@example.org/example/fixture", "https://github.com/example/fixture/blob/main/other.md"} {
		if _, e = a.Resolve(context.Background(), input, ""); e == nil {
			t.Fatalf("unsafe input %q accepted", input)
		}
	}
}

func TestGitlinksNeverBecomeSkillTreeScopes(t *testing.T) {
	root, a := fixture(t)
	put(t, root, "SKILL.md", document("fixture"), 0600)
	sha := commitFixture(t, root)
	git(t, root, "update-index", "--add", "--cacheinfo", "160000,"+sha+",module")
	git(t, root, "commit", "--quiet", "-m", "test: gitlink fixture")
	sha = git(t, root, "rev-parse", "HEAD")
	s, e := a.Open(context.Background(), Source{Repository: "example/fixture", Commit: sha})
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	if _, e = s.CandidateAt(context.Background(), "module"); e == nil {
		t.Fatal("gitlink traversed as a Skill directory")
	}
	c, e := s.Scan(context.Background(), "module")
	if e != nil || c.Complete || len(c.Candidates) != 0 {
		t.Fatalf("gitlink scope=%+v %v", c, e)
	}
}

// The rendered preview excludes YAML, while raw view and normalization retain
// every source byte. Resource inventory continues to include nested resources.
func TestPreviewSeparatesMarkdownBodyFromExactRawDocument(t *testing.T) {
	root, a := fixture(t)
	put(t, root, ".agents/outer/SKILL.md", document("outer"), 0600)
	put(t, root, ".agents/outer/inner/SKILL.md", document("inner"), 0600)
	put(t, root, ".agents/outer/scripts/run", "#!/bin/sh\nexit 0\n", 0700)
	if err := os.Symlink("run", filepath.Join(root, ".agents/outer/scripts/alias")); err != nil {
		t.Fatal(err)
	}
	sha := commitFixture(t, root)
	s, e := a.Open(context.Background(), Source{Repository: "example/fixture", Commit: sha})
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	preview, e := s.Preview(context.Background(), ".agents/outer")
	if e != nil || preview.Markdown != "# Original\n" || preview.RawContent != document("outer") || string(preview.Skill.Raw) != document("outer") || len(preview.Resources) != 3 {
		t.Fatalf("preview=%+v %v", preview, e)
	}
	link := preview.Resources[1]
	if link.Path != "scripts/alias" || link.Size != nil || link.Executable {
		t.Fatal("symlink resource was followed or omitted", link)
	}
}

// Observe the real object reader: a rejected document must not be downloaded
// first and discarded later, and aggregate reservation must bound both readers.
type blobReadProbe struct{ reads, checkouts atomic.Int32 }

func (p *blobReadProbe) Run(ctx context.Context, command sourceacquisition.Command) ([]byte, error) {
	for i, arg := range command.Args {
		if arg == "checkout" {
			p.checkouts.Add(1)
		}
		if arg == "cat-file" && i+1 < len(command.Args) && command.Args[i+1] == "blob" {
			p.reads.Add(1)
		}
	}
	return (sourceacquisition.OSRunner{}).Run(ctx, command)
}
func TestDiscoveryBudgetsPreventUnreservedObjectReads(t *testing.T) {
	root, a := fixture(t)
	put(t, root, "category/a/SKILL.md", document("a"), 0600)
	put(t, root, "category/b/SKILL.md", document("b"), 0600)
	sha := commitFixture(t, root)
	for _, tc := range []struct {
		name, reason      string
		policy            ScanPolicy
		reads, candidates int
	}{
		{"document", "document_bytes_limit", ScanPolicy{MaxDocumentBytes: len(document("a")) - 1}, 0, 2},
		{"aggregate", "aggregate_bytes_limit", ScanPolicy{MaxAggregateBytes: len(document("a"))}, 1, 2},
		{"candidate", "candidate_limit", ScanPolicy{MaxCandidates: 1}, 1, 1},
		{"depth", "tree_depth_limit", ScanPolicy{MaxDepth: 2}, 0, 0},
		{"metadata", "tree_metadata_unavailable", ScanPolicy{MaxMetadataBytes: 32}, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probe := &blobReadProbe{}
			bounded := a
			bounded.Runner = probe
			bounded.Policy = tc.policy
			s, err := bounded.Open(context.Background(), Source{Repository: "example/fixture", Commit: sha})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Close() }()
			catalog, err := s.Scan(context.Background(), "")
			if err != nil {
				t.Fatal(err)
			}
			if catalog.Complete || len(catalog.Candidates) != tc.candidates || int(probe.reads.Load()) != tc.reads || !slices.Contains(catalog.IncompleteReasons, tc.reason) {
				t.Fatalf("catalog complete=%v count=%d reads=%d reasons=%v", catalog.Complete, len(catalog.Candidates), probe.reads.Load(), catalog.IncompleteReasons)
			}
			if tc.policy.MaxAggregateBytes > 0 && catalog.Bytes > int64(tc.policy.MaxAggregateBytes) {
				t.Fatal("aggregate document budget exceeded", catalog.Bytes)
			}
		})
	}
}

func TestRootRawSkillRejectsGitlinksBeforeSnapshot(t *testing.T) {
	root, a := fixture(t)
	put(t, root, "SKILL.md", document("fixture"), 0600)
	cleanSHA := commitFixture(t, root)
	git(t, root, "update-index", "--add", "--cacheinfo", "160000,"+cleanSHA+",module")
	git(t, root, "commit", "--quiet", "-m", "test: root raw Skill submodule")
	sha := git(t, root, "rev-parse", "HEAD")
	probe := &blobReadProbe{}
	a.Runner = probe
	for _, verified := range []bool{false, true} {
		var snapshot domain.PackageSnapshot
		var err error
		if verified {
			snapshot, err = a.AcquireGitHubSkillVerified(context.Background(), "example/fixture", sha, "", "sha256:"+strings.Repeat("a", 64))
		} else {
			snapshot, err = a.AcquireGitHubSkill(context.Background(), "example/fixture", sha, "")
		}
		if err == nil || !strings.Contains(err.Error(), "submodule") || snapshot.Root != "" || probe.checkouts.Load() != 0 {
			t.Fatalf("raw root gitlink reached checkout/snapshot: verified=%v snapshot=%+v checkouts=%d err=%v", verified, snapshot, probe.checkouts.Load(), err)
		}
	}
	entries, err := os.ReadDir(a.TempRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed acquisition left temporary effects: entries=%v err=%v", entries, err)
	}
	clean, err := a.AcquireGitHubSkill(context.Background(), "example/fixture", cleanSHA, "")
	if err != nil {
		t.Fatal("valid root Skill was rejected", err)
	}
	defer func() { _ = packagedigest.Remove(clean) }()
	// Preserve the existing ordinary-package exception for unrelated root gitlinks.
	legacy, err := a.AcquireGitHub(context.Background(), "example/fixture", sha, "")
	if err != nil {
		t.Fatal("ordinary root package behavior changed", err)
	}
	if err := packagedigest.Remove(legacy); err != nil {
		t.Fatal(err)
	}
}
