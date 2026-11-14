//go:build windows

package managedstdio

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
	"golang.org/x/sys/windows"
)

// All processes are copies of this test executable in a disposable directory.
// File handshakes avoid inherited pipes keeping Cmd.Wait alive after owner death.
func TestWindowsJobFixture(t *testing.T) {
	if os.Getenv("UAP_WINDOWS_JOB_FIXTURE") != "1" {
		return
	}
	args := os.Args
	if len(args) != 4 {
		os.Exit(90)
	}
	role, dir := args[2], args[3]
	exe := filepath.Join(dir, "fixture.exe")
	if role == "owner" {
		_, code := Dispatch(Arguments(dir, dir, dir, pathcontract.Plugin, "./fixture.exe",
			[]string{"-test.run=^TestWindowsJobFixture$", "root", dir}), os.Stderr)
		os.Exit(code)
	}
	if role == "root" || role == "child" {
		next := "child"
		if role == "child" {
			next = "grandchild"
		}
		cmd := exec.Command(exe, "-test.run=^TestWindowsJobFixture$", next, dir)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, role+".pid"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if role == "root" {
			if _, err := os.Stat(filepath.Join(dir, "exit-root")); err == nil {
				os.Exit(0)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(91) // Safety bound if the test or launcher fails before cleanup.
}

func TestWindowsJobTreeCleanup(t *testing.T) {
	for _, trigger := range []string{"root-exit", "context-cancel", "owner-termination"} {
		t.Run(trigger, func(t *testing.T) {
			dir := t.TempDir()
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(exe)
			if err != nil {
				t.Fatal(err)
			}
			fixture := filepath.Join(dir, "fixture.exe")
			if err := os.WriteFile(fixture, body, 0700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			owner := exec.CommandContext(ctx, fixture, "-test.run=^TestWindowsJobFixture$", "owner", dir)
			owner.Env = append(os.Environ(), "UAP_WINDOWS_JOB_FIXTURE=1")
			owner.Stderr = os.Stderr
			if err := owner.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- owner.Wait() }()
			t.Cleanup(func() {
				_ = owner.Process.Kill()
				select {
				case <-done:
				case <-time.After(time.Second):
				}
			})

			// Pin process handles while alive: PID reuse cannot produce a false pass.
			var handles []windows.Handle
			for _, role := range []string{"root", "child", "grandchild"} {
				h := windowsFixtureHandle(t, dir, role)
				handles = append(handles, h)
			}
			switch trigger {
			case "root-exit":
				if err := os.WriteFile(filepath.Join(dir, "exit-root"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "context-cancel":
				cancel()
			case "owner-termination":
				if err := owner.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				if (err == nil) != (trigger == "root-exit") {
					t.Fatalf("unexpected owner exit after %s: %v", trigger, err)
				}
				close(done)
			case <-time.After(5 * time.Second):
				t.Fatal("managed owner failed to exit")
			}
			for i, h := range handles {
				status, err := windows.WaitForSingleObject(h, 5000)
				if err != nil || status != windows.WAIT_OBJECT_0 {
					t.Errorf("process generation %d survived %s: status=%d err=%v", i, trigger, status, err)
				}
			}
		})
	}
}

func TestWindowsAssignFailureKillsSuspendedProcessBeforeWait(t *testing.T) {
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(dir, "fixture.exe")
	body, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture, body, 0o700); err != nil {
		t.Fatal(err)
	}
	want := errors.New("assign denied")
	original := assignProcessToJobObject
	assignProcessToJobObject = func(windows.Handle, windows.Handle) error { return want }
	t.Cleanup(func() { assignProcessToJobObject = original })

	done := make(chan error, 1)
	go func() {
		done <- replaceProcess(fixture, []string{fixture, "-test.run=^TestWindowsJobFixture$", "blocked", dir}, dir)
	}()
	select {
	case err := <-done:
		if !errors.Is(err, want) {
			t.Fatalf("want assignment error, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("assignment failure waited on a suspended process")
	}
	if _, err := os.Stat(filepath.Join(dir, "blocked.pid")); !os.IsNotExist(err) {
		t.Fatalf("suspended process executed before failed assignment: %v", err)
	}
}

func windowsFixtureHandle(t *testing.T, dir, role string) windows.Handle {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(filepath.Join(dir, role+".pid"))
		if err == nil {
			pid, parseErr := strconv.ParseUint(string(body), 10, 32)
			if parseErr == nil {
				h, openErr := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
				if openErr != nil {
					t.Fatal(openErr)
				}
				t.Cleanup(func() {
					_ = windows.TerminateProcess(h, 92)
					_, _ = windows.WaitForSingleObject(h, 5000)
					_ = windows.CloseHandle(h)
				})
				if status, err := windows.WaitForSingleObject(h, 0); err != nil || status != uint32(windows.WAIT_TIMEOUT) {
					t.Fatalf("%s was not alive before trigger: status=%d err=%v", role, status, err)
				}
				return h
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s fixture did not become ready", role)
	return 0
}
