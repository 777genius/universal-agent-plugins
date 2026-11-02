package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
)

type dependencyScopeFile struct {
	data []byte
	info os.FileInfo
}
type dependencyScopePair struct {
	manifest dependencyScopeFile
	lock     dependencyScopeFile
}

func runDependencyScopeCLI(t *testing.T, app agentpluginscli.App, args ...string) error {
	t.Helper()
	var out bytes.Buffer
	root := agentpluginscli.NewRoot(app)
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	return root.ExecuteContext(t.Context())
}

func captureDependencyScopeFile(t *testing.T, path string) dependencyScopeFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("nonregular dependency file: %s", path)
	}
	return dependencyScopeFile{data: data, info: info}
}

func captureDependencyScope(t *testing.T, root string) dependencyScopePair {
	t.Helper()
	return dependencyScopePair{
		manifest: captureDependencyScopeFile(t, filepath.Join(root, "uap.toml")),
		lock:     captureDependencyScopeFile(t, filepath.Join(root, "uap.lock")),
	}
}

func assertDependencyScopeFileSame(t *testing.T, name string, before, after dependencyScopeFile) {
	t.Helper()
	if !bytes.Equal(before.data, after.data) || !os.SameFile(before.info, after.info) || !before.info.ModTime().Equal(after.info.ModTime()) {
		t.Fatalf("%s was rewritten across an unrelated scope", name)
	}
}

func assertDependencyScopeSame(t *testing.T, name string, before, after dependencyScopePair) {
	t.Helper()
	assertDependencyScopeFileSame(t, name+" manifest", before.manifest, after.manifest)
	assertDependencyScopeFileSame(t, name+" lock", before.lock, after.lock)
}

// Exercise the real public CLI and three distinct compositions. Manifest/lock,
// inode and timestamp of unrelated scopes must survive updates and refusals.
func TestProjectDependencyLockNeverCrossesUserProjectScopeE2E(t *testing.T) {
	app, user, a, b := projectCompositionFixture(t)
	scopes := []struct {
		root string
		args []string
	}{
		{user, []string{"--scope", "user"}},
		{a, []string{"--scope", "project", "--project-root", a}},
		{b, []string{"--scope", "project", "--project-root", b}},
	}
	for _, selected := range scopes {
		if err := runDependencyScopeCLI(t, app, append([]string{"deps", "init"}, selected.args...)...); err != nil {
			t.Fatalf("deps init for %s: %v", selected.root, err)
		}
		if err := runDependencyScopeCLI(t, app, append([]string{"deps", "lock"}, selected.args...)...); err != nil {
			t.Fatalf("deps lock for %s: %v", selected.root, err)
		}
	}
	userBefore, aBefore, bBefore := captureDependencyScope(t, user), captureDependencyScope(t, a), captureDependencyScope(t, b)
	aAfter := changeOnlyProjectAScope(t, app, a, aBefore)
	assertDependencyScopeSame(t, "user", userBefore, captureDependencyScope(t, user))
	assertDependencyScopeSame(t, "project B", bBefore, captureDependencyScope(t, b))
	refuseCrossScopeMutation(t, app, user, a, b, userBefore, aAfter, bBefore)
}

func changeOnlyProjectAScope(t *testing.T, app agentpluginscli.App, root string, before dependencyScopePair) dependencyScopePair {
	t.Helper()
	changed := append(bytes.Clone(before.manifest.data), []byte("codex = []\n")...)
	if err := os.WriteFile(filepath.Join(root, "uap.toml"), changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runDependencyScopeCLI(t, app, "deps", "lock", "--scope", "project", "--project-root", root); err != nil {
		t.Fatalf("project A lock update: %v", err)
	}
	after := captureDependencyScope(t, root)
	if bytes.Equal(before.lock.data, after.lock.data) {
		t.Fatal("project A lock did not change after selected target intent changed")
	}
	if err := runDependencyScopeCLI(t, app, "deps", "lock", "--scope", "project", "--project-root", root); err != nil {
		t.Fatalf("project A no-op lock: %v", err)
	}
	assertDependencyScopeSame(t, "project A no-op", after, captureDependencyScope(t, root))
	return after
}

func refuseCrossScopeMutation(t *testing.T, app agentpluginscli.App, user, a, b string, userBefore, aBefore, bBefore dependencyScopePair) {
	t.Helper()
	if err := runDependencyScopeCLI(t, app, "deps", "lock", "--scope", "user", "--project-root", a); err == nil {
		t.Fatal("accepted contradictory user scope and project root")
	}
	missing := filepath.Join(t.TempDir(), "nonexistent-project")
	if err := runDependencyScopeCLI(t, app, "deps", "lock", "--scope", "project", "--project-root", missing); err == nil {
		t.Fatal("silently accepted an unselected project")
	}
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Fatalf("unselected project appeared: %v", err)
	}
	assertDependencyScopeSame(t, "user after refusals", userBefore, captureDependencyScope(t, user))
	assertDependencyScopeSame(t, "project A after refusals", aBefore, captureDependencyScope(t, a))
	assertDependencyScopeSame(t, "project B after refusals", bBefore, captureDependencyScope(t, b))
}
