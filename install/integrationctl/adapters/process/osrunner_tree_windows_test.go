//go:build windows

package process

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

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"golang.org/x/sys/windows"
)

func TestWindowsDuplexCapabilityFailsClosedBeforeACPStart(t *testing.T) {
	err := (OS{}).DuplexCapability()
	if err == nil || !strings.Contains(err.Error(), "manual activation required") {
		t.Fatalf("Windows duplex capability = %v, want fail-closed manual activation", err)
	}
}

func TestOSRunnerTerminatesWindowsJobMembers(t *testing.T) {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("powershell.exe is unavailable")
	}
	root := t.TempDir()
	childScript := filepath.Join(root, "child.ps1")
	parentScript := filepath.Join(root, "parent.ps1")
	pidPath := filepath.Join(root, "job.pid")
	if err := os.WriteFile(childScript, []byte("Start-Sleep -Seconds 60\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	quote := func(value string) string { return strings.ReplaceAll(value, "'", "''") }
	parent := fmt.Sprintf("$child = Start-Process -FilePath '%s' -ArgumentList '-NoProfile','-File','%s' -PassThru\n\"$PID $($child.Id)\" | Set-Content -NoNewline -Path '%s'\nWait-Process -Id $child.Id\n",
		quote(powershell), quote(childScript), quote(pidPath))
	if err := os.WriteFile(parentScript, []byte(parent), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := time.Now()
	runResult := make(chan error, 1)
	go func() {
		_, runErr := (OS{}).RunWithTreeExitGrace(ctx, ports.Command{Argv: []string{powershell, "-NoProfile", "-File", parentScript}, Env: os.Environ()}, time.Second)
		runResult <- runErr
	}()
	var body []byte
	readyDeadline := time.Now().Add(10 * time.Second)
	for len(strings.Fields(string(body))) != 2 && time.Now().Before(readyDeadline) {
		body, _ = os.ReadFile(pidPath)
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	var runErr error
	select {
	case runErr = <-runResult:
	case <-time.After(3 * time.Second):
		t.Fatal("process job termination exceeded bound")
	}
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("run error = %v, want context canceled", runErr)
	}
	if elapsed := time.Since(started); elapsed > 13*time.Second {
		t.Fatalf("process job test took %s", elapsed)
	}
	if len(strings.Fields(string(body))) != 2 {
		t.Fatalf("process job pids = %q", body)
	}
	for _, field := range strings.Fields(string(body)) {
		pid, err := strconv.Atoi(field)
		if err != nil {
			t.Fatalf("parse process pid %q: %v", field, err)
		}
		if windowsProcessAlive(uint32(pid)) {
			t.Fatalf("process %d remained alive after job termination", pid)
		}
	}
}

func windowsProcessAlive(pid uint32) bool {
	const stillActive = 259
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(process)
	var exitCode uint32
	if err := windows.GetExitCodeProcess(process, &exitCode); err != nil {
		return false
	}
	if exitCode == stillActive {
		_ = windows.TerminateProcess(process, 1)
		return true
	}
	return false
}

func TestWindowsContainmentTerminationPreservesJobFailureAfterLeaderKill(t *testing.T) {
	want := errors.New("terminate job failed")
	original := terminateWindowsJob
	terminateWindowsJob = func(windows.Handle, uint32) error { return want }
	t.Cleanup(func() { terminateWindowsJob = original })
	containment := &commandContainment{attached: true, kill: func() error { return nil }, confirm: func(time.Duration) (bool, error) { return true, nil }}
	result := containment.terminate()
	if !result.leaderStopped || !errors.Is(result.err, want) {
		t.Fatalf("termination result = %+v, want stopped leader and preserved job failure", result)
	}
}

func TestWindowsContainmentTerminationReportsUnstoppedLeaderWhenBothStopsFail(t *testing.T) {
	jobErr := errors.New("terminate job failed")
	killErr := errors.New("terminate leader failed")
	original := terminateWindowsJob
	terminateWindowsJob = func(windows.Handle, uint32) error { return jobErr }
	t.Cleanup(func() { terminateWindowsJob = original })
	containment := &commandContainment{attached: true, kill: func() error { return killErr }}
	result := containment.terminate()
	if result.leaderStopped || !errors.Is(result.err, jobErr) || !errors.Is(result.err, killErr) {
		t.Fatalf("termination result = %+v, want unstopped leader and both errors", result)
	}
}

func TestWindowsAttachCleanupDoesNotKillLeaderAfterSuccessfulJobTermination(t *testing.T) {
	original := terminateWindowsJob
	terminateWindowsJob = func(windows.Handle, uint32) error { return nil }
	t.Cleanup(func() { terminateWindowsJob = original })
	killCalls := 0
	waitCalls := 0
	containment := &commandContainment{job: windows.Handle(1), attached: true, confirm: func(time.Duration) (bool, error) { return true, nil }, kill: func() error {
		t.Fatal("job termination unexpectedly used the containment's leader fallback")
		return nil
	}}
	attachErr := errors.New("attach failed")
	err := cleanupAttachFailure(attachErr, containment.terminate, func() error {
		killCalls++
		return errors.New("redundant Process.Kill failed")
	}, containment.exited, func() error { return nil }, func() error {
		waitCalls++
		return nil
	})
	if killCalls != 0 || waitCalls != 1 || !errors.Is(err, attachErr) {
		t.Fatalf("kill calls = %d wait calls = %d error = %v, want successful job termination followed by one wait", killCalls, waitCalls, err)
	}
}

func TestWindowsContainmentDoesNotTreatAcceptedJobKillAsCompletedCleanup(t *testing.T) {
	original := terminateWindowsJob
	terminateWindowsJob = func(windows.Handle, uint32) error { return nil }
	t.Cleanup(func() { terminateWindowsJob = original })
	containment := &commandContainment{attached: true, confirm: func(time.Duration) (bool, error) { return false, nil }}
	result := containment.terminate()
	if result.leaderStopped || result.err == nil || !strings.Contains(result.err.Error(), "remained active") {
		t.Fatalf("termination result = %+v, want accepted kill to remain unconfirmed failure", result)
	}
}

func TestWindowsContainmentCloseSurfacesHandleFailures(t *testing.T) {
	want := errors.New("close handle failed")
	original := closeWindowsHandle
	closeWindowsHandle = func(windows.Handle) error { return want }
	t.Cleanup(func() { closeWindowsHandle = original })
	containment := &commandContainment{job: windows.Handle(1), process: windows.Handle(2)}
	if err := containment.close(); !errors.Is(err, want) {
		t.Fatalf("close error = %v, want surfaced handle failure", err)
	}
}
