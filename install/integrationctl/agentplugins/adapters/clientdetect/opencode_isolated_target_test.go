package clientdetect

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

type isolatedProbeObservation struct {
	CWD      string
	Env      []string
	Stdin    string
	Args     []string
	PID      int
	Roots    map[string]string
	Config   string
	Failures []string
}

func readIsolatedProbeObservation(t *testing.T, binary string) isolatedProbeObservation {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "observed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var observed isolatedProbeObservation
	if err := json.Unmarshal(body, &observed); err != nil {
		t.Fatal(err)
	}
	return observed
}

// Regression: an absent HOME lets native startup fall back to the OS account.
// The child must receive private writable roots and empty configuration without
// inheriting parent config, credentials, or unrelated process-launch variables.
func TestIsolatedOpenCodeTargetPrivateRootsAndStableIdentity(t *testing.T) {
	binary := nativeOpenCodeFixture(t, "2.0.21", "isolated-ok")
	if runtime.GOOS != "windows" {
		// Native cwd resolves symlink aliases (including Darwin /var -> /private/var).
		// Private HOME/XDG paths must refer to that same physical root.
		realTemp := filepath.Join(filepath.Dir(binary), "TEST-real-temp")
		if err := os.Mkdir(realTemp, 0700); err != nil {
			t.Fatal(err)
		}
		alias := filepath.Join(filepath.Dir(binary), "TEST-temp-alias")
		if err := os.Symlink(realTemp, alias); err != nil {
			t.Fatal(err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		relativeAlias, err := filepath.Rel(cwd, alias)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("TMPDIR", relativeAlias)
	}
	parentHome := filepath.Join(filepath.Dir(binary), "parent-home")
	if err := os.Mkdir(parentHome, 0700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "USERPROFILE", "APPDATA", "LOCALAPPDATA"} {
		t.Setenv(key, parentHome)
	}
	t.Setenv("AGENTPLUGINS_TEST_CREDENTIAL", "parent-secret")
	t.Setenv("OPENCODE_CONFIG_CONTENT", `{"plugin":["parent-secret"]}`)
	target := ProbeTarget{Executable: binary, Environment: []string{"PATH="}}
	var identity string
	for range 2 {
		evidence, err := ProbeIsolatedOpenCodeTarget(context.Background(), target)
		if err != nil || evidence.ProbeStatus != "ok" || evidence.Version != "2.0.21" || evidence.ExecutableIdentity == "" {
			t.Fatalf("%+v %v", evidence, err)
		}
		if identity != "" && evidence.ExecutableIdentity != identity {
			t.Fatal("private temporary roots changed pinned executable authority")
		}
		identity = evidence.ExecutableIdentity
		observed := readIsolatedProbeObservation(t, binary)
		if observed.Stdin != "" || !reflect.DeepEqual(observed.Args, []string{"--version"}) || len(observed.Failures) != 0 || strings.TrimSpace(observed.Config) != "{}" {
			t.Fatalf("child contract: %+v", observed)
		}
		env := map[string]string{}
		allowed := map[string]bool{}
		for _, key := range []string{"PATH", "HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP", "OPENCODE_CONFIG", "OPENCODE_CONFIG_CONTENT", "OPENCODE_CLI_CONFIG_CONTENT", "OPENCODE_DISABLE_PROJECT_CONFIG", "OPENCODE_DISABLE_MODELS_FETCH", "OPENCODE_DISABLE_AUTOUPDATE"} {
			allowed[key] = true
		}
		if runtime.GOOS == "windows" {
			for _, key := range []string{"USERPROFILE", "APPDATA", "LOCALAPPDATA", "SYSTEMROOT", "WINDIR"} {
				allowed[key] = true
			}
		}
		for _, entry := range observed.Env {
			key, value, _ := strings.Cut(entry, "=")
			key = strings.ToUpper(key)
			if !allowed[key] {
				t.Fatalf("inherited unrelated variable: %s", key)
			}
			env[key] = value
			if strings.Contains(value, parentHome) || strings.Contains(value, "parent-secret") {
				t.Fatalf("inherited parent authority: %s", entry)
			}
		}
		for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP"} {
			root := observed.Roots[key]
			if root == "" || !strings.HasPrefix(root, observed.CWD+string(os.PathSeparator)) {
				t.Fatalf("root %s escaped private directory: %q", key, root)
			}
		}
		if runtime.GOOS == "windows" {
			for _, key := range []string{"USERPROFILE", "APPDATA", "LOCALAPPDATA"} {
				if !strings.HasPrefix(observed.Roots[key], observed.CWD+string(os.PathSeparator)) {
					t.Fatalf("Windows root %s escaped isolation", key)
				}
			}
		}
		if env["PATH"] != "" || env["OPENCODE_CONFIG_CONTENT"] != "{}" || env["OPENCODE_CLI_CONFIG_CONTENT"] != "{}" || env["OPENCODE_DISABLE_PROJECT_CONFIG"] != "1" || env["OPENCODE_DISABLE_MODELS_FETCH"] != "1" || env["OPENCODE_DISABLE_AUTOUPDATE"] != "1" {
			t.Fatalf("unsafe startup environment: %v", env)
		}
		if !strings.HasPrefix(env["OPENCODE_CONFIG"], observed.CWD+string(os.PathSeparator)) {
			t.Fatalf("configuration escaped private directory: %s", env["OPENCODE_CONFIG"])
		}
		if _, err := os.Stat(observed.CWD); !os.IsNotExist(err) {
			t.Fatalf("private roots survived successful probe: %v", err)
		}
	}
	entries, err := os.ReadDir(parentHome)
	if err != nil || len(entries) != 0 {
		t.Fatalf("probe wrote parent roots: %v %v", entries, err)
	}
}

// Regression: unsuccessful startup and bounded-output rejection must remove all
// private state, while canceled probes must reap the child before returning.
func TestIsolatedOpenCodeTargetFailureCleanupAndReaping(t *testing.T) {
	for _, tc := range []struct{ mode, status string }{{"failed", "failed"}, {"limit", "output_limit"}, {"slow", "timed_out"}} {
		t.Run(tc.mode, func(t *testing.T) {
			binary := nativeOpenCodeFixture(t, "2.0.21", "isolated-"+tc.mode)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.mode == "slow" {
				// Cancel only once the native child has started and written state.
				go func() {
					for ctx.Err() == nil {
						if _, err := os.Stat(filepath.Join(filepath.Dir(binary), "observed.json")); err == nil {
							cancel()
							return
						}
						time.Sleep(10 * time.Millisecond)
					}
				}()
			}
			evidence, err := ProbeIsolatedOpenCodeTarget(ctx, ProbeTarget{Executable: binary, Environment: []string{"PATH="}, Timeout: 5 * time.Second})
			if err == nil || evidence.ProbeStatus != tc.status || evidence.Version != "" || strings.Contains(err.Error(), binary) {
				t.Fatalf("%+v %v", evidence, err)
			}
			if tc.mode == "slow" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost caller cancellation: %v", err)
			}
			observed := readIsolatedProbeObservation(t, binary)
			if _, err := os.Stat(observed.CWD); !os.IsNotExist(err) {
				t.Fatalf("private roots survived failed probe: %v", err)
			}
			if tc.mode == "slow" && runtime.GOOS != "windows" {
				process, err := os.FindProcess(observed.PID)
				if err != nil {
					t.Fatal(err)
				}
				defer process.Release()
				if err := process.Signal(syscall.Signal(0)); err == nil {
					t.Fatal("canceled probe returned before child was reaped")
				}
			}
		})
	}
}

// Regression: isolation cannot admit caller-supplied HOME/config overrides or
// bypass native verification, and a retarget during startup invalidates evidence.
func TestIsolatedOpenCodeTargetAuthorityGuards(t *testing.T) {
	binary := nativeOpenCodeFixture(t, "2.0.21", "isolated-ok")
	for _, env := range [][]string{{"PATH=", "HOME=override"}, {"PATH=", "OPENCODE_CONFIG_CONTENT={}"}, {"PATH=", "TMPDIR=override"}} {
		_, err := ProbeIsolatedOpenCodeTarget(context.Background(), ProbeTarget{Executable: binary, Environment: env})
		if !errors.Is(err, ErrInvalidProbeTarget) {
			t.Fatalf("accepted arbitrary environment: %v %v", env, err)
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	wrapper := filepath.Join(filepath.Dir(binary), "wrapper")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\necho 2.0.21\n"), 0700); err != nil {
		t.Fatal(err)
	}
	evidence, err := ProbeIsolatedOpenCodeTarget(context.Background(), ProbeTarget{Executable: wrapper, Environment: []string{"PATH="}})
	if !errors.Is(err, ErrUnverifiedProbeTarget) || evidence.Reason != "host_target_unverified" {
		t.Fatalf("wrapper accepted: %+v %v", evidence, err)
	}
	binary = nativeOpenCodeFixture(t, "2.0.21", "isolated-retarget")
	body, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(binary), "replacement"), body, 0700); err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(filepath.Dir(binary), "selected")
	if err := os.Symlink(binary, selected); err != nil {
		t.Fatal(err)
	}
	evidence, err = ProbeIsolatedOpenCodeTarget(context.Background(), ProbeTarget{Executable: selected, Environment: []string{"PATH="}})
	if !errors.Is(err, ErrProbeTargetChanged) || evidence.Version != "" || evidence.Reason != "host_target_changed" {
		t.Fatalf("retarget authorized: %+v %v", evidence, err)
	}
	observed := readIsolatedProbeObservation(t, binary)
	if _, err := os.Stat(observed.CWD); !os.IsNotExist(err) {
		t.Fatalf("private roots survived identity rejection: %v", err)
	}
}

// Regression: a cold native startup taking longer than the legacy ten-second
// budget succeeds in isolation, but no requested timeout can exceed thirty.
func TestIsolatedOpenCodeTargetDefaultAndMaximumTimeout(t *testing.T) {
	binary := nativeOpenCodeFixture(t, "2.0.21", "isolated-cold")
	evidence, err := ProbeIsolatedOpenCodeTarget(context.Background(), ProbeTarget{Executable: binary, Environment: []string{"PATH="}})
	if err != nil || evidence.ProbeStatus != "ok" {
		t.Fatalf("isolated default too short: %+v %v", evidence, err)
	}
	binary = nativeOpenCodeFixture(t, "2.0.21", "isolated-slow")
	start := time.Now()
	evidence, err = ProbeIsolatedOpenCodeTarget(context.Background(), ProbeTarget{Executable: binary, Environment: []string{"PATH="}, Timeout: time.Minute})
	if !errors.Is(err, context.DeadlineExceeded) || evidence.ProbeStatus != "timed_out" || time.Since(start) < 28*time.Second || time.Since(start) > 35*time.Second {
		t.Fatalf("isolated maximum changed: %+v %v elapsed %s", evidence, err, time.Since(start))
	}
	observed := readIsolatedProbeObservation(t, binary)
	if _, err := os.Stat(observed.CWD); !os.IsNotExist(err) {
		t.Fatalf("private roots survived timeout: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start = time.Now()
	evidence, err = ProbeIsolatedOpenCodeTarget(ctx, ProbeTarget{Executable: binary, Environment: []string{"PATH="}})
	if !errors.Is(err, context.DeadlineExceeded) || evidence.ProbeStatus != "timed_out" || time.Since(start) > 2*time.Second {
		t.Fatalf("caller deadline ignored: %+v %v", evidence, err)
	}
}
