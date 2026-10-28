package agentpluginscli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestInstallScopeSelection(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(parent, "uap.toml"), "schema_version = 1\n")
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	for _, tc := range []struct {
		name, cwd, explicit, want string
		projectRoot               string
	}{
		{name: "exact directory", cwd: parent, want: "project"},
		{name: "no parent traversal", cwd: child, want: "user"},
		{name: "explicit user", cwd: parent, explicit: "user", want: "user"},
		{name: "explicit project", cwd: child, explicit: "project", want: "project"},
		{name: "explicit project root", cwd: parent, explicit: "project", want: "project", projectRoot: child},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.Chdir(tc.cwd); err != nil {
				t.Fatal(err)
			}
			opts := &options{projectRoot: tc.projectRoot}
			cmd := &cobra.Command{}
			cmd.Flags().StringVar(&opts.scope, "scope", "user", "")
			if tc.explicit != "" {
				if err := cmd.Flags().Set("scope", tc.explicit); err != nil {
					t.Fatal(err)
				}
			}
			if err := selectInstallScope(cmd, opts); err != nil {
				t.Fatal(err)
			}
			if opts.scope != tc.want {
				t.Fatalf("scope = %s, want %s", opts.scope, tc.want)
			}
			if tc.projectRoot != "" && opts.projectRoot != tc.projectRoot {
				t.Fatal("explicit project root replaced")
			}
			expectedRoot, err := filepath.EvalSymlinks(tc.cwd)
			if err != nil {
				t.Fatal(err)
			}
			if tc.explicit == "" && tc.want == "project" && opts.projectRoot != expectedRoot {
				t.Fatalf("inferred root %s", opts.projectRoot)
			}
		})
	}
}

func TestInstallExistingLockIsStrict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uap.lock")
	if strict, err := installLockedOnly(path, false); err != nil || strict {
		t.Fatalf("missing lock: %v %v", strict, err)
	}
	if strict, err := installLockedOnly(path, true); err != nil || !strict {
		t.Fatalf("explicit locked: %v %v", strict, err)
	}
	// Even an empty or malformed lock must take the strict path, never silently
	// become an unlocked first install.
	dependencyWrite(t, path, "")
	if strict, err := installLockedOnly(path, false); err != nil || !strict {
		t.Fatalf("existing lock: %v %v", strict, err)
	}
}

func TestInstallAutoScopeDispatch(t *testing.T) {
	root := t.TempDir()
	dependencyWrite(t, filepath.Join(root, "uap.toml"), "schema_version = 1\n")
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	selected := errors.New("project runtime selected")
	app := App{ForScope: func(scope domain.InstallScope, projectRoot string) (App, error) {
		canonical, err := canonicalProjectRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		if scope != domain.ScopeProject || projectRoot != canonical {
			t.Fatalf("selected %s %s", scope, projectRoot)
		}
		return App{}, selected
	}}
	command := NewRoot(app)
	command.SetArgs([]string{"install", "--dry-run"})
	if err := command.Execute(); !errors.Is(err, selected) {
		t.Fatalf("auto scope not dispatched: %v", err)
	}
}

func TestInstallPreviewShowsSelection(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	opts := &options{format: "human", installSelection: &installSelection{Scope: "project", ProjectRoot: "/fixture/project", ManifestPath: "/fixture/project/uap.toml", LockPath: "/fixture/project/uap.lock"}}
	if err := renderDependencySync(cmd, opts, nil, true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Scope: project", "Project: /fixture/project", "Manifest: /fixture/project/uap.toml", "Lock: /fixture/project/uap.lock"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}

func TestInstallAutoScopeRejectsNonregularManifest(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	for _, kind := range []string{"directory", "symlink", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "uap.toml")
			switch kind {
			case "directory":
				err = os.Mkdir(path, 0700)
			case "symlink":
				target := filepath.Join(root, "actual.toml")
				dependencyWrite(t, target, "schema_version = 1\n")
				err = os.Symlink(target, path)
			default:
				err = os.Symlink(filepath.Join(root, "missing.toml"), path)
			}
			if err != nil {
				t.Skipf("fixture unavailable: %v", err)
			}
			if err = os.Chdir(root); err != nil {
				t.Fatal(err)
			}
			cmd := &cobra.Command{}
			opts := &options{}
			cmd.Flags().StringVar(&opts.scope, "scope", "user", "")
			if err := selectInstallScope(cmd, opts); err == nil {
				t.Fatal("nonregular manifest accepted or silently fell back to user scope")
			}
			if err := cmd.Flags().Set("scope", "user"); err != nil {
				t.Fatal(err)
			}
			if err := selectInstallScope(cmd, opts); err != nil {
				t.Fatalf("explicit user did not override: %v", err)
			}
			if err := os.Chdir(original); err != nil {
				t.Fatal(err)
			}
		})
	}
}
