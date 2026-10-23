package installer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectNamespaceRejectsAnotherProjectsSymlink(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, dir := range []string{a, filepath.Join(b, ".uap")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(b, ".uap", "foreign-marker")
	if err := os.WriteFile(marker, []byte("preserve-project-b"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(b, ".uap"), filepath.Join(a, ".uap")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := New(testConfig(t, Config{ProjectRoot: a, StateRoot: filepath.Join(a, ".uap")})); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("another project's namespace was accepted: %v", err)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "preserve-project-b" {
		t.Fatalf("foreign project changed: %q, %v", body, err)
	}
	entries, err := os.ReadDir(filepath.Join(b, ".uap"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("namespace rejection created effects: %v, %v", entries, err)
	}
}

func TestProjectNamespacePinsExistingDirectoryIdentity(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".uap")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	engine, err := New(testConfig(t, Config{ProjectRoot: root, StateRoot: state}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(state, filepath.Join(root, ".uap-original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := engine.CheckProjectNamespace(); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("replaced namespace identity was accepted: %v", err)
	}
	if _, err := engine.WithTemporaryRoot(t.TempDir()); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("temporary preview rebound a replaced namespace: %v", err)
	}
}

func TestProjectNamespacePinsSelectedProjectIdentity(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "project")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	engine, err := New(testConfig(t, Config{ProjectRoot: root, StateRoot: filepath.Join(root, ".uap")}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, filepath.Join(base, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := engine.CheckProjectNamespace(); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("replaced selected project identity was accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".uap")); !os.IsNotExist(err) {
		t.Fatalf("refused project replacement created mutable namespace: %v", err)
	}
}

func TestProjectNamespacePreviewViewsSharePhysicalAuthority(t *testing.T) {
	for _, kind := range []string{"projected engine", "temporary root"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			namespace := filepath.Join(root, ".uap")
			engine, err := New(testConfig(t, Config{ProjectRoot: root, StateRoot: namespace}))
			if err != nil {
				t.Fatal(err)
			}
			projected := *engine // PreviewInstallAfterUpdate retains this authority.
			view := &projected
			if kind == "temporary root" {
				view, err = engine.WithTemporaryRoot(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(namespace, 0700); err != nil {
				t.Fatal(err)
			}
			if err := view.CheckProjectNamespace(); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(namespace, filepath.Join(root, ".uap-original")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(namespace, 0700); err != nil {
				t.Fatal(err)
			}
			for _, observer := range []*Engine{engine, view} {
				if err := observer.CheckProjectNamespace(); !errors.Is(err, ErrPlanChanged) {
					t.Fatalf("preview view reset selected namespace authority: %v", err)
				}
			}
		})
	}
}

func TestProjectNamespaceRejectsUninitializedGuard(t *testing.T) {
	for _, engine := range []*Engine{nil, {cfg: Config{ProjectRoot: t.TempDir()}}} {
		if err := engine.CheckProjectNamespace(); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("invalid project engine accepted: %v", err)
		}
	}
}
