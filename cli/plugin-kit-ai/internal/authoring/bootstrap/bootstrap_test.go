package bootstrap_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/bootstrap"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
)

func generated(t *testing.T) (string, project.Result) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "generated")
	plan, err := scaffold.BuildPlan(scaffold.Options{Template: scaffold.MCPStdio, Name: "demo", Description: "Disposable test package.", Runtime: "node"})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range plan.Files() {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, file.Bytes, file.Mode); err != nil {
			t.Fatal(err)
		}
	}
	p, err := (project.Service{Scratch: t.TempDir()}).Read(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, p
}

func errorCode(err error) string {
	var target *bootstrap.Error
	if errors.As(err, &target) {
		return target.Code
	}
	return ""
}

func TestPlanRecognizesOnlyExactGeneratedLockedNodeTemplate(t *testing.T) {
	root, p := generated(t)
	plan, err := (bootstrap.Service{}).Plan(root, p)
	if err != nil || plan.Runtime != "node" || plan.Manager != "npm" || !reflect.DeepEqual(plan.Command, []string{"npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund"}) {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
	defer plan.Close()
	for _, tc := range []struct{ name, path, body, code string }{
		{"missing lock", "package-lock.json", "", "bootstrap_lock_required"},
		{"changed package", "package.json", `{}`, "bootstrap_template_unrecognized"},
		{"python ambiguity", "pyproject.toml", `[project]`, "bootstrap_layout_ambiguous"},
		{"go ambiguity", "go.mod", `module example.test/demo`, "bootstrap_layout_ambiguous"},
		{"manager ambiguity", "yarn.lock", `fixture`, "bootstrap_layout_ambiguous"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copyRoot, copyProject := generated(t)
			path := filepath.Join(copyRoot, tc.path)
			if tc.name == "missing lock" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := (bootstrap.Service{}).Plan(copyRoot, copyProject)
			if errorCode(err) != tc.code {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestApplyUsesIsolatedHomesAndCommitsOnlyNodeModules(t *testing.T) {
	root, p := generated(t)
	plan, err := (bootstrap.Service{}).Plan(root, p)
	if err != nil {
		t.Fatal(err)
	}
	beforePackage, _ := os.ReadFile(filepath.Join(root, "package.json"))
	beforeLock, _ := os.ReadFile(filepath.Join(root, "package-lock.json"))
	calls := 0
	runner := func(_ context.Context, name string, args []string, dir string, env []string) error {
		calls++
		if name != "npm" || !reflect.DeepEqual(args, plan.Command[1:]) {
			t.Fatalf("process = %s %v", name, args)
		}
		joined := "\n" + strings.Join(env, "\n") + "\n"
		for _, forbidden := range []string{"HTTP_PROXY=", "HTTPS_PROXY=", "NPM_TOKEN=", "NODE_AUTH_TOKEN="} {
			if strings.Contains(strings.ToUpper(joined), "\n"+forbidden) {
				t.Fatalf("ambient variable inherited: %s", forbidden)
			}
		}
		for _, required := range []string{"HOME=", "NPM_CONFIG_CACHE=", "NPM_CONFIG_USERCONFIG=", "NPM_CONFIG_GLOBALCONFIG=", "TMPDIR="} {
			if !strings.Contains(joined, "\n"+required) {
				t.Fatalf("missing isolated %s", required)
			}
		}
		return os.Mkdir(filepath.Join(dir, "node_modules"), 0700)
	}
	if committed, err := (bootstrap.Service{Runner: runner}).Apply(context.Background(), root, plan); err != nil || !committed {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
	if info, err := os.Stat(filepath.Join(root, "node_modules")); err != nil || !info.IsDir() {
		t.Fatalf("node_modules: %v", err)
	}
	afterPackage, _ := os.ReadFile(filepath.Join(root, "package.json"))
	afterLock, _ := os.ReadFile(filepath.Join(root, "package-lock.json"))
	if !reflect.DeepEqual(beforePackage, afterPackage) || !reflect.DeepEqual(beforeLock, afterLock) {
		t.Fatal("source or lockfile changed")
	}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".agentplugins-bootstrap-") {
			t.Fatal("staging leaked")
		}
	}
}

func TestApplyFailureRemovesPartialStaging(t *testing.T) {
	root, p := generated(t)
	plan, err := (bootstrap.Service{}).Plan(root, p)
	if err != nil {
		t.Fatal(err)
	}
	failure := func(_ context.Context, _ string, _ []string, dir string, _ []string) error {
		_ = os.Mkdir(filepath.Join(dir, "node_modules"), 0700)
		return errors.New("fixture failure")
	}
	committed, err := (bootstrap.Service{Runner: failure}).Apply(context.Background(), root, plan)
	if errorCode(err) != "bootstrap_process_failed" {
		t.Fatalf("error = %v", err)
	}
	if committed {
		t.Fatal("failure claimed commit")
	}
	if _, err := os.Stat(filepath.Join(root, "node_modules")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial dependency tree committed")
	}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".agentplugins-bootstrap-") {
			t.Fatal("staging leaked")
		}
	}
}

func TestPlanAndApplyRejectSourceIdentityChanges(t *testing.T) {
	t.Run("root symlink", func(t *testing.T) {
		root, p := generated(t)
		link := root + "-link"
		if err := os.Symlink(root, link); err != nil {
			t.Skipf("directory symlinks unavailable: %v", err)
		}
		if _, err := (bootstrap.Service{}).Plan(link, p); errorCode(err) != "bootstrap_template_unrecognized" {
			t.Fatalf("root symlink plan error = %v", err)
		}
	})
	t.Run("source symlink", func(t *testing.T) {
		root, p := generated(t)
		lock := filepath.Join(root, "package-lock.json")
		body, err := os.ReadFile(lock)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(lock); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "lock")
		if err := os.WriteFile(target, body, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, lock); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if _, err := (bootstrap.Service{}).Plan(root, p); errorCode(err) != "bootstrap_lock_required" {
			t.Fatalf("symlink plan error = %v", err)
		}
	})
	t.Run("source directory symlink", func(t *testing.T) {
		root, p := generated(t)
		source := filepath.Join(root, "src")
		moved := filepath.Join(root, "src-real")
		if err := os.Rename(source, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(moved, source); err != nil {
			t.Skipf("directory symlinks unavailable: %v", err)
		}
		if _, err := (bootstrap.Service{}).Plan(root, p); errorCode(err) != "bootstrap_template_unrecognized" {
			t.Fatalf("directory symlink plan error = %v", err)
		}
	})

	for _, tc := range []struct {
		name   string
		change func(*testing.T, string)
	}{
		{"lock replacement", func(t *testing.T, root string) {
			path := filepath.Join(root, "package-lock.json")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"root replacement", func(t *testing.T, root string) {
			moved := root + "-moved"
			if err := os.Rename(root, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, p := generated(t)
			plan, err := (bootstrap.Service{}).Plan(root, p)
			if err != nil {
				t.Fatal(err)
			}
			tc.change(t, root)
			called := false
			committed, err := (bootstrap.Service{Runner: func(context.Context, string, []string, string, []string) error {
				called = true
				return nil
			}}).Apply(context.Background(), root, plan)
			if committed || called || errorCode(err) != "bootstrap_source_changed" {
				t.Fatalf("apply = %t, %v; called=%t", committed, err, called)
			}
		})
	}
}

func TestApplyRejectsLateSourceAndDestinationChanges(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, string)
		code   string
	}{
		{"source", func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "package-lock.json"), []byte("changed"), 0600); err != nil {
				t.Fatal(err)
			}
		}, "bootstrap_source_changed"},
		{"destination file", func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "node_modules"), []byte("winner"), 0600); err != nil {
				t.Fatal(err)
			}
		}, "bootstrap_destination_exists"},
		{"destination symlink", func(t *testing.T, root string) {
			target := filepath.Join(t.TempDir(), "outside")
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(root, "node_modules")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		}, "bootstrap_destination_exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, p := generated(t)
			plan, err := (bootstrap.Service{}).Plan(root, p)
			if err != nil {
				t.Fatal(err)
			}
			runner := func(_ context.Context, _ string, _ []string, dir string, _ []string) error {
				if err := os.Mkdir(filepath.Join(dir, "node_modules"), 0700); err != nil {
					return err
				}
				tc.mutate(t, root)
				return nil
			}
			committed, err := (bootstrap.Service{Runner: runner}).Apply(context.Background(), root, plan)
			if committed || errorCode(err) != tc.code {
				t.Fatalf("apply = %t, %v", committed, err)
			}
			if tc.name != "source" {
				body, readErr := os.ReadFile(filepath.Join(root, "node_modules"))
				if tc.name == "destination file" && (readErr != nil || string(body) != "winner") {
					t.Fatalf("destination winner replaced: %q, %v", body, readErr)
				}
			}
		})
	}
}

func TestApplyReportsCleanupFailureWithoutHidingCommit(t *testing.T) {
	root, p := generated(t)
	plan, err := (bootstrap.Service{}).Plan(root, p)
	if err != nil {
		t.Fatal(err)
	}
	runner := func(_ context.Context, _ string, _ []string, dir string, _ []string) error {
		if err := os.Mkdir(filepath.Join(dir, "node_modules"), 0700); err != nil {
			return err
		}
		stage := filepath.Dir(dir)
		moved := stage + "-owned"
		if err := os.Rename(stage, moved); err != nil {
			return err
		}
		return os.Mkdir(stage, 0700)
	}
	committed, err := (bootstrap.Service{Runner: runner}).Apply(context.Background(), root, plan)
	if !committed || errorCode(err) != "bootstrap_cleanup_failed" || !strings.Contains(err.Error(), "staging ownership changed") {
		t.Fatalf("apply = %t, %v", committed, err)
	}
	if info, statErr := os.Stat(filepath.Join(root, "node_modules")); statErr != nil || !info.IsDir() {
		t.Fatalf("atomic commit missing after cleanup failure: %v", statErr)
	}
}

func TestApplyJoinsProcessAndCleanupFailures(t *testing.T) {
	root, p := generated(t)
	plan, err := (bootstrap.Service{}).Plan(root, p)
	if err != nil {
		t.Fatal(err)
	}
	processErr := errors.New("primary fixture process error")
	runner := func(_ context.Context, _ string, _ []string, dir string, _ []string) error {
		stage := filepath.Dir(dir)
		if err := os.Rename(stage, stage+"-owned"); err != nil {
			return err
		}
		if err := os.Mkdir(stage, 0700); err != nil {
			return err
		}
		return processErr
	}
	committed, err := (bootstrap.Service{Runner: runner}).Apply(context.Background(), root, plan)
	if committed || errorCode(err) != "bootstrap_process_failed" || !errors.Is(err, processErr) || !strings.Contains(err.Error(), "bootstrap_cleanup_failed") {
		t.Fatalf("joined apply error = %t, %v", committed, err)
	}
}
