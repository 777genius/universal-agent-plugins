//go:build linux

package bootstrap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/bootstrap"
	"golang.org/x/sys/unix"
)

const fakeNPMConfigName = "fake-npm-config.json"

type fakeNPMConfig struct {
	OwnerPID         int
	Root             string
	MovedRoot        string
	PackageJSON      []byte
	Replaced         string
	Continue         string
	GrandchildChecks string
	SetsidIdentity   string
	Ready            string
}

type linuxProcessIdentity struct {
	PID       int
	ParentPID int
	StartTime uint64
	Session   int
	Zombie    bool
}

type linuxProcessHandle struct {
	Identity linuxProcessIdentity
	Parent   linuxProcessIdentity
	PIDFD    int
}

func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == "npm" {
		os.Exit(runFakeNPM())
	}
	os.Exit(m.Run())
}

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

func TestDefaultRunnerKeepsCapturedStagingAndContainsSetsidDescendant(t *testing.T) {
	root, project := generated(t)
	plan, err := (bootstrap.Service{}).Plan(root, project)
	if err != nil {
		t.Fatal(err)
	}
	wantPackage, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		t.Fatal(err)
	}

	control := t.TempDir()
	bin := filepath.Join(control, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(testExecutable, filepath.Join(bin, "npm")); err != nil {
		t.Fatal(err)
	}
	config := fakeNPMConfig{
		OwnerPID:         os.Getpid(),
		Root:             root,
		MovedRoot:        root + "-captured",
		PackageJSON:      wantPackage,
		Replaced:         filepath.Join(control, "replaced.json"),
		Continue:         filepath.Join(control, "continue"),
		GrandchildChecks: filepath.Join(control, "grandchild-checks"),
		SetsidIdentity:   filepath.Join(control, "setsid.json"),
		Ready:            filepath.Join(control, "ready.json"),
	}
	configBody, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, fakeNPMConfigName), configBody, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		_, applyErr := (bootstrap.Service{}).Apply(ctx, root, plan)
		done <- applyErr
	}()
	var manager, supervisor, setsid linuxProcessIdentity
	handles := make(map[int]linuxProcessHandle)
	var applyErr error
	applyFinished := false
	waitApply := func(timeout time.Duration) error {
		if applyFinished {
			return applyErr
		}
		applyErr = waitForApply(done, timeout)
		applyFinished = !errors.Is(applyErr, errWaitForApplyTimeout)
		return applyErr
	}
	tracked := func() []linuxProcessIdentity {
		for path, identity := range map[string]*linuxProcessIdentity{
			config.Replaced: &manager, config.SetsidIdentity: &setsid,
		} {
			if identity.PID == 0 {
				if body, err := os.ReadFile(path); err == nil {
					_ = json.Unmarshal(body, identity)
				}
			}
		}
		if supervisor.PID == 0 && manager.ParentPID > 1 {
			if handle, exists := handles[manager.PID]; exists {
				supervisor = handle.Parent
			} else if current, identityErr := readLinuxProcessIdentity(manager.PID); identityErr == nil && current.StartTime == manager.StartTime && current.ParentPID == manager.ParentPID && !current.Zombie {
				candidate, parentErr := readLinuxProcessIdentity(current.ParentPID)
				if parentErr == nil && !candidate.Zombie && candidate.ParentPID == os.Getpid() {
					supervisor = candidate
				}
			}
		}
		return []linuxProcessIdentity{setsid, manager, supervisor}
	}
	ensureHandles := func() {
		for _, identity := range tracked() {
			if identity.PID == 0 {
				continue
			}
			if _, exists := handles[identity.PID]; exists {
				continue
			}
			handle, err := captureLinuxProcessHandle(identity)
			if err == nil {
				handles[identity.PID] = handle
			}
		}
	}
	// This cleanup is independent of the behavior under test. On every failure
	// path it cancels Apply, waits for its owned reap, then uses stable pidfds to
	// stop any exact captured identity that production cleanup left behind.
	t.Cleanup(func() {
		defer closeLinuxProcessHandles(handles)
		cancel()
		ensureHandles()
		reportMissingLinuxProcessHandles(t, tracked(), handles)
		if !applyFinished {
			if err := waitApply(5 * time.Second); errors.Is(err, errWaitForApplyTimeout) {
				for _, identity := range tracked() {
					if stopErr := stopLinuxProcessHandle(handles[identity.PID], time.Second); stopErr != nil {
						t.Errorf("fallback cleanup: %v", stopErr)
					}
				}
				if err = waitApply(5 * time.Second); errors.Is(err, errWaitForApplyTimeout) {
					t.Errorf("fallback cleanup: %v", err)
				}
			}
		}
		if applyFinished {
			if err := cleanCancellationError(applyErr); err != nil {
				t.Errorf("fallback cancellation result: %v", err)
			}
		}
		for _, identity := range tracked() {
			if err := stopLinuxProcessHandle(handles[identity.PID], time.Second); err != nil {
				t.Errorf("fallback cleanup: %v", err)
			}
		}
	})

	replacedBody, err := waitForFile(config.Replaced, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(replacedBody, &manager); err != nil {
		t.Fatal(err)
	}
	if manager.ParentPID <= 1 || manager.ParentPID == config.OwnerPID {
		t.Fatalf("fake npm did not report its isolated supervisor: manager=%+v", manager)
	}
	supervisor, err = readLinuxProcessIdentity(manager.ParentPID)
	if err != nil || supervisor.Zombie {
		t.Fatalf("isolated supervisor was not alive before cancellation: identity=%+v err=%v", supervisor, err)
	}
	for _, identity := range []linuxProcessIdentity{manager, supervisor} {
		handle, captureErr := captureLinuxProcessHandle(identity)
		if captureErr != nil {
			t.Fatalf("capture stable process handle for %+v: %v", identity, captureErr)
		}
		handles[identity.PID] = handle
	}
	if current, identityErr := readLinuxProcessIdentity(manager.PID); identityErr != nil || current.StartTime != manager.StartTime || current.Zombie {
		t.Fatalf("fake npm was not alive at the replacement barrier: current=%+v err=%v", current, identityErr)
	}
	replacementPackage, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil || bytes.Equal(replacementPackage, wantPackage) {
		t.Fatalf("public package root was not replaced before manager checks: package=%q err=%v", replacementPackage, err)
	}
	if movedPackage, readErr := os.ReadFile(filepath.Join(config.MovedRoot, "package.json")); readErr != nil || !bytes.Equal(movedPackage, wantPackage) {
		t.Fatalf("captured package root after replacement: package=%q err=%v", movedPackage, readErr)
	}
	if err := atomicWrite(config.Continue, []byte("continue")); err != nil {
		t.Fatal(err)
	}

	readyBody, err := waitForFile(config.Ready, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var readyManager linuxProcessIdentity
	if err := json.Unmarshal(readyBody, &readyManager); err != nil || readyManager.PID != manager.PID || readyManager.ParentPID != supervisor.PID || readyManager.StartTime != manager.StartTime {
		t.Fatalf("manager identity changed before cancellation: ready=%+v err=%v", readyManager, err)
	}
	if current, identityErr := readLinuxProcessIdentity(supervisor.PID); identityErr != nil || current.StartTime != supervisor.StartTime || current.Zombie {
		t.Fatalf("isolated supervisor did not stay alive before cancellation: current=%+v err=%v", current, identityErr)
	}
	if current, identityErr := readLinuxProcessIdentity(manager.PID); identityErr != nil || current.StartTime != manager.StartTime || current.Zombie {
		t.Fatalf("fake npm did not stay alive across grandchild checks: current=%+v err=%v", current, identityErr)
	}
	if marker, readErr := os.ReadFile(config.GrandchildChecks); readErr != nil || string(marker) != "ok" {
		t.Fatalf("grandchild did not repeat captured-staging checks: marker=%q err=%v", marker, readErr)
	}
	setsidBody, err := os.ReadFile(config.SetsidIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(setsidBody, &setsid); err != nil || setsid.PID == 0 || setsid.Session != setsid.PID {
		t.Fatalf("immediate descendant did not establish a new session: identity=%+v err=%v", setsid, err)
	}
	setsidHandle, err := captureLinuxProcessHandle(setsid)
	if err != nil {
		t.Fatalf("capture stable setsid descendant handle: %v", err)
	}
	handles[setsid.PID] = setsidHandle
	if current, identityErr := readLinuxProcessIdentity(setsid.PID); identityErr != nil || current.StartTime != setsid.StartTime || current.Session != setsid.PID || current.Zombie {
		t.Fatalf("setsid descendant was not alive at the cancellation barrier: current=%+v err=%v", current, identityErr)
	}

	cancel()
	if err := cleanCancellationError(waitApply(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, identity := range []linuxProcessIdentity{supervisor, manager, setsid} {
		if err := waitForProcessIdentityStopped(identity, time.Second); err != nil {
			t.Error(err)
		}
	}
	if _, err := os.Lstat(filepath.Join(root, "node_modules")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("replacement root received dependency output: %v", err)
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
	var child, manager, supervisor linuxProcessIdentity
	handles := make(map[int]linuxProcessHandle)
	var applyErr error
	applyFinished := false
	waitApply := func(timeout time.Duration) error {
		if applyFinished {
			return applyErr
		}
		applyErr = waitForApply(done, timeout)
		applyFinished = !errors.Is(applyErr, errWaitForApplyTimeout)
		return applyErr
	}
	captureTree := func() {
		if child.PID == 0 {
			if body, readErr := os.ReadFile(pidFile); readErr == nil {
				if pid, parseErr := strconv.Atoi(strings.TrimSpace(string(body))); parseErr == nil {
					child, _ = readLinuxProcessIdentity(pid)
				}
			}
		}
		if child.PID != 0 {
			if _, exists := handles[child.PID]; !exists {
				if handle, captureErr := captureLinuxProcessHandle(child); captureErr == nil {
					handles[child.PID] = handle
				}
			}
		}
		if manager.PID == 0 {
			if handle, exists := handles[child.PID]; exists {
				manager = handle.Parent
			} else if child.ParentPID > 1 {
				if current, identityErr := readLinuxProcessIdentity(child.PID); identityErr == nil && current.StartTime == child.StartTime && current.ParentPID == child.ParentPID && !current.Zombie {
					manager, _ = readLinuxProcessIdentity(current.ParentPID)
				}
			}
		}
		if manager.PID != 0 {
			if _, exists := handles[manager.PID]; !exists {
				if handle, captureErr := captureLinuxProcessHandle(manager); captureErr == nil {
					handles[manager.PID] = handle
				}
			}
		}
		if supervisor.PID == 0 {
			if handle, exists := handles[manager.PID]; exists {
				supervisor = handle.Parent
			} else if manager.ParentPID > 1 {
				if current, identityErr := readLinuxProcessIdentity(manager.PID); identityErr == nil && current.StartTime == manager.StartTime && current.ParentPID == manager.ParentPID && !current.Zombie {
					candidate, parentErr := readLinuxProcessIdentity(current.ParentPID)
					if parentErr == nil && !candidate.Zombie && candidate.ParentPID == os.Getpid() {
						supervisor = candidate
					}
				}
			}
		}
		if supervisor.PID != 0 {
			if _, exists := handles[supervisor.PID]; !exists {
				if handle, captureErr := captureLinuxProcessHandle(supervisor); captureErr == nil {
					handles[supervisor.PID] = handle
				}
			}
		}
	}
	// Register the independent fallback before any assertion can terminate the
	// test. Stable handles captured at the ownership barrier prevent a reused
	// numeric PID from being signaled during emergency cleanup.
	t.Cleanup(func() {
		defer closeLinuxProcessHandles(handles)
		cancel()
		captureTree()
		reportMissingLinuxProcessHandles(t, []linuxProcessIdentity{child, manager, supervisor}, handles)
		if !applyFinished {
			if err := waitApply(5 * time.Second); errors.Is(err, errWaitForApplyTimeout) {
				for _, identity := range []linuxProcessIdentity{child, manager, supervisor} {
					if stopErr := stopLinuxProcessHandle(handles[identity.PID], time.Second); stopErr != nil {
						t.Errorf("fallback cleanup: %v", stopErr)
					}
				}
				if err = waitApply(5 * time.Second); errors.Is(err, errWaitForApplyTimeout) {
					t.Errorf("fallback cleanup: %v", err)
				}
			}
		}
		if applyFinished {
			if err := cleanCancellationError(applyErr); err != nil {
				t.Errorf("fallback cancellation result: %v", err)
			}
		}
		for _, identity := range []linuxProcessIdentity{child, manager, supervisor} {
			if err := stopLinuxProcessHandle(handles[identity.PID], time.Second); err != nil {
				t.Errorf("fallback cleanup: %v", err)
			}
		}
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		body, readErr := os.ReadFile(pidFile)
		if readErr == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(body)))
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			child, err = readLinuxProcessIdentity(pid)
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if child.PID == 0 {
		t.Fatal("fixture descendant did not start")
	}
	manager, err = readLinuxProcessIdentity(child.ParentPID)
	if err != nil || manager.Zombie {
		t.Fatalf("fixture manager identity = %+v, error = %v", manager, err)
	}
	supervisor, err = readLinuxProcessIdentity(manager.ParentPID)
	if err != nil || supervisor.Zombie || supervisor.ParentPID != os.Getpid() {
		t.Fatalf("isolated supervisor identity = %+v, error = %v", supervisor, err)
	}
	for _, identity := range []linuxProcessIdentity{child, manager, supervisor} {
		handle, captureErr := captureLinuxProcessHandle(identity)
		if captureErr != nil {
			t.Fatalf("capture stable process handle for %+v: %v", identity, captureErr)
		}
		handles[identity.PID] = handle
	}
	cancel()
	if err := cleanCancellationError(waitApply(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, identity := range []linuxProcessIdentity{supervisor, manager, child} {
		if err := waitForProcessIdentityStopped(identity, time.Second); err != nil {
			t.Error(err)
		}
	}
}

func runFakeNPM() int {
	configBody, err := os.ReadFile(filepath.Join(filepath.Dir(os.Args[0]), fakeNPMConfigName))
	if err != nil {
		return fakeNPMError(err)
	}
	var config fakeNPMConfig
	if err := json.Unmarshal(configBody, &config); err != nil {
		return fakeNPMError(err)
	}
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "--grandchild-parent":
			child := exec.Command(os.Args[0], "--grandchild-check")
			child.Stdout, child.Stderr = os.Stdout, os.Stderr
			if err := child.Run(); err != nil {
				return fakeNPMError(fmt.Errorf("run check grandchild: %w", err))
			}
			return 0
		case "--grandchild-check":
			if err := checkFakeNPMBoundary(config); err != nil {
				return fakeNPMError(fmt.Errorf("grandchild checks: %w", err))
			}
			if err := atomicWrite(config.GrandchildChecks, []byte("ok")); err != nil {
				return fakeNPMError(err)
			}
			return 0
		case "--setsid-descendant":
			identity, err := readLinuxProcessIdentity(os.Getpid())
			if err != nil {
				return fakeNPMError(err)
			}
			if identity.Session != identity.PID {
				return fakeNPMError(fmt.Errorf("setsid identity = %+v", identity))
			}
			body, err := json.Marshal(identity)
			if err != nil {
				return fakeNPMError(err)
			}
			if err := atomicWrite(config.SetsidIdentity, body); err != nil {
				return fakeNPMError(err)
			}
			for {
				time.Sleep(time.Hour)
			}
		}
	}
	if len(os.Args) != 5 || os.Args[1] != "ci" || os.Args[2] != "--ignore-scripts" || os.Args[3] != "--no-audit" || os.Args[4] != "--no-fund" {
		return fakeNPMError(fmt.Errorf("unexpected npm arguments: %q", os.Args[1:]))
	}
	// Replace the public name before opening any package or isolated manager
	// path. The barrier lets the parent test observe this exact launch window.
	if err := os.Rename(config.Root, config.MovedRoot); err != nil {
		return fakeNPMError(err)
	}
	if err := os.Mkdir(config.Root, 0700); err != nil {
		return fakeNPMError(err)
	}
	if err := os.WriteFile(filepath.Join(config.Root, "package.json"), []byte(`{"name":"replacement"}`), 0600); err != nil {
		return fakeNPMError(err)
	}
	manager, err := readLinuxProcessIdentity(os.Getpid())
	if err != nil {
		return fakeNPMError(err)
	}
	managerBody, err := json.Marshal(manager)
	if err != nil {
		return fakeNPMError(err)
	}
	if err := atomicWrite(config.Replaced, managerBody); err != nil {
		return fakeNPMError(err)
	}
	if _, err := waitForFile(config.Continue, 5*time.Second); err != nil {
		return fakeNPMError(err)
	}
	if err := checkFakeNPMBoundary(config); err != nil {
		return fakeNPMError(fmt.Errorf("manager checks: %w", err))
	}

	grandchildParent := exec.Command(os.Args[0], "--grandchild-parent")
	grandchildParent.Stdout, grandchildParent.Stderr = os.Stdout, os.Stderr
	if err := grandchildParent.Run(); err != nil {
		return fakeNPMError(err)
	}
	setsid := exec.Command(os.Args[0], "--setsid-descendant")
	setsid.Stdout, setsid.Stderr = os.Stdout, os.Stderr
	setsid.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := setsid.Start(); err != nil {
		return fakeNPMError(err)
	}
	if _, err := waitForFile(config.SetsidIdentity, 5*time.Second); err != nil {
		_ = setsid.Process.Kill()
		_ = setsid.Wait()
		return fakeNPMError(err)
	}
	if err := atomicWrite(config.Ready, managerBody); err != nil {
		_ = setsid.Process.Kill()
		_ = setsid.Wait()
		return fakeNPMError(err)
	}
	if err := setsid.Wait(); err != nil {
		return fakeNPMError(err)
	}
	return fakeNPMError(errors.New("setsid descendant exited without cancellation"))
}

func checkFakeNPMBoundary(config fakeNPMConfig) error {
	packageBody, err := os.ReadFile("package.json")
	if err != nil || !bytes.Equal(packageBody, config.PackageJSON) {
		return fmt.Errorf("read staged package through cwd: package=%q err=%w", packageBody, err)
	}
	home := os.Getenv("HOME")
	const homeSuffix = "/../home"
	if !strings.HasSuffix(home, homeSuffix) {
		return fmt.Errorf("HOME is not handle-relative: %q", home)
	}
	workHandle := strings.TrimSuffix(home, homeSuffix)
	wantHandlePrefix := fmt.Sprintf("/proc/%d/fd/", config.OwnerPID)
	// The fake npm is launched by the isolated supervisor, while the fd belongs
	// to the original Apply process. Validate the stable /proc fd spelling by
	// identity instead of depending on a particular supervisor PID depth.
	if !strings.HasPrefix(workHandle, wantHandlePrefix) {
		return fmt.Errorf("working directory is not owned by the Apply process handle: %q, want prefix %q", workHandle, wantHandlePrefix)
	}
	workInfo, err := os.Stat(workHandle)
	if err != nil {
		return err
	}
	cwdInfo, err := os.Stat(".")
	if err != nil || !os.SameFile(workInfo, cwdInfo) {
		return fmt.Errorf("cwd does not use captured work handle: %w", err)
	}
	stageInfo, err := os.Stat("..")
	if err != nil {
		return err
	}
	boundStageInfo, err := os.Stat(workHandle + "/..")
	if err != nil || !os.SameFile(stageInfo, boundStageInfo) {
		return fmt.Errorf("stage does not use captured work handle: %w", err)
	}

	paths := map[string]string{
		"HOME":                    "home",
		"USERPROFILE":             "home",
		"NPM_CONFIG_CACHE":        "cache",
		"NPM_CONFIG_USERCONFIG":   "user-npmrc",
		"NPM_CONFIG_GLOBALCONFIG": "global-npmrc",
		"NPM_CONFIG_PREFIX":       "prefix",
		"TMPDIR":                  "tmp",
		"TMP":                     "tmp",
		"TEMP":                    "tmp",
	}
	files := map[string]bool{"NPM_CONFIG_USERCONFIG": true, "NPM_CONFIG_GLOBALCONFIG": true}
	for key, leaf := range paths {
		value := os.Getenv(key)
		want := workHandle + "/../" + leaf
		if value != want {
			return fmt.Errorf("%s = %q, want %q", key, value, want)
		}
		if files[key] {
			file, openErr := os.OpenFile(value, os.O_RDWR|os.O_CREATE, 0600)
			if openErr != nil {
				return fmt.Errorf("open %s: %w", key, openErr)
			}
			if closeErr := file.Close(); closeErr != nil {
				return closeErr
			}
		} else if mkdirErr := os.MkdirAll(value, 0700); mkdirErr != nil {
			return fmt.Errorf("create %s: %w", key, mkdirErr)
		}
		resolvedInfo, statErr := os.Stat(value)
		relativeInfo, relativeErr := os.Stat(filepath.Join("..", leaf))
		if statErr != nil || relativeErr != nil || !os.SameFile(resolvedInfo, relativeInfo) {
			return fmt.Errorf("%s did not resolve into captured staging: %w", key, errors.Join(statErr, relativeErr))
		}
		resolved, evalErr := filepath.EvalSymlinks(value)
		if evalErr != nil || !pathWithin(config.MovedRoot, resolved) || pathWithin(config.Root, resolved) {
			return fmt.Errorf("%s resolved outside moved staging tree: path=%q err=%w", key, resolved, evalErr)
		}
	}
	return nil
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func fakeNPMError(err error) int {
	_, _ = fmt.Fprintln(os.Stderr, "fake npm:", err)
	return 97
}

func atomicWrite(path string, body []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".fake-npm-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func waitForFile(path string, timeout time.Duration) ([]byte, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		body, err := os.ReadFile(path)
		if err == nil {
			return body, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			return nil, fmt.Errorf("timed out waiting for %s", filepath.Base(path))
		}
	}
}

var errWaitForApplyTimeout = errors.New("timed out waiting for bootstrap Apply")

func waitForApply(done <-chan error, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return errWaitForApplyTimeout
	}
}

func cleanCancellationError(err error) error {
	if errorCode(err) != "bootstrap_process_failed" || !errors.Is(err, context.Canceled) {
		return fmt.Errorf("cancellation error = %v", err)
	}
	for _, diagnostic := range []string{
		"cleanup deadline",
		"sole reaper retains ownership until process exit",
		"ownership transferred to reaper",
		"ownership was transferred to its reaper",
		"shutdown was not proved",
		"did not empty",
		"survived cleanup",
	} {
		if strings.Contains(err.Error(), diagnostic) {
			return fmt.Errorf("cancellation reported incomplete containment or reap: %v", err)
		}
	}
	return nil
}

func readLinuxProcessIdentity(pid int) (linuxProcessIdentity, error) {
	body, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return linuxProcessIdentity{}, err
	}
	closing := strings.LastIndexByte(string(body), ')')
	if closing < 0 {
		return linuxProcessIdentity{}, errors.New("malformed Linux process stat")
	}
	fields := strings.Fields(string(body[closing+1:]))
	if len(fields) < 20 {
		return linuxProcessIdentity{}, errors.New("short Linux process stat")
	}
	parentPID, err := strconv.Atoi(fields[1])
	if err != nil {
		return linuxProcessIdentity{}, err
	}
	session, err := strconv.Atoi(fields[3])
	if err != nil {
		return linuxProcessIdentity{}, err
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return linuxProcessIdentity{}, err
	}
	return linuxProcessIdentity{PID: pid, ParentPID: parentPID, StartTime: startTime, Session: session, Zombie: fields[0] == "Z"}, nil
}

func waitForProcessIdentityStopped(want linuxProcessIdentity, timeout time.Duration) error {
	if want.PID == 0 {
		return nil
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		current, err := readLinuxProcessIdentity(want.PID)
		if errors.Is(err, os.ErrNotExist) || (err == nil && (current.StartTime != want.StartTime || current.Zombie)) {
			return nil
		}
		if err != nil {
			return err
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			return fmt.Errorf("process %d with start time %d remained after cancellation (current=%+v)", want.PID, want.StartTime, current)
		}
	}
}

func captureLinuxProcessHandle(want linuxProcessIdentity) (linuxProcessHandle, error) {
	if want.PID == 0 {
		return linuxProcessHandle{}, errors.New("cannot capture an empty process identity")
	}
	pidfd, err := unix.PidfdOpen(want.PID, 0)
	if err != nil {
		return linuxProcessHandle{}, fmt.Errorf("open stable handle for process %d: %w", want.PID, err)
	}
	current, err := readLinuxProcessIdentity(want.PID)
	if err != nil || current.StartTime != want.StartTime || current.ParentPID != want.ParentPID || current.Zombie {
		_ = unix.Close(pidfd)
		return linuxProcessHandle{}, fmt.Errorf("revalidate process %d after stable handle acquisition: current=%+v error=%v", want.PID, current, err)
	}
	parent, err := readLinuxProcessIdentity(want.ParentPID)
	if err != nil || parent.Zombie {
		_ = unix.Close(pidfd)
		return linuxProcessHandle{}, fmt.Errorf("capture parent identity for process %d: parent=%+v error=%v", want.PID, parent, err)
	}
	return linuxProcessHandle{Identity: current, Parent: parent, PIDFD: pidfd}, nil
}

func closeLinuxProcessHandles(handles map[int]linuxProcessHandle) {
	for pid, handle := range handles {
		_ = unix.Close(handle.PIDFD)
		delete(handles, pid)
	}
}

func reportMissingLinuxProcessHandles(t testing.TB, identities []linuxProcessIdentity, handles map[int]linuxProcessHandle) {
	t.Helper()
	for _, want := range identities {
		if want.PID == 0 {
			continue
		}
		if _, exists := handles[want.PID]; exists {
			continue
		}
		current, err := readLinuxProcessIdentity(want.PID)
		if err == nil && current.StartTime == want.StartTime && !current.Zombie {
			t.Errorf("fallback cleanup lacks stable handle for live process identity %+v", want)
		}
	}
}

func stopLinuxProcessHandle(handle linuxProcessHandle, timeout time.Duration) error {
	want := handle.Identity
	if want.PID == 0 {
		return nil
	}
	if !linuxPidfdAlive(handle.PIDFD) {
		return nil
	}
	current, err := readLinuxProcessIdentity(want.PID)
	if errors.Is(err, os.ErrNotExist) || (err == nil && (current.StartTime != want.StartTime || current.Zombie)) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("verify cleanup identity for process %d: %w", want.PID, err)
	}
	if current.ParentPID != want.ParentPID {
		parent, parentErr := readLinuxProcessIdentity(handle.Parent.PID)
		if parentErr == nil && parent.StartTime == handle.Parent.StartTime && !parent.Zombie {
			return fmt.Errorf("verify cleanup parentage for process %d: live parent changed from %d to %d", want.PID, want.ParentPID, current.ParentPID)
		}
	}
	if err := unix.PidfdSendSignal(handle.PIDFD, unix.SIGKILL, nil, 0); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("stop exact cleanup identity for process %d: %w", want.PID, err)
	}
	return waitForProcessIdentityStopped(want, timeout)
}

func linuxPidfdAlive(pidfd int) bool {
	err := unix.PidfdSendSignal(pidfd, 0, nil, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
