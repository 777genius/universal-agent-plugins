package integrationctl

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/evidence"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/jsonstate"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/workspacelock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestNormalizeTargetsTrimsAndLowercases(t *testing.T) {
	got := NormalizeTargets([]string{" Claude ", "", "GEMINI", " codex "})
	want := []string{"claude", "gemini", "codex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeTargets() = %#v want %#v", got, want)
	}
}

func TestDiscoverRepoRootFallsBackToStart(t *testing.T) {
	root := t.TempDir()
	start := filepath.Join(root, "nested", "workspace")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := discoverRepoRoot(start); got != start {
		t.Fatalf("discoverRepoRoot() = %q want %q", got, start)
	}
}

func TestNewServiceUsesHomeAndRepoDerivedPaths(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	cwd := filepath.Join(repo, "tools", "integrationctl")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "docs", "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})
	t.Setenv("HOME", home)
	actualCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	actualRepo := discoverRepoRoot(actualCwd)

	svc, err := newService()
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}
	if svc.CurrentWorkspaceRoot != actualCwd {
		t.Fatalf("CurrentWorkspaceRoot = %q want %q", svc.CurrentWorkspaceRoot, actualCwd)
	}

	stateStore, ok := svc.StateStore.(jsonstate.Store)
	if !ok {
		t.Fatalf("StateStore type = %T", svc.StateStore)
	}
	if want := filepath.Join(home, ".plugin-kit-ai", "state.json"); stateStore.Path != want {
		t.Fatalf("StateStore.Path = %q want %q", stateStore.Path, want)
	}

	lockStore, ok := svc.WorkspaceLock.(workspacelock.Store)
	if !ok {
		t.Fatalf("WorkspaceLock type = %T", svc.WorkspaceLock)
	}
	if want := filepath.Join(actualRepo, ".uap-integrations.lock"); lockStore.File != want {
		t.Fatalf("WorkspaceLock.File = %q want %q", lockStore.File, want)
	}

	evidenceRegistry, ok := svc.Evidence.(evidence.Registry)
	if !ok {
		t.Fatalf("Evidence type = %T", svc.Evidence)
	}
	if want := filepath.Join(actualRepo, "docs", "generated", "integrationctl_evidence_registry.json"); evidenceRegistry.Path != want {
		t.Fatalf("Evidence.Path = %q want %q", evidenceRegistry.Path, want)
	}

	claudeAdapter, ok := svc.Adapters[domain.TargetClaude].(claude.Adapter)
	if !ok {
		t.Fatalf("Claude adapter type = %T", svc.Adapters[domain.TargetClaude])
	}
	if claudeAdapter.ProjectRoot != actualCwd || claudeAdapter.UserHome != home {
		t.Fatalf("Claude adapter = %+v", claudeAdapter)
	}
}
