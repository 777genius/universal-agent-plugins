//go:build linux

package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"golang.org/x/sys/unix"
)

func TestOSRunnerPlannedDuplexShutdownContainsImmediateSetsidDescendant(t *testing.T) {
	requireDuplexCapability(t)
	if os.Getenv("AGENTPLUGINS_DUPLEX_CGROUP_ESCAPE_CHILD") == "1" {
		_, _ = syscall.Setsid()
		for {
			time.Sleep(time.Hour)
		}
	}
	if pidPath := os.Getenv("AGENTPLUGINS_DUPLEX_CGROUP_ESCAPE_PID"); pidPath != "" {
		child := exec.Command(os.Args[0], "-test.run=TestOSRunnerPlannedDuplexShutdownContainsImmediateSetsidDescendant")
		child.Env = append(os.Environ(), "AGENTPLUGINS_DUPLEX_CGROUP_ESCAPE_CHILD=1")
		if err := child.Start(); err != nil {
			os.Exit(81)
		}
		if err := writeTestProcessIdentity(pidPath, child.Process.Pid); err != nil {
			os.Exit(82)
		}
		fmt.Fprintln(os.Stdout, "ready")
		for {
			time.Sleep(time.Hour)
		}
	}
	pidPath := t.TempDir() + "/escape.pid"
	environment := append(os.Environ(), "AGENTPLUGINS_DUPLEX_CGROUP_ESCAPE_PID="+pidPath)
	err := (OS{}).RunDuplexWithPlannedShutdown(context.Background(), ports.Command{
		Argv: []string{os.Args[0], "-test.run=TestOSRunnerPlannedDuplexShutdownContainsImmediateSetsidDescendant"}, Env: environment,
	}, func(_ io.Writer, stdout io.Reader) error {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil || line != "ready\n" {
			return fmt.Errorf("escape helper readiness = %q: %w", line, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, startTime, err := parseTestProcessIdentity(body)
	if err != nil {
		t.Fatal(err)
	}
	// cgroup.kill and populated=0 prove that no task remains in the scope. The
	// namespace's PID 1 may retain the dead helper as a zombie briefly (or
	// indefinitely), so kill(pid, 0) is not a liveness test here.
	deadline := time.Now().Add(time.Second)
	for processIdentityStillRunning(pid, startTime) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if processIdentityStillRunning(pid, startTime) {
		t.Fatalf("immediate setsid descendant %d survived cgroup cleanup", pid)
	}
}

func TestOSRunnerCancellationContainsImmediateSetsidDescendant(t *testing.T) {
	requireDuplexCapability(t)
	if os.Getenv("AGENTPLUGINS_RUN_CANCEL_ESCAPE_CHILD") == "1" {
		_, _ = syscall.Setsid()
		for {
			time.Sleep(time.Hour)
		}
	}
	if pidPath := os.Getenv("AGENTPLUGINS_RUN_CANCEL_ESCAPE_PID"); pidPath != "" {
		child := exec.Command(os.Args[0], "-test.run=TestOSRunnerCancellationContainsImmediateSetsidDescendant")
		child.Env = append(os.Environ(), "AGENTPLUGINS_RUN_CANCEL_ESCAPE_CHILD=1")
		if err := child.Start(); err != nil {
			os.Exit(91)
		}
		if err := writeTestProcessIdentity(pidPath, child.Process.Pid); err != nil {
			os.Exit(92)
		}
		for {
			time.Sleep(time.Hour)
		}
	}

	pidPath := t.TempDir() + "/escape.pid"
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := (OS{}).Run(ctx, ports.Command{
			Argv: []string{os.Args[0], "-test.run=TestOSRunnerCancellationContainsImmediateSetsidDescendant"},
			Env:  append(os.Environ(), "AGENTPLUGINS_RUN_CANCEL_ESCAPE_PID="+pidPath),
		})
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(pidPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("escape descendant was not started")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run cancellation error = %v, want context cancellation", err)
	}
	body, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, startTime, err := parseTestProcessIdentity(body)
	if err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(time.Second)
	for processIdentityStillRunning(pid, startTime) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if processIdentityStillRunning(pid, startTime) {
		t.Fatalf("setsid descendant %d survived Run cancellation", pid)
	}
}

func TestDuplexCapabilityRejectsNonCgroupDelegationBeforeStart(t *testing.T) {
	_, _, err := createDelegatedCgroupAt(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "cgroup") {
		t.Fatalf("capability primitive error = %v, want honest pre-start unsupported result", err)
	}
}

func TestOrdinaryCommandContainmentDoesNotRequireDelegatedCgroup(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "printf ordinary")
	var before int
	if err := unix.Prctl(unix.PR_GET_CHILD_SUBREAPER, uintptr(unsafe.Pointer(&before)), 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	containment, err := newOrdinaryCommandContainment(cmd, time.Second)
	if err != nil {
		t.Fatalf("ordinary containment unexpectedly required delegated cgroup: %v", err)
	}
	defer func() {
		if err := containment.close(); err != nil {
			t.Errorf("close ordinary containment: %v", err)
		}
	}()
	if containment.cgroupPath != "" || containment.cgroupDir != nil {
		t.Fatalf("ordinary containment allocated delegated cgroup: path=%q dir=%v", containment.cgroupPath, containment.cgroupDir)
	}
	if containment.descendants == nil || containment.stop == nil || containment.done == nil {
		t.Fatal("ordinary pidfd descendant tracker fields were not initialized")
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid || cmd.SysProcAttr.UseCgroupFD {
		t.Fatalf("ordinary process attributes = %#v, want process group without cgroup requirement", cmd.SysProcAttr)
	}
	var after int
	if err := unix.Prctl(unix.PR_GET_CHILD_SUBREAPER, uintptr(unsafe.Pointer(&after)), 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("ordinary containment changed process-wide subreaper state from %d to %d", before, after)
	}
	if len(cmd.ExtraFiles) != 3 {
		t.Fatalf("ordinary supervisor descriptors = %d, want request/status boundary", len(cmd.ExtraFiles))
	}
}

func TestOrdinaryRunWorksWhenDelegatedCgroupIsUnavailable(t *testing.T) {
	root := t.TempDir()
	if _, _, err := createDelegatedCgroupAt(root); err == nil {
		t.Fatal("test fixture unexpectedly supplied a cgroup v2 delegation")
	}

	result, err := (OS{}).Run(context.Background(), ports.Command{Argv: []string{"/bin/sh", "-c", "printf ordinary-without-cgroup"}})
	if err != nil {
		t.Fatalf("ordinary Run failed without delegated cgroup: %v", err)
	}
	if result.ExitCode != 0 || string(result.Stdout) != "ordinary-without-cgroup" {
		t.Fatalf("ordinary Run result = %#v", result)
	}
}

func TestOrdinarySupervisorDescriptorsAreClosedBeforeCommandExec(t *testing.T) {
	result, err := (OS{}).Run(context.Background(), ports.Command{
		Argv: []string{"/bin/sh", "-c", `if printf '{"Forced":false}\n' 2>/dev/null >&5; then exit 97; fi; printf supervisor-fds-closed`},
	})
	if err != nil || result.ExitCode != 0 || string(result.Stdout) != "supervisor-fds-closed" {
		t.Fatalf("supervised descriptor isolation result = %+v error = %v", result, err)
	}
}

func TestSupervisorStatusRequiresOneAuthoritativeRecordAndEOF(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(writer, "{\"Forced\":false}\n{\"Forced\":true}\n"); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	containment := &commandContainment{supervisorStatus: reader}
	if err := containment.readSupervisorResult(); err == nil || !strings.Contains(err.Error(), "trailing status data") {
		t.Fatalf("forged multi-record supervisor status error = %v", err)
	}
}

func TestExitedLeaderCannotAuthorizeNumericProcessGroupSignal(t *testing.T) {
	called := false
	err := signalOwnedLinuxProcessGroup(1234, true, func(int, syscall.Signal) error {
		called = true
		return nil
	})
	if err != nil || called {
		t.Fatalf("signal after leader exit = (%v, called %v), want no numeric PGID signal", err, called)
	}
}

func TestSupervisorEmptyBoundaryRequiresQuiescentErrorFreePasses(t *testing.T) {
	boundary := linuxQuiescentBoundary{required: 3}
	if boundary.observe(true, nil) || boundary.observe(true, nil) {
		t.Fatal("non-atomic empty snapshots prematurely proved the boundary empty")
	}
	if boundary.observe(false, nil) {
		t.Fatal("rapid adopted descendant was ignored")
	}
	if boundary.observe(true, nil) || boundary.observe(true, errors.New("proc scan uncertain")) {
		t.Fatal("scan uncertainty did not reset the empty proof")
	}
	if boundary.observe(true, nil) || boundary.observe(true, nil) || !boundary.observe(true, nil) {
		t.Fatal("three stable error-free snapshots did not prove quiescence")
	}
}

func TestOSRunnerRapidSuccessfulCommandsPreserveNaturalExit(t *testing.T) {
	// /bin/true commonly becomes waitable before the runner can acquire its
	// post-Start pidfd. Exercise that attachment boundary enough times to prove
	// rapid natural exits remain ordinary successful command results.
	const attempts = 250
	for attempt := 0; attempt < attempts; attempt++ {
		result, err := (OS{}).Run(context.Background(), ports.Command{Argv: []string{"/bin/true"}})
		if err != nil || result.ExitCode != 0 {
			t.Fatalf("rapid command attempt %d result = %#v error = %v", attempt, result, err)
		}
	}
}

func TestOSRunnerRapidShellCommandsWithShortLivedChildrenPreserveNaturalExit(t *testing.T) {
	root := t.TempDir()
	script := root + "/client"
	body := `#!/bin/sh
set -eu
mkdir -p "$(dirname "$TEST_STATE")" "$(dirname "$TEST_LOG")" "$(dirname "$TEST_MARKETPLACES")"
touch "$TEST_STATE" "$TEST_LOG" "$TEST_MARKETPLACES"
printf '%s\n' "$*" >> "$TEST_LOG"
printf '[]\n'
`
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	environment := append(os.Environ(),
		"TEST_STATE="+root+"/state/value",
		"TEST_LOG="+root+"/log/value",
		"TEST_MARKETPLACES="+root+"/marketplaces/value",
	)
	const attempts = 500
	for attempt := 0; attempt < attempts; attempt++ {
		result, err := (OS{}).Run(context.Background(), ports.Command{
			Argv: []string{script, "plugin", "list", "--json"},
			Env:  environment,
			Dir:  root,
		})
		if err != nil || result.ExitCode != 0 || string(result.Stdout) != "[]\n" {
			t.Fatalf("rapid shell attempt %d result = %#v error = %v", attempt, result, err)
		}
	}
}

func TestDuplexCapabilityProbeUnprovedShutdownWaitIsHardBoundAndRetainsSoleReaper(t *testing.T) {
	waitRelease := make(chan struct{})
	waitStarted := make(chan struct{})
	waitReturned := make(chan struct{})
	defer func() {
		close(waitRelease)
		select {
		case <-waitReturned:
		case <-time.After(time.Second):
			t.Fatal("probe's owned Wait did not finish after simulated exit")
		}
	}()
	started := time.Now()
	terminationErr := errors.New("containment and fallback shutdown failed")
	err := finishDuplexContainmentProbe(terminationResult{err: terminationErr}, func() error { return nil }, func() error {
		close(waitStarted)
		<-waitRelease
		close(waitReturned)
		return nil
	}, time.Millisecond)
	<-waitStarted
	if err == nil || !errors.Is(err, terminationErr) || !strings.Contains(err.Error(), "shutdown was not proved") || !strings.Contains(err.Error(), "exceeded cleanup deadline") || !strings.Contains(err.Error(), "sole reaper retains ownership until process exit") {
		t.Fatalf("unproved stuck probe Wait error = %v, want bounded fail-closed owned-reaper failure", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("stuck probe Wait returned too slowly after %s", elapsed)
	}
}

func TestDuplexCapabilityProbeCannotSucceedWithoutProvenLeaderStop(t *testing.T) {
	waitCalls := 0
	err := finishDuplexContainmentProbe(terminationResult{}, func() error { return nil }, func() error {
		waitCalls++
		return nil
	}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "shutdown was not proved") || waitCalls != 1 {
		t.Fatalf("unproved probe cleanup error = %v wait calls = %d, want fail-closed sole reap", err, waitCalls)
	}
}

func TestLinuxTrackerTimeoutTransfersHandleOwnershipUntilJoin(t *testing.T) {
	done := make(chan struct{})
	containment := &commandContainment{
		cmd: &exec.Cmd{Process: &os.Process{Pid: os.Getpid()}}, descendants: make(map[int]linuxTrackedProcess),
		stop: make(chan struct{}), done: done, trackerStarted: true,
	}
	err := containment.close()
	if err == nil || !strings.Contains(err.Error(), "ownership transferred") {
		t.Fatalf("stalled tracker close error = %v", err)
	}
	containment.mu.Lock()
	owned := containment.descendants != nil
	containment.mu.Unlock()
	if !owned {
		t.Fatal("tracker-owned map was released before the tracker joined")
	}
	close(done)
	deadline := time.Now().Add(time.Second)
	for {
		containment.mu.Lock()
		released := containment.descendants == nil
		containment.mu.Unlock()
		if released {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("owned tracker reaper did not release handles after join")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestOSRunnerCleansSameProcessGroupMemberAfterLeaderNaturalExit(t *testing.T) {
	requireDuplexCapability(t)
	if os.Getenv("AGENTPLUGINS_PROCESS_RUN_DESCENDANT") == "1" {
		for {
			time.Sleep(time.Hour)
		}
	}
	if pidPath := os.Getenv("AGENTPLUGINS_PROCESS_RUN_LEADER_PID_PATH"); pidPath != "" {
		child := exec.Command(os.Args[0], "-test.run=TestOSRunnerCleansSameProcessGroupMemberAfterLeaderNaturalExit")
		child.Env = append(os.Environ(), "AGENTPLUGINS_PROCESS_RUN_DESCENDANT=1")
		devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			os.Exit(61)
		}
		child.Stdin = devNull
		child.Stdout = devNull
		child.Stderr = devNull
		if err := child.Start(); err != nil {
			os.Exit(62)
		}
		if err := writeTestProcessIdentity(pidPath, child.Process.Pid); err != nil {
			os.Exit(63)
		}
		os.Exit(0)
	}

	pidPath := t.TempDir() + "/descendant.pid"
	environment := append(os.Environ(), "AGENTPLUGINS_PROCESS_RUN_LEADER_PID_PATH="+pidPath)
	_, err := (OS{}).RunWithTreeExitGrace(context.Background(), ports.Command{
		Argv: []string{os.Args[0], "-test.run=TestOSRunnerCleansSameProcessGroupMemberAfterLeaderNaturalExit"},
		Env:  environment,
	}, time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "live descendants that required forced cleanup") {
		t.Fatalf("clean leader exit error = %v, want forced-descendant causality", err)
	}
	body, readErr := os.ReadFile(pidPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	pid, startTime, parseErr := parseTestProcessIdentity(body)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	deadline := time.Now().Add(time.Second)
	for processIdentityStillRunning(pid, startTime) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processIdentityStillRunning(pid, startTime) {
		t.Fatalf("same-process-group member %d survived leader cleanup", pid)
	}
}

func TestOSRunnerDescendantGraceAcceptsOnlyNaturalHelperExit(t *testing.T) {
	requireDuplexCapability(t)
	if os.Getenv("AGENTPLUGINS_PROCESS_GRACE_DESCENDANT") == "1" {
		time.Sleep(150 * time.Millisecond)
		os.Exit(0)
	}
	if os.Getenv("AGENTPLUGINS_PROCESS_GRACE_LEADER") == "1" {
		child := exec.Command(os.Args[0], "-test.run=TestOSRunnerDescendantGraceAcceptsOnlyNaturalHelperExit")
		child.Env = append(os.Environ(), "AGENTPLUGINS_PROCESS_GRACE_DESCENDANT=1")
		devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			os.Exit(91)
		}
		child.Stdin, child.Stdout, child.Stderr = devNull, devNull, devNull
		if err := child.Start(); err != nil {
			os.Exit(92)
		}
		os.Exit(0)
	}

	_, err := (OS{}).RunWithDescendantExitGrace(context.Background(), ports.Command{
		Argv: []string{os.Args[0], "-test.run=TestOSRunnerDescendantGraceAcceptsOnlyNaturalHelperExit"},
		Env:  append(os.Environ(), "AGENTPLUGINS_PROCESS_GRACE_LEADER=1"),
	}, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("naturally exiting descendant within grace failed: %v", err)
	}
}

func TestOSRunnerDescendantGraceStillRejectsForcedCleanup(t *testing.T) {
	requireDuplexCapability(t)
	if os.Getenv("AGENTPLUGINS_PROCESS_GRACE_LONG_DESCENDANT") == "1" {
		for {
			time.Sleep(time.Hour)
		}
	}
	if os.Getenv("AGENTPLUGINS_PROCESS_GRACE_LONG_LEADER") == "1" {
		child := exec.Command(os.Args[0], "-test.run=TestOSRunnerDescendantGraceStillRejectsForcedCleanup")
		child.Env = append(os.Environ(), "AGENTPLUGINS_PROCESS_GRACE_LONG_DESCENDANT=1")
		devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			os.Exit(93)
		}
		child.Stdin, child.Stdout, child.Stderr = devNull, devNull, devNull
		if err := child.Start(); err != nil {
			os.Exit(94)
		}
		os.Exit(0)
	}

	_, err := (OS{}).RunWithDescendantExitGrace(context.Background(), ports.Command{
		Argv: []string{os.Args[0], "-test.run=TestOSRunnerDescendantGraceStillRejectsForcedCleanup"},
		Env:  append(os.Environ(), "AGENTPLUGINS_PROCESS_GRACE_LONG_LEADER=1"),
	}, 25*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "live descendants that required forced cleanup") {
		t.Fatalf("long-lived descendant error = %v, want forced-cleanup failure", err)
	}
}

func TestOSRunnerPreservesDiagnosticAndStatusWhileCleaningNaturalExitGroup(t *testing.T) {
	requireDuplexCapability(t)
	if os.Getenv("AGENTPLUGINS_PROCESS_DIAGNOSTIC_DESCENDANT") == "1" {
		for {
			time.Sleep(time.Hour)
		}
	}
	if pidPath := os.Getenv("AGENTPLUGINS_PROCESS_DIAGNOSTIC_LEADER_PID_PATH"); pidPath != "" {
		child := exec.Command(os.Args[0], "-test.run=TestOSRunnerPreservesDiagnosticAndStatusWhileCleaningNaturalExitGroup")
		child.Env = append(os.Environ(), "AGENTPLUGINS_PROCESS_DIAGNOSTIC_DESCENDANT=1")
		devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			os.Exit(71)
		}
		child.Stdin = devNull
		child.Stdout = devNull
		child.Stderr = devNull
		if err := child.Start(); err != nil {
			os.Exit(72)
		}
		if err := writeTestProcessIdentity(pidPath, child.Process.Pid); err != nil {
			os.Exit(73)
		}
		fmt.Fprint(os.Stderr, "fatal bounded-process diagnostic")
		os.Exit(47)
	}

	pidPath := t.TempDir() + "/descendant.pid"
	environment := append(os.Environ(), "AGENTPLUGINS_PROCESS_DIAGNOSTIC_LEADER_PID_PATH="+pidPath)
	_, err := (OS{}).RunWithTreeExitGrace(context.Background(), ports.Command{
		Argv: []string{os.Args[0], "-test.run=TestOSRunnerPreservesDiagnosticAndStatusWhileCleaningNaturalExitGroup"},
		Env:  environment,
	}, time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "live descendants that required forced cleanup") || !strings.Contains(err.Error(), "fatal bounded-process diagnostic") {
		t.Fatalf("error = %v, want forced-descendant causality and bounded diagnostic", err)
	}
	body, readErr := os.ReadFile(pidPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	pid, startTime, parseErr := parseTestProcessIdentity(body)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	deadline := time.Now().Add(time.Second)
	for processIdentityStillRunning(pid, startTime) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processIdentityStillRunning(pid, startTime) {
		t.Fatalf("same-process-group member %d survived diagnostic leader cleanup", pid)
	}
}

func TestOSRunnerTreeGraceCleansImmediateDescendantThatCreatesNewSession(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_PROCESS_SETSID_DESCENDANT") == "1" {
		for {
			time.Sleep(time.Hour)
		}
	}
	if pidPath := os.Getenv("AGENTPLUGINS_PROCESS_SETSID_LEADER_PID_PATH"); pidPath != "" {
		child := exec.Command(os.Args[0], "-test.run=TestOSRunnerTreeGraceCleansImmediateDescendantThatCreatesNewSession")
		child.Env = append(os.Environ(), "AGENTPLUGINS_PROCESS_SETSID_DESCENDANT=1")
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			os.Exit(81)
		}
		child.Stdin, child.Stdout, child.Stderr = devNull, devNull, devNull
		if err := child.Start(); err != nil {
			os.Exit(82)
		}
		if err := writeTestProcessIdentity(pidPath, child.Process.Pid); err != nil {
			os.Exit(83)
		}
		os.Exit(0)
	}

	pidPath := t.TempDir() + "/setsid-descendant.pid"
	_, err := (OS{}).RunWithTreeExitGrace(context.Background(), ports.Command{
		Argv: []string{os.Args[0], "-test.run=TestOSRunnerTreeGraceCleansImmediateDescendantThatCreatesNewSession"},
		Env:  append(os.Environ(), "AGENTPLUGINS_PROCESS_SETSID_LEADER_PID_PATH="+pidPath),
	}, time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "live descendants that required forced cleanup") {
		t.Fatalf("clean leader exit with tracked setsid descendant error = %v, want forced-descendant causality", err)
	}
	body, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, startTime, err := parseTestProcessIdentity(body)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for processIdentityStillRunning(pid, startTime) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processIdentityStillRunning(pid, startTime) {
		t.Fatalf("tracked new-session descendant %d survived leader cleanup", pid)
	}
}

func TestCgroupMemberInspectionFailurePreservesForcedCleanupCausality(t *testing.T) {
	want := errors.New("cgroup.procs unreadable")
	forced, err := cgroupForcedMemberCausality(1234, nil, want)
	if !forced || !errors.Is(err, want) {
		t.Fatalf("forced = %v error = %v, want uncertain forced cleanup causality", forced, err)
	}
}

func TestFailedCgroupKillCannotClaimTreeShutdown(t *testing.T) {
	originalRead, originalWrite := readCgroupProcs, writeCgroupKill
	defer func() { readCgroupProcs, writeCgroupKill = originalRead, originalWrite }()
	readCgroupProcs = func(string) ([]byte, error) { return []byte("1234\n5678\n"), nil }
	killFailure := errors.New("cgroup.kill failed")
	writeCgroupKill = func(string, []byte, os.FileMode) error { return killFailure }
	containment := &commandContainment{
		cmd:          &exec.Cmd{Process: &os.Process{Pid: 1234}},
		cgroupPath:   "/deterministic-test-cgroup",
		observeExit:  func() (bool, error) { return true, nil },
		observeEmpty: func() (bool, error) { return false, nil },
		inspectLive:  func(int) (bool, error) { return true, nil },
	}
	result := containment.terminateCgroupWithin(time.Millisecond)
	if !result.leaderStopped || result.leaderTerminationInitiated || !result.forcedMembers || !errors.Is(result.err, killFailure) || !strings.Contains(result.err.Error(), "did not empty") {
		t.Fatalf("termination = %+v, want observed leader plus failed tree shutdown", result)
	}
}

func TestCgroupTerminationCausalityRequiresLiveLeaderAtSuccessfulKillBoundary(t *testing.T) {
	originalRead, originalWrite := readCgroupProcs, writeCgroupKill
	defer func() { readCgroupProcs, writeCgroupKill = originalRead, originalWrite }()
	readCgroupProcs = func(string) ([]byte, error) { return []byte("1234\n"), nil }
	writeCgroupKill = func(string, []byte, os.FileMode) error { return nil }

	for _, test := range []struct {
		name       string
		leaderLive bool
		initiated  bool
	}{
		{name: "live leader", leaderLive: true, initiated: true},
		{name: "naturally stopped leader", leaderLive: false, initiated: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			containment := &commandContainment{
				cmd:          &exec.Cmd{Process: &os.Process{Pid: 1234}},
				cgroupPath:   "/deterministic-test-cgroup",
				observeExit:  func() (bool, error) { return true, nil },
				observeEmpty: func() (bool, error) { return true, nil },
				inspectLive: func(pid int) (bool, error) {
					return pid == 1234 && test.leaderLive, nil
				},
				prepareLeaderTermination: func(int, time.Duration) (bool, error) {
					return test.leaderLive, nil
				},
			}
			result := containment.terminateCgroupWithin(time.Millisecond)
			if !result.leaderStopped || result.leaderTerminationInitiated != test.initiated || result.err != nil {
				t.Fatalf("termination = %+v, want initiated = %v", result, test.initiated)
			}
		})
	}
}

func TestCgroupCausalityExcludesNaturallyExitedZombieMember(t *testing.T) {
	inspect := func(pid int) (bool, error) {
		if pid == 5678 {
			return false, nil
		}
		return true, nil
	}
	forced, err := cgroupForcedMemberCausalityWithInspect(1234, []byte("1234\n5678\n"), nil, inspect)
	if err != nil || forced {
		t.Fatalf("forced = %v error = %v, want naturally exited member excluded from cleanup causality", forced, err)
	}
}

func TestSuccessfulLeaderKillRequestRequiresObservedShutdown(t *testing.T) {
	result := stopLeaderAfterContainmentFailure(terminationResult{}, func() error { return nil }, func() (bool, error) { return false, errors.New("observation failed") })
	if result.leaderStopped || result.err == nil || !strings.Contains(result.err.Error(), "confirm process leader stopped") {
		t.Fatalf("termination = %+v, want unconfirmed leader shutdown failure", result)
	}
}

func TestLinuxTerminationDoesNotClassifyNaturalZombieDescendantAsForced(t *testing.T) {
	child := exec.Command("/bin/sh", "-c", "exit 0")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	reaped := false
	defer func() {
		if !reaped {
			_ = child.Wait()
		}
	}()
	identity, err := readLinuxProcessIdentity(child.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	pidfd, err := unix.PidfdOpen(child.Process.Pid, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(pidfd)
	deadline := time.Now().Add(time.Second)
	for {
		current, readErr := readLinuxProcessIdentity(child.Process.Pid)
		if readErr == nil && current.startTime == identity.startTime && current.zombie {
			break
		}
		if readErr != nil {
			t.Fatalf("observe unreaped child: %v", readErr)
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not become an observable zombie")
		}
		time.Sleep(time.Millisecond)
	}

	containment := &commandContainment{
		cmd:         &exec.Cmd{Process: &os.Process{Pid: 1 << 30}},
		descendants: map[int]linuxTrackedProcess{child.Process.Pid: {startTime: identity.startTime, pidfd: pidfd}},
		observeExit: func() (bool, error) { return true, nil },
	}
	result := containment.terminate()
	if result.forcedMembers || result.err != nil || !result.leaderStopped {
		t.Fatalf("termination = %+v, want naturally exited zombie excluded from forced cleanup", result)
	}
	if err := child.Wait(); err != nil && !errors.Is(err, syscall.ECHILD) {
		t.Fatal(err)
	}
	reaped = true
}

func TestLinuxProcessGoneIncludesProcfsESRCHRace(t *testing.T) {
	for _, err := range []error{
		&os.PathError{Op: "read", Path: "/proc/123/stat", Err: syscall.ENOENT},
		&os.PathError{Op: "read", Path: "/proc/123/stat", Err: syscall.ESRCH},
	} {
		if !linuxProcessGone(err) {
			t.Fatalf("linuxProcessGone(%v) = false, want benign disappearance", err)
		}
	}
	if linuxProcessGone(syscall.EACCES) {
		t.Fatal("permission failure was misclassified as process disappearance")
	}
}

func processStillRunning(pid int) bool {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return true
	}
	closing := strings.LastIndexByte(string(stat), ')')
	return closing < 0 || len(stat) <= closing+2 || stat[closing+2] != 'Z'
}

func writeTestProcessIdentity(path string, pid int) error {
	identity, err := readLinuxProcessIdentity(pid)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d %d", pid, identity.startTime)), 0o600)
}

func parseTestProcessIdentity(body []byte) (int, uint64, error) {
	var pid int
	var startTime uint64
	_, err := fmt.Fscan(strings.NewReader(string(body)), &pid, &startTime)
	return pid, startTime, err
}

func processIdentityStillRunning(pid int, startTime uint64) bool {
	identity, err := readLinuxProcessIdentity(pid)
	if linuxProcessGone(err) {
		return false
	}
	if err != nil {
		return true
	}
	return identity.startTime == startTime && !identity.zombie
}
