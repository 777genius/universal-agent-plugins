//go:build linux

package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
)

// These regressions require a real Linux proc children interface and pidfds.
// Existing process runner tests own Darwin process-group and Windows Job proof.
func TestLockedNPMCancellationStopsInheritedStderrHelperBeforeRuntimeCleanup(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			requireLockedNPMProcessPrerequisites(t)
			root := installLockedNPMProcessFixture(t)
			manager, envelope, plan, dataPath, _ := lockedRuntimeFixture(t)
			manager.RunNPM = nil // Exercise the production command and supervisor.
			ctx, cancel := context.WithCancel(t.Context())
			want := context.Canceled
			if mode == "deadline" {
				cancel()
				ctx, cancel = context.WithTimeout(t.Context(), 3*time.Second)
				want = context.DeadlineExceeded
			}
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- manager.PrepareRuntime(ctx, envelope, plan, dataPath) }()
			completed := false
			t.Cleanup(func() {
				cancel()
				if !completed {
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						t.Error("fixture preparation did not finish during test cleanup")
					}
				}
			})
			leader := waitForLockedNPMPIDFD(t, root, "leader.pid")
			child := waitForLockedNPMPIDFD(t, root, "child.pid")
			if mode == "cancel" {
				cancel()
			}
			select {
			case err := <-done:
				completed = true
				if !errors.Is(err, want) {
					t.Fatalf("PrepareRuntime error = %v, want %v", err, want)
				}
				if mode == "cancel" && strings.Contains(err.Error(), "timed out") {
					t.Fatalf("cancellation mislabeled as timeout: %v", err)
				}
			case <-time.After(6 * time.Second):
				t.Fatal("npm cancellation hung on an inherited stderr descriptor")
			}
			for name, fd := range map[string]int{"npm leader": leader, "inherited-stderr helper": child} {
				if !lockedNPMPIDFDExited(t, fd) {
					t.Errorf("%s remained alive after preparation returned", name)
				}
			}
			assertNoPreparedNPMRuntime(t, dataPath)
			// The helper is armed to recreate a file in npm's cache after the
			// caller returns. It must already be dead before this trigger.
			if err := os.WriteFile(filepath.Join(root, "mutate"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			time.Sleep(50 * time.Millisecond)
			if _, err := os.Stat(filepath.Join(dataPath, "npm-cache", "late-mutation")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("helper mutated cache after cancellation: %v", err)
			}
			t.Setenv("UAP_LOCKED_NPM_FIXTURE_MODE", "success")
			if err := manager.PrepareRuntime(t.Context(), envelope, plan, dataPath); err != nil {
				t.Fatalf("real-command retry after %s: %v", mode, err)
			}
		})
	}
}

func TestRunLockedNPMRealCommandPreservesArgumentsEnvironmentAndExitStatus(t *testing.T) {
	requireLockedNPMProcessPrerequisites(t)
	installLockedNPMProcessFixture(t)
	t.Setenv("npm_config_proxy", "http://must-not-reach-fixture.invalid")
	t.Setenv("NODE_OPTIONS", "--must-not-reach-fixture")
	t.Setenv("UAP_LOCKED_NPM_EXPECT_OMIT_OPTIONAL", "1")
	for _, mode := range []string{"success", "failure"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("UAP_LOCKED_NPM_FIXTURE_MODE", mode)
			err := runLockedNPM(t.Context(), t.TempDir(), t.TempDir(), true)
			if mode == "success" {
				if err != nil {
					t.Fatalf("real npm shim command: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "exited with code 23") ||
				!strings.HasSuffix(err.Error(), "final npm diagnostic") || len(err.Error()) > 1400 {
				t.Fatalf("real npm shim failure = %v", err)
			}
		})
	}
}

func requireLockedNPMProcessPrerequisites(t *testing.T) {
	t.Helper()
	if err := (process.OS{}).OrdinaryPreflight(); err != nil {
		if errors.Is(err, process.ErrOrdinaryContainmentUnsupported) {
			if os.Getenv("RUNNER_ENVIRONMENT") == "github-hosted" {
				t.Fatalf("GitHub-hosted npm regression requires qualified Linux process containment: %v", err)
			}
			t.Skipf("real npm cancellation requires qualified Linux process containment: %v", err)
		}
		t.Fatalf("Linux process containment preflight: %v", err)
	}
	fd, err := unix.PidfdOpen(os.Getpid(), 0)
	if err != nil {
		if os.Getenv("RUNNER_ENVIRONMENT") == "github-hosted" {
			t.Fatalf("GitHub-hosted npm regression requires stable pidfd handles: %v", err)
		}
		t.Skipf("real npm cancellation requires stable pidfd handles: %v", err)
	}
	_ = unix.Close(fd)
}

func installLockedNPMProcessFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
	script := "#!/bin/sh\nexec " + quoted + " -test.run=^TestLockedNPMProcessFixture$ -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "npm"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("UAP_LOCKED_NPM_FIXTURE_ROOT", root)
	t.Setenv("UAP_LOCKED_NPM_FIXTURE_MODE", "hang")
	t.Setenv("UAP_LOCKED_NPM_EXPECT_OMIT_OPTIONAL", "0")
	return root
}

func waitForLockedNPMPIDFD(t *testing.T, root, name string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if body, err := os.ReadFile(filepath.Join(root, name)); err == nil {
			pid, err := strconv.Atoi(string(body))
			if err != nil {
				t.Fatal(err)
			}
			fd, err := unix.PidfdOpen(pid, 0)
			if err != nil {
				t.Fatalf("capture fixture process %d: %v", pid, err)
			}
			t.Cleanup(func() {
				_ = unix.PidfdSendSignal(fd, unix.SIGKILL, nil, 0)
				_ = unix.Close(fd)
			})
			return fd
		}
		if time.Now().After(deadline) {
			t.Fatalf("fake npm did not reach %s readiness", name)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func lockedNPMPIDFDExited(t *testing.T, fd int) bool {
	t.Helper()
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	if _, err := unix.Poll(fds, 0); err != nil {
		t.Fatal(err)
	}
	return fds[0].Revents&unix.POLLIN != 0
}

func TestLockedNPMProcessFixture(t *testing.T) {
	root := os.Getenv("UAP_LOCKED_NPM_FIXTURE_ROOT")
	if root == "" {
		return
	}
	if os.Getenv("UAP_LOCKED_NPM_FIXTURE_CHILD") == "1" {
		if err := writeLockedNPMFixturePID(root, "child.pid"); err != nil {
			os.Exit(81)
		}
		for {
			if _, err := os.Stat(filepath.Join(root, "mutate")); err == nil {
				_ = os.WriteFile(filepath.Join(os.Getenv("npm_config_cache"), "late-mutation"), []byte("late"), 0o600)
				os.Exit(0)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if err := validateLockedNPMProcessFixture(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(82)
	}
	_, _ = fmt.Fprint(os.Stdout, strings.Repeat("discard this npm progress output", 100000))
	switch os.Getenv("UAP_LOCKED_NPM_FIXTURE_MODE") {
	case "success":
		entrypoint := filepath.Join("node_modules", "@example", "mcp", "cli.js")
		if err := os.MkdirAll(filepath.Dir(entrypoint), 0o700); err != nil {
			os.Exit(83)
		}
		if err := os.MkdirAll(filepath.Join("node_modules", ".bin"), 0o700); err != nil {
			os.Exit(84)
		}
		if err := os.WriteFile(entrypoint, []byte("// fixture"), 0o600); err != nil {
			os.Exit(85)
		}
		os.Exit(0)
	case "failure":
		_, _ = fmt.Fprint(os.Stderr, strings.Repeat("prefix", 10000)+"final npm diagnostic")
		os.Exit(23)
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(86)
	}
	child := exec.CommandContext(context.Background(), executable, "-test.run=^TestLockedNPMProcessFixture$")
	child.Env = append(os.Environ(), "UAP_LOCKED_NPM_FIXTURE_CHILD=1")
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		os.Exit(87)
	}
	if err := writeLockedNPMFixturePID(root, "leader.pid"); err != nil {
		_ = child.Process.Kill()
		_ = child.Wait()
		os.Exit(88)
	}
	_ = child.Wait()
	os.Exit(0)
}

func writeLockedNPMFixturePID(root, name string) error {
	path := filepath.Join(root, name)
	if err := os.WriteFile(path+".tmp", []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func validateLockedNPMProcessFixture() error {
	var args []string
	for index, arg := range os.Args {
		if arg == "--" {
			args = os.Args[index+1:]
			break
		}
	}
	want := "ci --ignore-scripts --omit=dev"
	if os.Getenv("UAP_LOCKED_NPM_EXPECT_OMIT_OPTIONAL") == "1" {
		want += " --omit=optional"
	}
	want += " --no-audit --no-fund"
	if strings.Join(args, " ") != want {
		return fmt.Errorf("unexpected npm command: %v", args)
	}
	if os.Getenv("npm_config_proxy") != "" || os.Getenv("NODE_OPTIONS") != "" ||
		os.Getenv("npm_config_ignore_scripts") != "true" ||
		os.Getenv("npm_config_registry") != "https://registry.npmjs.org/" {
		return fmt.Errorf("npm command did not receive its isolated environment")
	}
	return nil
}
