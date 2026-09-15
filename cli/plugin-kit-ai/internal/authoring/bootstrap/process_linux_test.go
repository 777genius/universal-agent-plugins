//go:build linux

package bootstrap_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/bootstrap"
)

func TestFinalVerificationToLaunchReplacementUsesCapturedStaging(t *testing.T) {
	root, project := generated(t)
	plan, err := (bootstrap.Service{}).Plan(root, project)
	if err != nil {
		t.Fatal(err)
	}
	wantPackage, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	moved := root + "-reviewed"
	calls := 0
	runner := func(_ context.Context, _ string, _ []string, dir string, env []string) error {
		calls++
		// This is the deterministic final-verification-to-manager-read window:
		// replace the public root name before the simulated manager opens inputs.
		if err := os.Rename(root, moved); err != nil {
			return err
		}
		if err := os.Mkdir(root, 0700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"replacement"}`), 0600); err != nil {
			return err
		}
		if !strings.HasPrefix(dir, fmt.Sprintf("/proc/%d/fd/", os.Getpid())) {
			t.Fatalf("launch directory is not handle-bound: %q", dir)
		}
		gotPackage, readErr := os.ReadFile(filepath.Join(dir, "package.json"))
		if readErr != nil || string(gotPackage) != string(wantPackage) {
			t.Fatalf("manager input came from replacement root: %q, %v", gotPackage, readErr)
		}
		values := map[string]string{}
		for _, item := range env {
			if key, value, ok := strings.Cut(item, "="); ok {
				values[key] = value
			}
		}
		for _, key := range []string{"HOME", "NPM_CONFIG_CACHE", "NPM_CONFIG_USERCONFIG", "NPM_CONFIG_GLOBALCONFIG", "NPM_CONFIG_PREFIX", "TMPDIR"} {
			value := values[key]
			if value == "" || !filepath.IsAbs(value) || !strings.HasPrefix(value, dir+string(filepath.Separator)+".."+string(filepath.Separator)) {
				t.Fatalf("%s is not anchored to captured work directory: %q", key, value)
			}
		}
		for _, key := range []string{"HOME", "NPM_CONFIG_CACHE", "TMPDIR"} {
			if info, statErr := os.Stat(values[key]); statErr != nil || !info.IsDir() {
				t.Fatalf("%s did not resolve inside captured staging: %v", key, statErr)
			}
		}
		return os.Mkdir(filepath.Join(dir, "node_modules"), 0700)
	}
	committed, err := (bootstrap.Service{Runner: runner}).Apply(context.Background(), root, plan)
	if committed || calls != 1 || errorCode(err) != "bootstrap_source_changed" {
		t.Fatalf("apply = %t, %v; calls=%d", committed, err, calls)
	}
	if _, statErr := os.Lstat(filepath.Join(root, "node_modules")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("replacement root received dependency output: %v", statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(moved, "node_modules")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed apply retained staged dependency output: %v", statErr)
	}
}

func TestDefaultRunnerExecutesOnlyLockedCommandWithSanitizedEnvironment(t *testing.T) {
	root, project := generated(t)
	plan, err := (bootstrap.Service{}).Plan(root, project)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := `#!/bin/sh
set -eu
test "$#" -eq 4
test "$1" = ci
test "$2" = --ignore-scripts
test "$3" = --no-audit
test "$4" = --no-fund
test -n "$HOME"
test -n "$NPM_CONFIG_CACHE"
test -n "$NPM_CONFIG_USERCONFIG"
test -n "$NPM_CONFIG_GLOBALCONFIG"
test -n "$TMPDIR"
test -z "${NPM_TOKEN:-}"
test -z "${NODE_AUTH_TOKEN:-}"
test -z "${GH_TOKEN:-}"
test -z "${GITHUB_TOKEN:-}"
test -z "${HTTPS_PROXY:-}"
test -z "${http_proxy:-}"
test -z "${npm_config_proxy:-}"
/bin/mkdir node_modules
`
	if err := os.WriteFile(filepath.Join(bin, "npm"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("NPM_TOKEN", "must-not-reach-process")
	t.Setenv("NODE_AUTH_TOKEN", "must-not-reach-process")
	t.Setenv("GH_TOKEN", "must-not-reach-process")
	t.Setenv("GITHUB_TOKEN", "must-not-reach-process")
	t.Setenv("HTTPS_PROXY", "https://must-not-reach-process.invalid")
	t.Setenv("http_proxy", "http://must-not-reach-process.invalid")
	t.Setenv("npm_config_proxy", "http://must-not-reach-process.invalid")
	committed, err := (bootstrap.Service{}).Apply(context.Background(), root, plan)
	if err != nil || !committed {
		t.Fatalf("apply = %t, %v", committed, err)
	}

	failureRoot, failureProject := generated(t)
	failurePlan, err := (bootstrap.Service{}).Plan(failureRoot, failureProject)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "npm"), []byte("#!/bin/sh\nexit 23\n"), 0700); err != nil {
		t.Fatal(err)
	}
	committed, err = (bootstrap.Service{}).Apply(context.Background(), failureRoot, failurePlan)
	if committed || errorCode(err) != "bootstrap_process_failed" {
		t.Fatalf("failed process = %t, %v", committed, err)
	}
	if _, err := os.Stat(filepath.Join(failureRoot, "node_modules")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed process committed output")
	}
}

func TestDefaultRunnerCancellationStopsDescendants(t *testing.T) {
	root, project := generated(t)
	plan, err := (bootstrap.Service{}).Plan(root, project)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	script := "#!/bin/sh\nset -eu\n/bin/sleep 30 &\necho $! > '" + pidFile + "'\nwait\n"
	if err := os.WriteFile(filepath.Join(bin, "npm"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, applyErr := (bootstrap.Service{}).Apply(ctx, root, plan)
		done <- applyErr
	}()
	var pid int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		body, readErr := os.ReadFile(pidFile)
		if readErr == nil {
			pid, err = strconv.Atoi(strings.TrimSpace(string(body)))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("fixture descendant did not start")
	}
	cancel()
	select {
	case err = <-done:
		if errorCode(err) != "bootstrap_process_failed" || !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("bootstrap cancellation did not return")
	}
	for deadline = time.Now().Add(time.Second); time.Now().Before(deadline); {
		killErr := syscall.Kill(pid, 0)
		if errors.Is(killErr, syscall.ESRCH) || processStopped(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("descendant %d remained after cancellation", pid)
}

func processStopped(pid int) bool {
	body, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	fields := strings.Fields(string(body))
	return len(fields) > 2 && fields[2] == "Z"
}
