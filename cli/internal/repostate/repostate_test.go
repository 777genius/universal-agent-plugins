package repostate

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInspectReportsNoGitRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	state := Inspect(root)
	if !state.GitAvailable {
		t.Skip("git not available")
	}
	if state.InGitRepo || state.HasOriginRemote || state.OriginIsGitHub {
		t.Fatalf("state = %+v", state)
	}
}

func TestInspectReportsGitHubOrigin(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := exec.Command("git", "-C", root, "init").Run(); err != nil {
		t.Skipf("git init unavailable: %v", err)
	}
	if err := exec.Command("git", "-C", root, "remote", "add", "origin", "https://github.com/acme/demo.git").Run(); err != nil {
		t.Fatal(err)
	}
	state := Inspect(root)
	if !state.GitAvailable || !state.InGitRepo || !state.HasOriginRemote || !state.OriginIsGitHub {
		t.Fatalf("state = %+v", state)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Clean(state.RepoRoot); got != filepath.Clean(want) {
		t.Fatalf("repo root = %q want %q", got, root)
	}
}

func TestInspectReportsNonGitHubOrigin(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := exec.Command("git", "-C", root, "init").Run(); err != nil {
		t.Skipf("git init unavailable: %v", err)
	}
	if err := exec.Command("git", "-C", root, "remote", "add", "origin", "https://gitlab.com/acme/demo.git").Run(); err != nil {
		t.Fatal(err)
	}
	state := Inspect(root)
	if !state.InGitRepo || !state.HasOriginRemote || state.OriginIsGitHub || state.OriginHost != "gitlab.com" {
		t.Fatalf("state = %+v", state)
	}
}
