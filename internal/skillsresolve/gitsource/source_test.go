package gitsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/internal/skillsresolve"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func put(t *testing.T, root, path, content string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func document(name string) string {
	return "---\r\nname: " + name + "\r\ndescription: Проверка\r\n---\r\n# Original instructions\r\n"
}

func commit(t *testing.T, root string) string {
	t.Helper()
	git(t, root, "add", ".")
	git(t, root, "commit", "--quiet", "-m", "fixture")
	return git(t, root, "rev-parse", "HEAD")
}

func fixture(t *testing.T) (string, Source, string) {
	t.Helper()
	root, scratch := t.TempDir(), t.TempDir()
	git(t, root, "init", "--quiet", "--initial-branch=main")
	git(t, root, "config", "uploadpack.allowFilter", "true")
	u := (&url.URL{Scheme: "file", Path: filepath.ToSlash(root)}).String()
	source := Source{Acquirer: skillsource.Acquirer{Acquirer: sourceacquisition.Acquirer{
		TempRoot: scratch, URLForRepo: func(string) string { return u },
	}, Policy: Policy}}
	return root, source, scratch
}

func pointer(s string) *string { return &s }

func assertClean(t *testing.T, scratch string) {
	t.Helper()
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 {
		t.Fatalf("scratch was not cleaned: %v %v", entries, err)
	}
}

func TestRealGitResolutionAndPinnedSelection(t *testing.T) {
	root, source, scratch := fixture(t)
	put(t, root, "actual/review/SKILL.md", document("review"))
	put(t, root, "actual/review/assets/unused.bin", strings.Repeat("asset", 100000))
	first := commit(t, root)
	service := skillsresolve.New(source)
	result, err := service.Resolve(t.Context(), skillsresolve.Request{Repository: "Example/Fixture", SkillNameHint: pointer("review")})
	hash := sha256.Sum256([]byte(document("review")))
	if err != nil || result.Status != "resolved" || result.Repository != "example/fixture" || result.Commit != first || result.Skill.Path != "actual/review" || result.Skill.RawSHA256 != hex.EncodeToString(hash[:]) || !result.Skill.HasResources {
		t.Fatalf("unexpected real resolution %+v %v", result, err)
	}
	assertClean(t, scratch)
	put(t, root, "actual/review/SKILL.md", document("review")+"Changed after preview.\n")
	commit(t, root)
	selected, err := service.Resolve(t.Context(), skillsresolve.Request{Repository: "example/fixture", Commit: &first, Path: pointer("actual/review")})
	if err != nil || selected.Status != "resolved" || selected.Skill.RawContent != document("review") || selected.Commit != first {
		t.Fatal("selection followed moving branch", selected, err)
	}
	assertClean(t, scratch)
}

func TestRealGitDiscoveryAndUnsafeKinds(t *testing.T) {
	root, source, scratch := fixture(t)
	put(t, root, "a/review/SKILL.md", document("review"))
	put(t, root, "b/review/SKILL.md", document("review"))
	sha := commit(t, root)
	result, err := skillsresolve.New(source).Resolve(t.Context(), skillsresolve.Request{Repository: "example/fixture", SkillNameHint: pointer("review")})
	if err != nil || result.Status != "selection_required" || len(result.Candidates) != 2 {
		t.Fatal("ambiguous candidates were guessed", result, err)
	}
	if err = os.Symlink("../SKILL.md", filepath.Join(root, "a/review/link")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "update-index", "--add", "--cacheinfo", "160000,"+sha+",b/review/vendor")
	git(t, root, "commit", "--quiet", "-m", "unsafe resource fixture")
	sha = git(t, root, "rev-parse", "HEAD")
	for _, path := range []string{"a/review", "b/review"} {
		result, err = skillsresolve.New(source).Resolve(t.Context(), skillsresolve.Request{Repository: "example/fixture", Commit: &sha, Path: &path})
		if err != nil || result.Status != "incomplete" || result.Skill != nil {
			t.Fatal("unsafe selected-tree kind was accepted", path, result, err)
		}
		assertClean(t, scratch)
	}
	source.Acquirer.Policy.MaxCandidates = 1
	result, err = skillsresolve.New(source).Resolve(t.Context(), skillsresolve.Request{Repository: "example/fixture", SkillNameHint: pointer("review")})
	if err != nil || result.Status != "incomplete" || result.Skill != nil {
		t.Fatal("partial discovery auto-selected", result, err)
	}
	assertClean(t, scratch)
}

func TestRealGitRootOversizeInvalidAndCancellation(t *testing.T) {
	root, source, scratch := fixture(t)
	put(t, root, "SKILL.md", document("fixture"))
	sha := commit(t, root)
	r, err := skillsresolve.New(source).Resolve(t.Context(), skillsresolve.Request{Repository: "example/fixture", Commit: &sha, Path: pointer("")})
	if err != nil || r.Status != "resolved" || r.Skill.Path != "" {
		t.Fatal("explicit root failed", r, err)
	}
	put(t, root, "SKILL.md", document("fixture")+strings.Repeat("x", 64<<10))
	commit(t, root)
	r, err = skillsresolve.New(source).Resolve(t.Context(), skillsresolve.Request{Repository: "example/fixture"})
	if err != nil || r.Status != "incomplete" {
		t.Fatal("oversize document reported complete", r, err)
	}
	for _, repository := range []string{"https://example.test/x", "owner/repo.git", "owner/../repo", "-c/x"} {
		_, err = skillsresolve.New(source).Resolve(t.Context(), skillsresolve.Request{Repository: repository})
		if !errors.Is(err, skillsresolve.ErrInvalidRequest) {
			t.Fatal("invalid repository was accepted", repository, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = skillsresolve.New(source).Resolve(ctx, skillsresolve.Request{Repository: "example/fixture", Commit: &sha, Path: pointer("")}); err == nil {
		t.Fatal("canceled acquisition succeeded")
	}
	assertClean(t, scratch)
}

func TestHostedGitRejectsAlternateTransport(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := sourceacquisition.Command{Dir: t.TempDir(), Args: []string{"ls-remote", "file:///nonexistent"}, MaxOutputBytes: 4096}
	if _, err := (&hostedRunner{}).Run(ctx, command); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatal("file transport was not rejected", err)
	}
	command.NativeCredentials = true
	if _, err := (&hostedRunner{}).Run(ctx, command); !errors.Is(err, skillsresolve.ErrInvalidRequest) {
		t.Fatal("hosted native credentials accepted", err)
	}
}
