package source

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type stubRunner struct {
	run func(context.Context, ports.Command) (ports.CommandResult, error)
}

func (s stubRunner) Run(ctx context.Context, cmd ports.Command) (ports.CommandResult, error) {
	return s.run(ctx, cmd)
}

func TestResolveGitURLUsesProcessRunnerAndHashesMaterializedTree(t *testing.T) {
	t.Parallel()
	resolver := Resolver{
		Runner: stubRunner{run: func(_ context.Context, cmd ports.Command) (ports.CommandResult, error) {
			if len(cmd.Argv) >= 5 && cmd.Argv[0] == "git" && cmd.Argv[1] == "clone" {
				dst := cmd.Argv[len(cmd.Argv)-1]
				if err := os.MkdirAll(filepath.Join(dst, "plugin"), 0o755); err != nil {
					t.Fatalf("mkdir clone dst: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dst, "plugin", "plugin.yaml"), []byte("api_version: v1\nname: demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n"), 0o644); err != nil {
					t.Fatalf("write clone plugin.yaml: %v", err)
				}
				return ports.CommandResult{ExitCode: 0}, nil
			}
			if len(cmd.Argv) >= 5 && cmd.Argv[0] == "git" && cmd.Argv[3] == "rev-parse" {
				return ports.CommandResult{ExitCode: 0, Stdout: []byte("abc123\n")}, nil
			}
			t.Fatalf("unexpected command: %+v", cmd.Argv)
			return ports.CommandResult{}, nil
		}},
	}

	resolved, err := resolver.Resolve(context.Background(), domain.IntegrationRef{Raw: "https://example.com/acme/demo.git"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Kind != "git_url" {
		t.Fatalf("kind = %s, want git_url", resolved.Kind)
	}
	if resolved.Cleanup == nil {
		t.Fatal("expected cleanup capability")
	}
	if !strings.HasSuffix(resolved.Resolved.Value, "@abc123") {
		t.Fatalf("resolved ref = %s", resolved.Resolved.Value)
	}
	if !strings.HasPrefix(resolved.SourceDigest, "sha256:") {
		t.Fatalf("source digest = %s, want sha256 digest", resolved.SourceDigest)
	}
	if _, err := os.Stat(filepath.Join(resolved.LocalPath, "plugin", "plugin.yaml")); err != nil {
		t.Fatalf("snapshot content: %v", err)
	}
	if err := resolved.Cleanup(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := os.Stat(resolved.LocalPath); !os.IsNotExist(err) {
		t.Fatalf("resolved source survived cleanup: %v", err)
	}
}

func TestResolveGitHubSubdirUsesSubtreeDigestAndCleanupRoot(t *testing.T) {
	t.Parallel()
	resolver := Resolver{
		Runner: stubRunner{run: func(_ context.Context, cmd ports.Command) (ports.CommandResult, error) {
			if len(cmd.Argv) >= 5 && cmd.Argv[0] == "git" && cmd.Argv[1] == "clone" {
				dst := cmd.Argv[len(cmd.Argv)-1]
				if err := os.MkdirAll(filepath.Join(dst, "plugins", "demo", "plugin"), 0o755); err != nil {
					t.Fatalf("mkdir clone dst: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dst, "README.md"), []byte("root readme\n"), 0o644); err != nil {
					t.Fatalf("write clone readme: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dst, "plugins", "demo", "plugin", "plugin.yaml"), []byte("api_version: v1\nname: demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - opencode\n"), 0o644); err != nil {
					t.Fatalf("write subtree plugin.yaml: %v", err)
				}
				return ports.CommandResult{ExitCode: 0}, nil
			}
			if len(cmd.Argv) >= 5 && cmd.Argv[0] == "git" && cmd.Argv[3] == "rev-parse" {
				return ports.CommandResult{ExitCode: 0, Stdout: []byte("def456\n")}, nil
			}
			t.Fatalf("unexpected command: %+v", cmd.Argv)
			return ports.CommandResult{}, nil
		}},
	}

	resolved, err := resolver.Resolve(context.Background(), domain.IntegrationRef{Raw: "github:acme/demo//plugins/demo"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Kind != "github_repo_path" {
		t.Fatalf("kind = %s, want github_repo_path", resolved.Kind)
	}
	if resolved.Repository != "acme/demo" || resolved.PackageSubpath != "plugins/demo" || resolved.ResolvedRevision != "def456" || resolved.CanonicalSource != "https://github.com/acme/demo//plugins/demo" {
		t.Fatalf("structured provenance = %+v", resolved)
	}
	if _, err := os.Stat(filepath.Join(resolved.LocalPath, "plugin", "plugin.yaml")); err != nil {
		t.Fatalf("snapshot subtree content: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resolved.LocalPath, "README.md")); !os.IsNotExist(err) {
		t.Fatalf("snapshot escaped requested subdir: %v", err)
	}
	if resolved.Cleanup == nil {
		t.Fatal("expected cleanup capability")
	}
	if !strings.HasPrefix(resolved.SourceDigest, "sha256:") {
		t.Fatalf("source digest = %s, want sha256 digest", resolved.SourceDigest)
	}
	if err := resolved.Cleanup(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := os.Stat(resolved.LocalPath); !os.IsNotExist(err) {
		t.Fatalf("resolved subdir survived cleanup: %v", err)
	}
}

func TestResolveFirstPartyAliasMapsToCanonicalGitHubSource(t *testing.T) {
	t.Parallel()
	resolver := Resolver{
		Runner: stubRunner{run: func(_ context.Context, cmd ports.Command) (ports.CommandResult, error) {
			if len(cmd.Argv) >= 6 && cmd.Argv[0] == "git" && cmd.Argv[1] == "clone" {
				if got, want := cmd.Argv[5], "https://github.com/777genius/universal-plugins-for-ai-agents.git"; got != want {
					t.Fatalf("clone url = %q, want %q", got, want)
				}
				dst := cmd.Argv[len(cmd.Argv)-1]
				if err := os.MkdirAll(filepath.Join(dst, "plugins", "notion", "plugin"), 0o755); err != nil {
					t.Fatalf("mkdir clone dst: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dst, "plugins", "notion", "plugin", "plugin.yaml"), []byte("api_version: v1\nname: notion\nversion: 0.1.0\ndescription: test\ntargets:\n  - claude\n"), 0o644); err != nil {
					t.Fatalf("write subtree plugin.yaml: %v", err)
				}
				return ports.CommandResult{ExitCode: 0}, nil
			}
			if len(cmd.Argv) >= 5 && cmd.Argv[0] == "git" && cmd.Argv[3] == "rev-parse" {
				return ports.CommandResult{ExitCode: 0, Stdout: []byte("alias123\n")}, nil
			}
			t.Fatalf("unexpected command: %+v", cmd.Argv)
			return ports.CommandResult{}, nil
		}},
	}

	resolved, err := resolver.Resolve(context.Background(), domain.IntegrationRef{Raw: "notion"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Kind != "github_repo_path" {
		t.Fatalf("kind = %s, want github_repo_path", resolved.Kind)
	}
	if _, err := os.Stat(filepath.Join(resolved.LocalPath, "plugin", "plugin.yaml")); err != nil {
		t.Fatalf("alias snapshot content: %v", err)
	}
	t.Cleanup(func() { _ = resolved.Cleanup() })
}

func TestResolveGitHubRefFetchesPinnedRevision(t *testing.T) {
	t.Parallel()
	var sawFetch bool
	resolver := Resolver{
		Runner: stubRunner{run: func(_ context.Context, cmd ports.Command) (ports.CommandResult, error) {
			switch {
			case len(cmd.Argv) >= 5 && cmd.Argv[0] == "git" && cmd.Argv[1] == "clone":
				dst := cmd.Argv[len(cmd.Argv)-1]
				if err := os.MkdirAll(filepath.Join(dst, "plugin"), 0o755); err != nil {
					t.Fatalf("mkdir clone dst: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dst, "plugin", "README.md"), []byte("demo\n"), 0o644); err != nil {
					t.Fatalf("write clone readme: %v", err)
				}
				return ports.CommandResult{ExitCode: 0}, nil
			case len(cmd.Argv) >= 7 && cmd.Argv[0] == "git" && cmd.Argv[3] == "fetch":
				sawFetch = true
				if got := cmd.Argv[len(cmd.Argv)-1]; got != "v1.2.3" {
					t.Fatalf("fetch ref = %q, want v1.2.3", got)
				}
				return ports.CommandResult{ExitCode: 0}, nil
			case len(cmd.Argv) >= 5 && cmd.Argv[0] == "git" && cmd.Argv[3] == "checkout":
				if got := cmd.Argv[len(cmd.Argv)-1]; got != "FETCH_HEAD" {
					t.Fatalf("checkout target = %q, want FETCH_HEAD", got)
				}
				return ports.CommandResult{ExitCode: 0}, nil
			case len(cmd.Argv) >= 5 && cmd.Argv[0] == "git" && cmd.Argv[3] == "rev-parse":
				return ports.CommandResult{ExitCode: 0, Stdout: []byte("fedcba\n")}, nil
			default:
				t.Fatalf("unexpected command: %+v", cmd.Argv)
				return ports.CommandResult{}, nil
			}
		}},
	}

	resolved, err := resolver.Resolve(context.Background(), domain.IntegrationRef{Raw: "github:acme/demo@v1.2.3//plugin"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !sawFetch {
		t.Fatal("expected pinned ref fetch")
	}
	if resolved.Resolved.Value != "https://github.com/acme/demo@fedcba" {
		t.Fatalf("resolved ref = %s", resolved.Resolved.Value)
	}
	if resolved.CanonicalSource != "https://github.com/acme/demo//plugin" || resolved.Repository != "acme/demo" || resolved.PackageSubpath != "plugin" || resolved.ResolvedRevision != "fedcba" {
		t.Fatalf("structured provenance = %+v", resolved)
	}
	t.Cleanup(func() { _ = resolved.Cleanup() })
}

func TestParseGitURLRefNormalizesRefFragment(t *testing.T) {
	t.Parallel()

	repoURL, gitRef, ok := parseGitURLRef("https://example.com/acme/demo.git#ref=v1.2.3")
	if !ok {
		t.Fatal("expected git url to parse")
	}
	if repoURL != "https://example.com/acme/demo.git" {
		t.Fatalf("repo url = %q", repoURL)
	}
	if gitRef != "v1.2.3" {
		t.Fatalf("git ref = %q", gitRef)
	}
}

func TestParseGitURLRejectsCredentialsAndOptionLikeInputs(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"https://user:token@example.com/acme/demo.git",
		"https://example.com/acme/demo.git?token=secret",
		"ssh://git:secret@example.com/acme/demo.git",
		"--upload-pack=demo.git",
		"https://example.com/acme/demo.git#--upload-pack=evil",
	} {
		if _, _, ok := parseGitURLRef(raw); ok {
			t.Fatalf("credentialed or option-like source %q was accepted", raw)
		}
	}
}

func TestParseGitHubRefRejectsUnsafeSubdir(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"github:acme/demo@--upload-pack=evil",
		"github:ac me/demo",
		"github:acme/de:mo",
		"github:acme/demo//../outside",
		"github:acme/demo//plugins/../outside",
		"github:acme/demo///absolute",
		"github:acme/demo//C:/outside",
		`github:acme/demo//plugins\demo`,
		"github:acme/demo//plugins//demo",
		"github:acme/demo//CON",
	} {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			if _, _, _, ok := parseGitHubRef(raw); ok {
				t.Fatalf("parseGitHubRef(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

func TestCloneRejectsUnsafeSubdirBeforeRunningGit(t *testing.T) {
	t.Parallel()

	resolver := Resolver{Runner: stubRunner{run: func(_ context.Context, cmd ports.Command) (ports.CommandResult, error) {
		t.Fatalf("unexpected command for unsafe subdir: %+v", cmd.Argv)
		return ports.CommandResult{}, nil
	}}}
	if _, err := resolver.clone(context.Background(), "https://github.com/acme/demo.git", "../outside", ""); err == nil {
		t.Fatal("expected unsafe subdir error")
	}
}

func TestHashLocalTreeIncludesExecutableIntent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "bin", "server")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("server\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	nonExecutable, err := hashLocalTree(root)
	if err != nil {
		t.Fatalf("hash non-executable tree: %v", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	executable, err := hashLocalTree(root)
	if err != nil {
		t.Fatalf("hash executable tree: %v", err)
	}
	if executable == nonExecutable {
		t.Fatal("digest did not change when executable intent changed")
	}
}

func TestHashLocalTreeIgnoresGitMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	before, err := hashLocalTree(root)
	if err != nil {
		t.Fatalf("hash before git metadata: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git", "objects"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "objects", "noise"), []byte("noise"), 0o644); err != nil {
		t.Fatalf("write .git metadata: %v", err)
	}
	after, err := hashLocalTree(root)
	if err != nil {
		t.Fatalf("hash after git metadata: %v", err)
	}
	if after != before {
		t.Fatalf("digest changed because of .git metadata: before=%s after=%s", before, after)
	}
}

func TestHashLocalTreeRejectsSymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked-secret")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := hashLocalTree(root); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("hash error = %v, want symlink rejection", err)
	}
}

func TestHashLocalTreeRejectsNonPortablePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "NUL"), []byte("reserved"), 0o644); err != nil {
		t.Fatalf("write reserved path: %v", err)
	}
	if _, err := hashLocalTree(root); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("hash error = %v, want reserved-name rejection", err)
	}
}

func TestResolverCanDisableLegacyAliasesForStandardFirstCLI(t *testing.T) {
	t.Parallel()
	resolver := Resolver{DisableAliases: true}
	if _, err := resolver.Resolve(context.Background(), domain.IntegrationRef{Raw: "context7"}); err == nil {
		t.Fatal("legacy alias was resolved for standard-first CLI")
	}
}
