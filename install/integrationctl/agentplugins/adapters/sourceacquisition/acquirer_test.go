package sourceacquisition

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestAcquireGitHubExactSHASparselySnapshotsOnlyPluginRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	repository := newRepository(t)
	writeRepo(t, repository, "plugin/plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sparse"}`, 0o644)
	writeRepo(t, repository, "plugin/bin/server", "fixture", 0o755)
	writeRepo(t, repository, "unrelated/large", strings.Repeat("x", 4096), 0o644)
	revision := commit(t, repository)
	tempRoot := t.TempDir()
	acquirer := Acquirer{
		TempRoot:   tempRoot,
		Runner:     localGitTestRunner{},
		Digester:   packagedigest.Builder{TempRoot: tempRoot, Limits: domain.PackageLimits{MaxTreeBytes: 1024}},
		URLForRepo: func(string) string { return repository },
	}
	snapshot, err := acquirer.AcquireGitHub(context.Background(), "example/plugin", revision, "plugin")
	if err != nil {
		t.Fatal(err)
	}
	defer packagedigest.Remove(snapshot)
	if snapshot.Source.ResolvedRevision != revision || snapshot.Source.PackageSubpath != "plugin" || snapshot.FileCount != 2 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if _, err := os.Stat(filepath.Join(snapshot.Root, "unrelated")); !os.IsNotExist(err) {
		t.Fatalf("unrelated monorepo content entered snapshot: %v", err)
	}
	if len(snapshot.ExecutableFiles) != 1 || snapshot.ExecutableFiles[0] != "bin/server" {
		t.Fatalf("executables = %v", snapshot.ExecutableFiles)
	}
}

func TestDiscoverGitHubPackagesPrefersRepositoryRootManifest(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	repository := newRepository(t)
	writeRepo(t, repository, "plugin.json", `{}`, 0o644)
	writeRepo(t, repository, "mcp.json", `{}`, 0o644)
	writeRepo(t, repository, "packages/nested/plugin.json", `{}`, 0o644)
	writeRepo(t, repository, "packages/nested/mcp.json", `{}`, 0o644)
	revision := commit(t, repository)
	acquirer := Acquirer{TempRoot: t.TempDir(), Runner: localGitTestRunner{}, URLForRepo: func(string) string { return repository }}

	paths, err := acquirer.DiscoverGitHubPackages(context.Background(), "example/plugin", revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "" {
		t.Fatalf("root-preferred package paths = %q", paths)
	}
}

func TestDiscoverGitHubPackagesPrefersRepositoryRootNativeManifest(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	repository := newRepository(t)
	writeRepo(t, repository, ".codex-plugin/plugin.json", `{"name":"native"}`, 0o644)
	writeRepo(t, repository, "packages/nested/plugin.json", `{}`, 0o644)
	writeRepo(t, repository, "packages/nested/mcp.json", `{}`, 0o644)
	revision := commit(t, repository)
	acquirer := Acquirer{TempRoot: t.TempDir(), Runner: localGitTestRunner{}, URLForRepo: func(string) string { return repository }}

	paths, err := acquirer.DiscoverGitHubPackages(context.Background(), "example/plugin", revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "" {
		t.Fatalf("root-native package paths = %q", paths)
	}
}

func TestDiscoverGitHubPackagesReturnsOnlyPackageShapedNestedDirectories(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	repository := newRepository(t)
	writeRepo(t, repository, "manifest-only/plugin.json", `{}`, 0o644)
	writeRepo(t, repository, "packages/zeta/plugin.json", `{}`, 0o644)
	writeRepo(t, repository, "packages/zeta/skills/demo/SKILL.md", "# Demo", 0o644)
	writeRepo(t, repository, "packages/alpha/plugin.json", `{}`, 0o644)
	writeRepo(t, repository, "packages/alpha/mcp.json", `{}`, 0o644)
	writeRepo(t, repository, "packages/not-a-plugin/mcp.json", `{}`, 0o644)
	revision := commit(t, repository)
	acquirer := Acquirer{TempRoot: t.TempDir(), Runner: localGitTestRunner{}, URLForRepo: func(string) string { return repository }}

	paths, err := acquirer.DiscoverGitHubPackages(context.Background(), "example/plugin", revision)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"packages/alpha", "packages/zeta"}
	if strings.Join(paths, "|") != strings.Join(want, "|") {
		t.Fatalf("nested package paths = %q, want %q", paths, want)
	}
}

func TestPackagePathsFromTreeRejectsMalformedEntries(t *testing.T) {
	if _, err := packagePathsFromTree([]byte("100644 blob hash-without-path\x00")); err == nil {
		t.Fatal("malformed Git tree entry was accepted")
	}
}

func TestPackagePathsFromTreeIgnoresSymlinkAndGitlinkCandidates(t *testing.T) {
	tree := []byte("120000 blob a\tpackages/symlink/plugin.json\x00" +
		"100644 blob b\tpackages/symlink/mcp.json\x00" +
		"100644 blob c\tpackages/gitlink/plugin.json\x00" +
		"160000 commit d\tpackages/gitlink/mcp.json\x00" +
		"100644 blob e\tpackages/valid/plugin.json\x00" +
		"100644 blob f\tpackages/valid/skills/demo/SKILL.md\x00")
	paths, err := packagePathsFromTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "packages/valid" {
		t.Fatalf("regular-file candidates = %q", paths)
	}
}

func TestDiscoverGitHubPackagesBoundsGitTreeMetadata(t *testing.T) {
	revision := strings.Repeat("a", 40)
	runner := &discoveryLimitRunner{revision: revision}
	acquirer := Acquirer{TempRoot: t.TempDir(), Runner: runner}
	_, err := acquirer.DiscoverGitHubPackages(context.Background(), "example/plugin", revision)
	if err == nil || !strings.Contains(err.Error(), "choose a package explicitly with //path") {
		t.Fatalf("metadata limit error = %v", err)
	}
	if !runner.sawBoundedTree {
		t.Fatal("Git tree inspection was not output-bounded")
	}
}

func TestOSRunnerClassifiesRealGitTreeOverflowAsSoleLimit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	repository := newRepository(t)
	for index := range 1024 {
		writeRepo(t, repository, fmt.Sprintf("packages/plugin-%04d/plugin.json", index), "{}", 0o644)
	}
	revision := commit(t, repository)

	_, err := (OSRunner{}).Run(context.Background(), Command{
		Dir:            t.TempDir(),
		Args:           []string{"-C", repository, "ls-tree", "-r", "-z", revision},
		MaxOutputBytes: 128,
	})
	if !processadapter.IsOnlyStdoutLimitExceeded(err) {
		t.Fatalf("real Git metadata limit error = %v", err)
	}
}

func TestDiscoverGitHubPackagesDoesNotMaskConcurrentLimitFailure(t *testing.T) {
	revision := strings.Repeat("c", 40)
	runner := &joinedDiscoveryLimitRunner{revision: revision}
	acquirer := Acquirer{TempRoot: t.TempDir(), Runner: runner}
	_, err := acquirer.DiscoverGitHubPackages(context.Background(), "example/plugin", revision)
	if err == nil || strings.Contains(err.Error(), "metadata exceeds") || !strings.Contains(err.Error(), "inspect repository") {
		t.Fatalf("joined discovery failure = %v", err)
	}
}

func TestDiscoverGitHubPackagesFindsRootBeforeRecursiveMetadataLimit(t *testing.T) {
	revision := strings.Repeat("b", 40)
	runner := &rootBeforeRecursiveLimitRunner{revision: revision}
	acquirer := Acquirer{TempRoot: t.TempDir(), Runner: runner}
	paths, err := acquirer.DiscoverGitHubPackages(context.Background(), "example/plugin", revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "" {
		t.Fatalf("root package paths = %q", paths)
	}
	if runner.recursiveCalls != 0 {
		t.Fatalf("root package triggered %d recursive tree scans", runner.recursiveCalls)
	}
}

func TestAcquireGitHubRepositoryRootExcludesGitDirectoryAndSubmoduleContent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	submodule := newRepository(t)
	writeRepo(t, submodule, "README", "unrelated submodule content", 0o644)
	_ = commit(t, submodule)
	repository := newRepository(t)
	writeRepo(t, repository, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"root"}`, 0o644)
	runGit(t, "-c", "protocol.file.allow=always", "-C", repository, "submodule", "add", "--quiet", submodule, "third_party/unrelated")
	revision := commit(t, repository)
	acquirer := Acquirer{TempRoot: t.TempDir(), Runner: localGitTestRunner{}, URLForRepo: func(string) string { return repository }}
	snapshot, err := acquirer.AcquireGitHub(context.Background(), "example/root-plugin", revision, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := packagedigest.Remove(snapshot); err != nil {
			t.Errorf("remove repository-root snapshot: %v", err)
		}
	})
	if snapshot.FileCount != 2 || snapshot.Source.RequestedSource != "example/root-plugin@"+revision || strings.HasSuffix(snapshot.Source.CanonicalSource, "//") {
		t.Fatalf("root snapshot identity = %+v", snapshot)
	}
	if _, err := os.Lstat(filepath.Join(snapshot.Root, ".git")); !os.IsNotExist(err) {
		t.Fatalf("Git metadata entered repository-root snapshot: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(snapshot.Root, "third_party", "unrelated", "README")); !os.IsNotExist(err) {
		t.Fatalf("unfetched submodule content entered repository-root snapshot: %v", err)
	}
}

func TestAcquireGitHubRejectsMutableRevisionAndUnsafeSubpathBeforeGit(t *testing.T) {
	acquirer := Acquirer{Runner: panicRunner{}}
	for _, test := range []struct{ revision, subpath string }{{"main", "plugin"}, {strings.Repeat("a", 40), "../plugin"}, {strings.ToUpper(strings.Repeat("a", 40)), "plugin"}} {
		if _, err := acquirer.AcquireGitHub(context.Background(), "example/plugin", test.revision, test.subpath); err == nil {
			t.Fatalf("accepted revision=%q subpath=%q", test.revision, test.subpath)
		}
	}
}

func TestAcquireGitHubRejectsSubmoduleInsidePluginRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	submodule := newRepository(t)
	writeRepo(t, submodule, "README", "submodule", 0o644)
	_ = commit(t, submodule)
	repository := newRepository(t)
	writeRepo(t, repository, "plugin/plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"submodule"}`, 0o644)
	runGit(t, "-c", "protocol.file.allow=always", "-C", repository, "submodule", "add", "--quiet", submodule, "plugin/vendor")
	revision := commit(t, repository)
	acquirer := Acquirer{TempRoot: t.TempDir(), Runner: localGitTestRunner{}, URLForRepo: func(string) string { return repository }}
	if _, err := acquirer.AcquireGitHub(context.Background(), "example/plugin", revision, "plugin"); err == nil || !strings.Contains(err.Error(), "submodule") {
		t.Fatalf("submodule error = %v", err)
	}
}

func TestExecutablePathsFromTreeArePluginRelative(t *testing.T) {
	tree := []byte("100644 blob a\tplugin/plugin.json\x00100755 blob b\tplugin/bin/server\x00120000 blob c\tplugin/link\x00")
	paths, err := executablePathsFromTree(tree, "plugin")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "bin/server" {
		t.Fatalf("executable paths = %v", paths)
	}
	if _, err := executablePathsFromTree([]byte("160000 commit a\tplugin/vendor\x00"), "plugin"); err == nil || !strings.Contains(err.Error(), "submodule") {
		t.Fatalf("submodule tree error = %v", err)
	}
	paths, err = executablePathsFromTree([]byte("160000 commit a\tthird_party/vendor\x00"), "")
	if err != nil || len(paths) != 0 {
		t.Fatalf("repository-root submodule executable paths = %v, %v", paths, err)
	}
}

func TestGitEnvironmentDoesNotForwardUserCredentialsOrConfiguration(t *testing.T) {
	environment := isolatedGitEnvironment([]string{
		"PATH=/usr/bin",
		"HOME=/users/example",
		"XDG_CONFIG_HOME=/users/example/.config",
		"GIT_CONFIG_PARAMETERS='http.extraHeader=Authorization: secret'",
		"GIT_ASKPASS=/tmp/credential-helper",
		"SSH_AUTH_SOCK=/tmp/agent.sock",
		"NETRC=/users/example/.netrc",
	}, "/isolated")
	joined := strings.Join(environment, "\n")
	for _, secret := range []string{"/users/example", "Authorization: secret", "credential-helper", "agent.sock", ".netrc"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("credential-bearing environment value %q was forwarded: %v", secret, environment)
		}
	}
	for _, required := range []string{"PATH=/usr/bin", "HOME=/isolated", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_KEY_0=credential.helper", "GIT_TERMINAL_PROMPT=0"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("isolated Git environment is missing %q: %v", required, environment)
		}
	}
}

func TestAcquireGitHubRedactsTemporaryPathsAndRawGitFailure(t *testing.T) {
	tempRoot := t.TempDir()
	revision := strings.Repeat("a", 40)
	runner := &failGitOperationRunner{operation: "fetch", raw: "fatal: credential rejected in " + filepath.Join(tempRoot, "agentplugins-git-secret", "repository")}
	acquirer := Acquirer{TempRoot: tempRoot, Runner: runner, URLForRepo: func(string) string { return "https://secret.invalid/token/repository.git" }}
	_, err := acquirer.AcquireGitHub(context.Background(), "example/plugin", revision, "plugin")
	if err == nil {
		t.Fatal("raw Git failure was accepted")
	}
	message := err.Error()
	for _, required := range []string{`repository "example/plugin"`, revision, "fetch immutable revision"} {
		if !strings.Contains(message, required) {
			t.Fatalf("public error omitted %q: %s", required, message)
		}
	}
	for _, secret := range []string{tempRoot, "agentplugins-git-secret", "fatal:", "credential rejected", "secret.invalid", "token"} {
		if strings.Contains(message, secret) {
			t.Fatalf("public error leaked %q: %s", secret, message)
		}
	}
}

func TestAcquireLocalRedactsInternalSnapshotPath(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "plugin.json"), []byte(`{"name":"demo"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	privateTempRoot := filepath.Join(t.TempDir(), "missing", "private-snapshots")
	_, err := (Acquirer{TempRoot: privateTempRoot}).AcquireLocal(context.Background(), source)
	if err == nil || !strings.Contains(err.Error(), "snapshot package content failed") {
		t.Fatalf("local snapshot error = %v", err)
	}
	if strings.Contains(err.Error(), privateTempRoot) || strings.Contains(err.Error(), "private-snapshots") {
		t.Fatalf("local snapshot error leaked internal path: %v", err)
	}
}

type failGitOperationRunner struct {
	operation string
	raw       string
}

func (runner *failGitOperationRunner) Run(_ context.Context, command Command) ([]byte, error) {
	for _, argument := range command.Args {
		if argument == runner.operation {
			return nil, errors.New(runner.raw)
		}
	}
	return nil, nil
}

type panicRunner struct{}

func (panicRunner) Run(context.Context, Command) ([]byte, error) {
	panic("git must not run for invalid source")
}

type discoveryLimitRunner struct {
	revision       string
	sawBoundedTree bool
}

func (runner *discoveryLimitRunner) Run(_ context.Context, command Command) ([]byte, error) {
	for _, argument := range command.Args {
		switch argument {
		case "rev-parse":
			return []byte(runner.revision + "\n"), nil
		case "ls-tree":
			for _, treeArgument := range command.Args {
				if treeArgument == "-r" {
					runner.sawBoundedTree = command.MaxOutputBytes == maxGitHubDiscoveryMetadataBytes
					return nil, processadapter.ErrStdoutLimitExceeded
				}
			}
			return nil, nil
		}
	}
	return nil, nil
}

type rootBeforeRecursiveLimitRunner struct {
	revision       string
	recursiveCalls int
}

type joinedDiscoveryLimitRunner struct{ revision string }

func (runner *joinedDiscoveryLimitRunner) Run(_ context.Context, command Command) ([]byte, error) {
	for _, argument := range command.Args {
		switch argument {
		case "rev-parse":
			return []byte(runner.revision + "\n"), nil
		case "ls-tree":
			for _, treeArgument := range command.Args {
				if treeArgument == "-r" {
					return nil, errors.Join(processadapter.ErrStdoutLimitExceeded, errors.New("synthetic containment failure"))
				}
			}
			return nil, nil
		}
	}
	return nil, nil
}

func (runner *rootBeforeRecursiveLimitRunner) Run(_ context.Context, command Command) ([]byte, error) {
	for _, argument := range command.Args {
		switch argument {
		case "rev-parse":
			return []byte(runner.revision + "\n"), nil
		case "ls-tree":
			for _, treeArgument := range command.Args {
				if treeArgument == "-r" {
					runner.recursiveCalls++
					return nil, processadapter.ErrStdoutLimitExceeded
				}
			}
			return []byte("100644 blob a\t.codex-plugin/plugin.json\x00"), nil
		}
	}
	return nil, nil
}

// localGitTestRunner keeps repository-fixture tests independent of host kernel
// cgroup delegation. Production's default OSRunner remains fail-closed and is
// covered by the process containment regressions.
type localGitTestRunner struct{}

func (localGitTestRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = isolatedGitEnvironment(os.Environ(), command.Dir)
	return cmd.CombinedOutput()
}

func newRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, "init", "--quiet", "--initial-branch=main", root)
	runGit(t, "-C", root, "config", "user.email", "test@example.invalid")
	runGit(t, "-C", root, "config", "user.name", "Test")
	return root
}

func writeRepo(t *testing.T, root, relative, body string, mode os.FileMode) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func commit(t *testing.T, root string) string {
	t.Helper()
	runGit(t, "-C", root, "add", ".")
	runGit(t, "-C", root, "commit", "--quiet", "-m", "fixture")
	return strings.TrimSpace(runGit(t, "-C", root, "rev-parse", "HEAD"))
}

func runGit(t *testing.T, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}
