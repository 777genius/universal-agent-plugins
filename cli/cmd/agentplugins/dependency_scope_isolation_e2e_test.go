package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
)

// No mocked resolver or target adapter: run the real public CLI composition in
// a disposable user environment and two independent project roots.
func TestProjectDependencyLockNeverCrossesUserProjectScopeE2E(t *testing.T) {
	app, user, a, b := projectCompositionFixture(t)
	run := func(args ...string) error {
		t.Helper()
		var out bytes.Buffer
		root := agentpluginscli.NewRoot(app)
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		return root.ExecuteContext(t.Context())
	}
	type scoped struct {
		root string
		args []string
	}
	scopes := []scoped{
		{root: user, args: []string{"--scope", "user"}},
		{root: a, args: []string{"--scope", "project", "--project-root", a}},
		{root: b, args: []string{"--scope", "project", "--project-root", b}},
	}
	for _, s := range scopes {
		if err := run(append([]string{"deps", "init"}, s.args...)...); err != nil {
			t.Fatalf("deps init for %q: %v", s.root, err)
		}
		if err := run(append([]string{"deps", "lock"}, s.args...)...); err != nil {
			t.Fatalf("deps lock for %q: %v", s.root, err)
		}
	}
	type snapshot struct {
		data []byte
		info os.FileInfo
	}
	record := func(path string) snapshot {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("nonregular dependency file %s: %v", path, err)
		}
		return snapshot{data: data, info: info}
	}
	type pair struct{ manifest, lock snapshot }
	capture := func(root string) pair {
		t.Helper()
		return pair{record(filepath.Join(root, "uap.toml")), record(filepath.Join(root, "uap.lock"))}
	}
	same := func(name string, before, after snapshot) {
		t.Helper()
		if !bytes.Equal(before.data, after.data) || !os.SameFile(before.info, after.info) || !before.info.ModTime().Equal(after.info.ModTime()) {
			t.Fatalf("%s was rewritten across an unrelated scope", name)
		}
	}
	u0, a0, b0 := capture(user), capture(a), capture(b)

	// A target-only intent change should re-resolve project A's lock, but must
	// not write the global user or sibling project manifest/lock or receipts.
	if err := os.WriteFile(filepath.Join(a, "uap.toml"), append(bytes.Clone(a0.manifest.data), []byte("codex = []\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run("deps", "lock", "--scope", "project", "--project-root", a); err != nil {
		t.Fatalf("project A lock update: %v", err)
	}
	a1 := capture(a)
	if bytes.Equal(a0.lock.data, a1.lock.data) {
		t.Fatal("project A lock did not change with its target declaration")
	}
	u1, b1 := capture(user), capture(b)
	same("user manifest", u0.manifest, u1.manifest)
	same("user lock", u0.lock, u1.lock)
	same("project B manifest", b0.manifest, b1.manifest)
	same("project B lock", b0.lock, b1.lock)

	// A repeated lock must not rewrite even file identities/timestamps.
	if err := run("deps", "lock", "--scope", "project", "--project-root", a); err != nil {
		t.Fatalf("project A no-op lock: %v", err)
	}
	a2 := capture(a)
	same("project A no-op manifest", a1.manifest, a2.manifest)
	same("project A no-op lock", a1.lock, a2.lock)

	// User scope cannot be redirected by an unrelated project-root argument;
	// a nonexistent project must never be auto-created as a side effect.
	if err := run("deps", "lock", "--scope", "user", "--project-root", a); err == nil {
		t.Fatal("accepted an explicit user-scope/project-root mismatch")
	}
	missing := filepath.Join(t.TempDir(), "nonexistent-project")
	if err := run("deps", "lock", "--scope", "project", "--project-root", missing); err == nil {
		t.Fatal("silently created or accepted a nonexistent project")
	}
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Fatalf("unselected project path unexpectedly appeared: %v", err)
	}
	// A rejected command must preserve *every* root, including the selected
	// project from the contradictory arguments, not just user and sibling B.
	u2, a3, b2 := capture(user), capture(a), capture(b)
	same("user manifest after refusals", u0.manifest, u2.manifest)
	same("user lock after refusals", u0.lock, u2.lock)
	same("project A manifest after refusals", a2.manifest, a3.manifest)
	same("project A lock after refusals", a2.lock, a3.lock)
	same("project B manifest after refusals", b0.manifest, b2.manifest)
	same("project B lock after refusals", b0.lock, b2.lock)
}
