//go:build linux

package process

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type commandContainment struct {
	cmd                      *exec.Cmd
	mu                       sync.Mutex
	descendants              map[int]linuxTrackedProcess
	scanErr                  error
	stop                     chan struct{}
	done                     chan struct{}
	trackerStarted           bool
	supervisorRequest        *os.File
	supervisorStatus         *os.File
	supervisorResult         linuxOrdinarySupervisorResult
	supervisorRead           bool
	cgroupPath               string
	cgroupDir                *os.File
	closeTransferred         bool
	observeExit              func() (bool, error)
	observeEmpty             func() (bool, error)
	inspectLive              func(int) (bool, error)
	prepareLeaderTermination func(int, time.Duration) (bool, error)
}

// This interval is frequent enough to observe normal Kiro startup forks while
// avoiding high-frequency process inspection for long-lived clients.
const linuxDescendantScanInterval = 25 * time.Millisecond

const linuxStartupDescendantScanInterval = time.Millisecond
const linuxStartupDescendantScanWindow = 250 * time.Millisecond

const linuxOrdinarySupervisorEnvironment = "PLUGIN_KIT_AI_ORDINARY_PROCESS_SUPERVISOR=1"

type linuxOrdinarySupervisorSpec struct {
	Path  string
	Args  []string
	Env   []string
	Dir   string
	Grace time.Duration
}

type linuxOrdinarySupervisorResult struct {
	Forced bool
	Error  string
}

func init() {
	if os.Getenv("PLUGIN_KIT_AI_ORDINARY_PROCESS_SUPERVISOR") != "1" {
		return
	}
	os.Exit(runLinuxOrdinarySupervisor())
}

func explicitContainmentSupervision() bool { return true }

const linuxCgroupRoot = "/sys/fs/cgroup"

func duplexContainmentPreflight() error {
	cmd := exec.Command("/bin/sh", "-c", "while :; do :; done")
	containment, err := newDuplexCommandContainment(cmd)
	if err != nil {
		return fmt.Errorf("safe duplex process containment unavailable; manual activation required: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return errors.Join(fmt.Errorf("prove CLONE_INTO_CGROUP process creation: %w", err), containment.close())
	}
	if err := containment.attach(cmd); err != nil {
		return fmt.Errorf("attach duplex containment primitive probe: %w", cleanupAttachFailure(err, containment.terminate, cmd.Process.Kill, containment.exited, containment.close, cmd.Wait))
	}
	termination := containment.terminate()
	if !termination.leaderStopped {
		termination = stopLeaderAfterContainmentFailure(termination, cmd.Process.Kill, containment.exited)
	}
	return finishDuplexContainmentProbe(termination, containment.close, cmd.Wait, processReapTimeout)
}

func finishDuplexContainmentProbe(termination terminationResult, closeContainment, wait func() error, timeout time.Duration) error {
	// Close probe-owned containment descriptors before starting its sole Wait.
	// If shutdown cannot be proved or Wait itself stalls, the buffered reaper
	// keeps ownership until actual exit while this capability check fails closed
	// within a real deadline.
	waitErr := closeContainmentAndWait(closeContainment, wait, timeout)
	var proofErr error
	if !termination.leaderStopped {
		proofErr = fmt.Errorf("probe leader shutdown was not proved")
	} else if !termination.leaderTerminationInitiated {
		proofErr = fmt.Errorf("probe leader shutdown was not initiated by containment")
	}
	if termination.err != nil || proofErr != nil {
		return fmt.Errorf("safe duplex process containment primitive probe failed: %w", errors.Join(termination.err, proofErr, waitErr))
	}
	if waitErr != nil {
		if _, expectedKill := waitErr.(*exec.ExitError); !expectedKill {
			return fmt.Errorf("reap duplex containment primitive probe: %w", waitErr)
		}
	}
	return nil
}

func naturalExitNeedsContainmentCleanup() bool { return true }

func plannedTerminationExitExpected(err error) bool {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGKILL
}

func newCommandContainment(cmd *exec.Cmd) (*commandContainment, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &commandContainment{
		cmd:         cmd,
		descendants: make(map[int]linuxTrackedProcess),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}, nil
}

func newOrdinaryCommandContainment(cmd *exec.Cmd, grace time.Duration) (*commandContainment, error) {
	request, status, err := wrapLinuxOrdinaryCommand(cmd, grace)
	if err != nil {
		return nil, err
	}
	containment, err := newCommandContainment(cmd)
	if err != nil {
		_ = request.Close()
		_ = status.Close()
		return nil, err
	}
	containment.supervisorRequest = request
	containment.supervisorStatus = status
	return containment, nil
}

func wrapLinuxOrdinaryCommand(cmd *exec.Cmd, grace time.Duration) (*os.File, *os.File, error) {
	if len(cmd.ExtraFiles) != 0 {
		return nil, nil, fmt.Errorf("Linux ordinary-command supervisor requires unallocated extra descriptors")
	}
	commandEnvironment := cmd.Env
	if commandEnvironment == nil {
		commandEnvironment = os.Environ()
	}
	spec := linuxOrdinarySupervisorSpec{Path: cmd.Path, Args: append([]string(nil), cmd.Args...), Env: append([]string(nil), commandEnvironment...), Dir: cmd.Dir, Grace: grace}
	encoded, err := json.Marshal(spec)
	if err != nil {
		return nil, nil, fmt.Errorf("encode Linux ordinary-command supervisor request: %w", err)
	}
	fd, err := unix.MemfdCreate("plugin-kit-ai-command", unix.MFD_CLOEXEC)
	if err != nil {
		return nil, nil, fmt.Errorf("create Linux ordinary-command supervisor request: %w", err)
	}
	specFile := os.NewFile(uintptr(fd), "plugin-kit-ai-command")
	cleanupSpec := true
	defer func() {
		if cleanupSpec {
			_ = specFile.Close()
		}
	}()
	if _, err := specFile.Write(encoded); err != nil {
		return nil, nil, fmt.Errorf("write Linux ordinary-command supervisor request: %w", err)
	}
	if _, err := specFile.Seek(0, io.SeekStart); err != nil {
		return nil, nil, fmt.Errorf("rewind Linux ordinary-command supervisor request: %w", err)
	}
	requestReader, requestWriter, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	statusReader, statusWriter, err := os.Pipe()
	if err != nil {
		_ = requestReader.Close()
		_ = requestWriter.Close()
		return nil, nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		_ = requestReader.Close()
		_ = requestWriter.Close()
		_ = statusReader.Close()
		_ = statusWriter.Close()
		return nil, nil, fmt.Errorf("locate Linux ordinary-command supervisor: %w", err)
	}
	cmd.Path = executable
	cmd.Args = []string{executable}
	cmd.Env = append(os.Environ(), linuxOrdinarySupervisorEnvironment)
	cmd.Dir = ""
	cmd.ExtraFiles = append(cmd.ExtraFiles, specFile, requestReader, statusWriter)
	cleanupSpec = false
	// These child-side descriptors are closed by attach after Start has copied
	// them into the isolated supervisor. Keep them on the containment so every
	// pre-Start and attach failure also has one cleanup owner.
	cmd.Cancel = nil
	return requestWriter, statusReader, nil
}

func runLinuxOrdinarySupervisor() int {
	specFile := os.NewFile(3, "plugin-kit-ai-command")
	request := os.NewFile(4, "plugin-kit-ai-command-cleanup")
	status := os.NewFile(5, "plugin-kit-ai-command-status")
	// ExtraFiles deliberately clears CLOEXEC while launching this supervisor.
	// Restore it before starting the supervised command: descriptors 3-5 are
	// supervisor authority and must never be available to the command itself.
	for _, fd := range []int{3, 4, 5} {
		unix.CloseOnExec(fd)
	}
	result := linuxOrdinarySupervisorResult{}
	writeResult := func() {
		_ = json.NewEncoder(status).Encode(result)
		_ = status.Close()
	}
	var spec linuxOrdinarySupervisorSpec
	if err := json.NewDecoder(specFile).Decode(&spec); err != nil {
		result.Error = fmt.Sprintf("decode supervisor request: %v", err)
		writeResult()
		return 125
	}
	_ = specFile.Close()
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		result.Error = fmt.Sprintf("establish isolated child-subreaper boundary: %v", err)
		writeResult()
		return 125
	}
	child := exec.Command(spec.Path, spec.Args[1:]...)
	child.Env = spec.Env
	child.Dir = spec.Dir
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		result.Error = fmt.Sprintf("start supervised command: %v", err)
		writeResult()
		return 125
	}
	requested := make(chan struct{}, 1)
	go func() {
		_, _ = io.Copy(io.Discard, request)
		requested <- struct{}{}
	}()
	cleanupErr, forced := superviseLinuxOrdinaryChildren(child.Process.Pid, requested, spec.Grace)
	// Wait only after supervision has stopped using the leader as the stable
	// process-group anchor. Until this sole reap, its numeric PID/PGID cannot be
	// recycled, so no group signal can target an unrelated process.
	waitErr := child.Wait()
	result.Forced = forced
	if cleanupErr != nil {
		result.Error = cleanupErr.Error()
	}
	writeResult()
	if waitErr == nil {
		return 0
	}
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			// Go ignores SIGPIPE by default, so signalling this supervisor would
			// fall through to the generic 125 status instead of preserving the
			// supervised command's broken-pipe cause.
			if status.Signal() == syscall.SIGPIPE {
				return 128 + int(syscall.SIGPIPE)
			}
			_ = syscall.Kill(os.Getpid(), status.Signal())
			return 125
		}
		if exitErr.ExitCode() >= 0 {
			return exitErr.ExitCode()
		}
	}
	return 125
}

func superviseLinuxOrdinaryChildren(leaderPID int, requested <-chan struct{}, grace time.Duration) (error, bool) {
	waitComplete := false
	forced := false
	var deadline time.Time
	var cleanupErr error
	emptyBoundary := linuxQuiescentBoundary{required: 3}
	groupSignaled := false
	for {
		live, inspectErr := linuxSupervisorLiveDescendants(os.Getpid())
		if !waitComplete {
			var info unix.Siginfo
			if err := unix.Waitid(unix.P_PID, leaderPID, &info, unix.WEXITED|unix.WNOWAIT|unix.WNOHANG, nil); err != nil {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("observe supervised command exit: %w", err))
			} else if info.Signo != 0 {
				waitComplete = true
				deadline = time.Now().Add(grace)
			}
		}
		select {
		case <-requested:
			if !forced {
				forced = true
				deadline = time.Now().Add(processReapTimeout)
			}
		default:
		}
		if waitComplete && live > 0 && !deadline.IsZero() && time.Now().After(deadline) && !forced {
			forced = true
			deadline = time.Now().Add(processReapTimeout)
		}
		if forced {
			// The leader is deliberately unreaped here. Signal its numeric group at
			// most once and only while that stable anchor is still controllably
			// live; after natural exit, cleanup uses pidfd-bound descendants only.
			if !groupSignaled {
				groupSignaled = true
				if err := signalOwnedLinuxProcessGroup(leaderPID, waitComplete, syscall.Kill); err != nil && !errors.Is(err, syscall.ESRCH) {
					cleanupErr = errors.Join(cleanupErr, fmt.Errorf("terminate supervised command group: %w", err))
				}
			}
			cleanupErr = errors.Join(cleanupErr, killLinuxSupervisorDescendants(os.Getpid()))
		}
		// A single non-atomic procfs traversal can race a fork/reparent. Require
		// three error-free, time-separated empty snapshots after leader exit.
		if emptyBoundary.observe(waitComplete && live == 0, inspectErr) {
			return cleanupErr, forced
		} else {
			if inspectErr != nil {
				cleanupErr = errors.Join(cleanupErr, inspectErr)
			}
		}
		if forced && !deadline.IsZero() && time.Now().After(deadline) {
			return errors.Join(cleanupErr, fmt.Errorf("isolated ordinary-command supervisor did not empty before cleanup deadline")), forced
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// signalOwnedLinuxProcessGroup prohibits numeric PGID signaling once leader
// exit has been observed. The leader remains unreaped throughout supervision,
// but an exited leader no longer proves that its process group is the intended
// live ownership boundary; detached members are handled through stable pidfds.
func signalOwnedLinuxProcessGroup(leaderPID int, leaderExited bool, signal func(int, syscall.Signal) error) error {
	if leaderExited {
		return nil
	}
	return signal(-leaderPID, syscall.SIGKILL)
}

type linuxQuiescentBoundary struct {
	required int
	passes   int
}

func (boundary *linuxQuiescentBoundary) observe(empty bool, scanErr error) bool {
	if !empty || scanErr != nil {
		boundary.passes = 0
		return false
	}
	boundary.passes++
	return boundary.passes >= boundary.required
}

func linuxSupervisorLiveDescendants(root int) (int, error) {
	identities, err := linuxDescendantIdentities(root)
	if err != nil {
		return 0, err
	}
	live := 0
	for _, identity := range identities {
		if identity.zombie {
			continue
		}
		live++
	}
	return live, nil
}

func killLinuxSupervisorDescendants(root int) error {
	identities, err := linuxDescendantIdentities(root)
	var resultErr = err
	for pid, identity := range identities {
		if identity.zombie {
			continue
		}
		pidfd, err := unix.PidfdOpen(pid, 0)
		if err != nil {
			if !errors.Is(err, syscall.ESRCH) {
				resultErr = errors.Join(resultErr, fmt.Errorf("open stable supervisor child handle for %d: %w", pid, err))
			}
			continue
		}
		current, err := readLinuxProcessIdentity(pid)
		if err == nil && current.startTime == identity.startTime && !current.zombie {
			if err := unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0); err != nil && !errors.Is(err, syscall.ESRCH) {
				resultErr = errors.Join(resultErr, fmt.Errorf("terminate stable supervisor child %d: %w", pid, err))
			}
		}
		_ = unix.Close(pidfd)
	}
	return resultErr
}

func linuxDescendantIdentities(root int) (map[int]linuxProcessIdentity, error) {
	result := make(map[int]linuxProcessIdentity)
	queue := []int{root}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		children, err := linuxDirectChildIdentities(parent)
		if err != nil {
			if linuxProcessGone(err) {
				continue
			}
			return result, err
		}
		for pid := range children {
			if _, exists := result[pid]; exists {
				continue
			}
			identity, err := readLinuxProcessIdentity(pid)
			if linuxProcessGone(err) {
				continue
			}
			if err != nil {
				return result, err
			}
			result[pid] = identity
			queue = append(queue, pid)
		}
	}
	return result, nil
}

func newDuplexCommandContainment(cmd *exec.Cmd) (*commandContainment, error) {
	path, directory, err := createDelegatedCgroup()
	if err != nil {
		return nil, fmt.Errorf("create duplex cgroup containment: %w", err)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, UseCgroupFD: true, CgroupFD: int(directory.Fd())}
	return &commandContainment{cmd: cmd, cgroupPath: path, cgroupDir: directory}, nil
}

func createDelegatedCgroup() (string, *os.File, error) {
	return createDelegatedCgroupAt(linuxCgroupRoot)
}

func createDelegatedCgroupAt(root string) (string, *os.File, error) {
	value, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", nil, err
	}
	var relative string
	for _, line := range strings.Split(string(value), "\n") {
		if strings.HasPrefix(line, "0::") {
			relative = strings.TrimPrefix(line, "0::")
			break
		}
	}
	if relative == "" || !filepath.IsAbs(relative) {
		return "", nil, fmt.Errorf("unified cgroup v2 membership is unavailable")
	}
	base := filepath.Join(root, filepath.Clean(relative))
	var filesystem unix.Statfs_t
	if err := unix.Statfs(base, &filesystem); err != nil {
		return "", nil, fmt.Errorf("inspect cgroup filesystem: %w", err)
	}
	if filesystem.Type != unix.CGROUP2_SUPER_MAGIC {
		return "", nil, fmt.Errorf("unified cgroup v2 filesystem is unavailable")
	}
	path, err := os.MkdirTemp(base, "agentplugins-duplex-")
	if err != nil {
		return "", nil, fmt.Errorf("delegated cgroup v2 subtree is unavailable: %w", err)
	}
	directory, err := os.Open(path)
	if err != nil {
		_ = os.Remove(path)
		return "", nil, err
	}
	for _, control := range []string{"cgroup.events", "cgroup.kill", "cgroup.procs"} {
		if _, err := os.Stat(filepath.Join(path, control)); err != nil {
			_ = directory.Close()
			_ = os.Remove(path)
			return "", nil, fmt.Errorf("required cgroup v2 control %s is unavailable: %w", control, err)
		}
	}
	return path, directory, nil
}

func (containment *commandContainment) attach(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return fmt.Errorf("process supervision cannot attach before start")
	}
	if containment.cgroupPath != "" {
		return nil // CLONE_INTO_CGROUP attached the child atomically at creation.
	}
	if containment.supervisorRequest != nil {
		for _, file := range cmd.ExtraFiles {
			if err := file.Close(); err != nil {
				return fmt.Errorf("close inherited Linux supervisor descriptor: %w", err)
			}
		}
		cmd.ExtraFiles = nil
	}
	identity, err := readLinuxProcessIdentity(cmd.Process.Pid)
	if err != nil {
		// A trusted command can complete between Start and attachment. Its
		// process group remains the containment boundary and Wait still owns the
		// exact exit status; there is no live leader identity left to stabilize.
		// Treat this narrow natural-exit race as successful attachment instead of
		// turning a rapid successful command into a runner failure.
		if linuxProcessGone(err) {
			return nil
		}
		return fmt.Errorf("read Linux process leader identity: %w", err)
	}
	pidfd, err := unix.PidfdOpen(identity.pid, 0)
	if err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return fmt.Errorf("open stable Linux process handle: %w", err)
	}
	containment.mu.Lock()
	containment.descendants[identity.pid] = linuxTrackedProcess{startTime: identity.startTime, pidfd: pidfd}
	containment.mu.Unlock()
	if err := containment.scanDescendants(); err != nil {
		return err
	}
	containment.trackerStarted = true
	go containment.trackDescendants()
	return nil
}

// Git and other tree-grace callers need the same detached-descendant proof as
// every ordinary command. Retain this hook for platform-neutral call sites;
// Linux intentionally does not reduce containment to a process group.
func (containment *commandContainment) limitToProcessGroup() {}

func (containment *commandContainment) terminate() terminationResult {
	if containment.cmd.Process == nil {
		return terminationResult{err: os.ErrProcessDone}
	}
	if containment.cgroupPath != "" {
		return containment.terminateCgroup()
	}
	if containment.supervisorRequest != nil {
		return containment.terminateSupervisor()
	}
	scanErr := containment.scanDescendants()
	leaderLive, leaderInspectErr := linuxProcessLive(containment.cmd.Process.Pid)
	scanErr = errors.Join(scanErr, leaderInspectErr)
	forcedMembers := false
	groupMembers, groupScanErr := liveLinuxProcessGroupMembers(containment.cmd.Process.Pid)
	if groupMembers > 0 {
		forcedMembers = true
	}
	scanErr = errors.Join(scanErr, groupScanErr)
	containment.mu.Lock()
	for pid, tracked := range containment.descendants {
		if pid == containment.cmd.Process.Pid {
			continue
		}
		live, err := liveLinuxTrackedProcess(pid, tracked)
		if err != nil {
			scanErr = errors.Join(scanErr, fmt.Errorf("classify tracked descendant %d before cleanup: %w", pid, err))
			// If identity cannot be classified, preserve forced-cleanup
			// causality rather than accepting a natural-looking command exit.
			forcedMembers = true
		} else if live {
			forcedMembers = true
		}
	}
	containment.mu.Unlock()
	groupErr := syscall.Kill(-containment.cmd.Process.Pid, syscall.SIGKILL)
	leaderTerminationInitiated := leaderLive && groupErr == nil
	if errors.Is(groupErr, syscall.ESRCH) {
		groupErr = nil
	}
	containment.mu.Lock()
	identities := make(map[int]linuxTrackedProcess, len(containment.descendants))
	for pid, tracked := range containment.descendants {
		identities[pid] = tracked
	}
	trackerErr := containment.scanErr
	containment.mu.Unlock()
	var cleanupErr = errors.Join(scanErr, trackerErr)
	for pid, tracked := range identities {
		if pid == containment.cmd.Process.Pid {
			continue
		}
		identity, err := readLinuxProcessIdentity(pid)
		if errors.Is(err, os.ErrNotExist) || (err == nil && identity.zombie) {
			continue
		}
		if err != nil || identity.startTime != tracked.startTime {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("prove tracked descendant %d identity before cleanup: %w", pid, err))
			continue
		}
		if err := unix.PidfdSendSignal(tracked.pidfd, unix.SIGKILL, nil, 0); err != nil && !errors.Is(err, syscall.ESRCH) {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("terminate tracked descendant %d through pidfd: %w", pid, err))
		}
	}
	deadline := time.Now().Add(time.Second)
	emptyPasses := 0
	for {
		// Killing a tracked parent can expose a child that forked after the
		// previous snapshot. As the active subreaper, recover each newly adopted
		// identity and repeat until a full pass proves that no live member remains.
		if err := containment.scanDescendants(); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
		containment.mu.Lock()
		for pid, tracked := range containment.descendants {
			if _, known := identities[pid]; known {
				continue
			}
			identities[pid] = tracked
			if pid == containment.cmd.Process.Pid {
				continue
			}
			live, err := liveLinuxTrackedProcess(pid, tracked)
			if err != nil {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("classify newly adopted descendant %d before cleanup: %w", pid, err))
				forcedMembers = true
			} else if live {
				forcedMembers = true
				if err := unix.PidfdSendSignal(tracked.pidfd, unix.SIGKILL, nil, 0); err != nil && !errors.Is(err, syscall.ESRCH) {
					cleanupErr = errors.Join(cleanupErr, fmt.Errorf("terminate newly adopted descendant %d through pidfd: %w", pid, err))
				}
			}
		}
		containment.mu.Unlock()
		leaderStopped, observeErr := containment.exited()
		remaining := 0
		for pid, tracked := range identities {
			if pid == containment.cmd.Process.Pid {
				continue
			}
			identity, err := readLinuxProcessIdentity(pid)
			if err == nil && identity.startTime == tracked.startTime && identity.zombie && identity.ppid == os.Getpid() {
				var info unix.Siginfo
				if reapErr := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOHANG, nil); reapErr != nil && !errors.Is(reapErr, syscall.ECHILD) && !errors.Is(reapErr, syscall.ESRCH) {
					cleanupErr = errors.Join(cleanupErr, fmt.Errorf("reap adopted tracked descendant %d: %w", pid, reapErr))
				}
				continue
			}
			if err == nil && identity.startTime == tracked.startTime && !identity.zombie && linuxPidfdAlive(tracked.pidfd) {
				remaining++
			} else if err != nil && linuxPidfdAlive(tracked.pidfd) {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("prove tracked descendant %d stopped: %w", pid, err))
			}
		}
		if leaderStopped && remaining == 0 {
			emptyPasses++
			if emptyPasses >= 2 {
				return terminationResult{leaderStopped: true, leaderTerminationInitiated: leaderTerminationInitiated, forcedMembers: forcedMembers, err: errors.Join(groupErr, cleanupErr)}
			}
		} else {
			emptyPasses = 0
		}
		if observeErr != nil {
			cleanupErr = errors.Join(cleanupErr, observeErr)
		}
		if time.Now().After(deadline) {
			if !leaderStopped {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("process leader did not become waitable before cleanup deadline"))
			}
			if remaining != 0 {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%d tracked descendant process(es) survived cleanup deadline", remaining))
			}
			return terminationResult{leaderStopped: leaderStopped, leaderTerminationInitiated: leaderTerminationInitiated, forcedMembers: forcedMembers, err: errors.Join(groupErr, cleanupErr)}
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (containment *commandContainment) terminateSupervisor() terminationResult {
	alreadyExited, observeErr := containment.exited()
	requestErr := containment.supervisorRequest.Close()
	containment.supervisorRequest = nil
	if errors.Is(requestErr, os.ErrClosed) {
		requestErr = nil
	}
	deadline := time.Now().Add(processReapTimeout)
	exited := alreadyExited
	for !exited && observeErr == nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		exited, observeErr = containment.exited()
	}
	if !exited && observeErr == nil {
		observeErr = fmt.Errorf("isolated ordinary-command supervisor did not stop before cleanup deadline")
	}
	var statusErr error
	if exited {
		statusErr = containment.readSupervisorResult()
		if containment.supervisorResult.Error != "" {
			statusErr = errors.Join(statusErr, errors.New(containment.supervisorResult.Error))
		}
	}
	return terminationResult{
		leaderStopped:              exited,
		leaderTerminationInitiated: !alreadyExited && requestErr == nil,
		forcedMembers:              containment.supervisorResult.Forced,
		err:                        errors.Join(requestErr, observeErr, statusErr),
	}
}

func (containment *commandContainment) readSupervisorResult() error {
	if containment.supervisorRead || containment.supervisorStatus == nil {
		return nil
	}
	containment.supervisorRead = true
	decoder := json.NewDecoder(containment.supervisorStatus)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&containment.supervisorResult)
	if err == nil {
		var trailing any
		if trailingErr := decoder.Decode(&trailing); !errors.Is(trailingErr, io.EOF) {
			if trailingErr == nil {
				trailingErr = fmt.Errorf("additional JSON value")
			}
			err = fmt.Errorf("isolated ordinary-command supervisor returned trailing status data: %w", trailingErr)
		}
	}
	closeErr := containment.supervisorStatus.Close()
	containment.supervisorStatus = nil
	if errors.Is(err, io.EOF) {
		err = fmt.Errorf("isolated ordinary-command supervisor returned no cleanup proof")
	}
	return errors.Join(err, closeErr)
}

func (containment *commandContainment) exited() (bool, error) {
	if containment.observeExit != nil {
		return containment.observeExit()
	}
	if containment.cmd.Process == nil {
		return false, nil
	}
	var info unix.Siginfo
	if err := unix.Waitid(unix.P_PID, containment.cmd.Process.Pid, &info, unix.WEXITED|unix.WNOWAIT|unix.WNOHANG, nil); err != nil {
		return false, err
	}
	return info.Signo != 0, nil
}

func (containment *commandContainment) settleNaturalExit(ctx context.Context, timeout time.Duration) (bool, error) {
	if containment.cgroupPath != "" {
		return containment.settleCgroup(ctx, timeout)
	}
	deadline := time.Now().Add(timeout)
	for {
		members, err := liveLinuxProcessGroupMembers(containment.cmd.Process.Pid)
		if err != nil {
			return false, err
		}
		containment.mu.Lock()
		for pid, tracked := range containment.descendants {
			if pid == containment.cmd.Process.Pid {
				continue
			}
			identity, identityErr := readLinuxProcessIdentity(pid)
			if identityErr == nil && !identity.zombie && linuxPidfdAlive(tracked.pidfd) {
				members++
			}
		}
		containment.mu.Unlock()
		if members == 0 {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (containment *commandContainment) close() (resultErr error) {
	if containment.closeTransferred {
		return fmt.Errorf("Linux descendant tracker ownership was transferred to its reaper")
	}
	if containment.cgroupPath != "" {
		if containment.cgroupDir != nil {
			_ = containment.cgroupDir.Close()
			containment.cgroupDir = nil
		}
		if err := os.Remove(containment.cgroupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove duplex cgroup scope: %w", err)
		}
		containment.cgroupPath = ""
		return nil
	}
	if containment.supervisorRequest != nil {
		resultErr = errors.Join(resultErr, containment.supervisorRequest.Close())
		containment.supervisorRequest = nil
	}
	if containment.supervisorStatus != nil {
		resultErr = errors.Join(resultErr, containment.supervisorStatus.Close())
		containment.supervisorStatus = nil
	}
	for _, file := range containment.cmd.ExtraFiles {
		resultErr = errors.Join(resultErr, file.Close())
	}
	containment.cmd.ExtraFiles = nil
	if containment.cmd.Process == nil {
		return nil
	}
	if containment.trackerStarted {
		select {
		case <-containment.stop:
		default:
			close(containment.stop)
		}
		select {
		case <-containment.done:
		case <-time.After(500 * time.Millisecond):
			containment.closeTransferred = true
			go containment.closeTrackedHandlesAfterStop()
			return fmt.Errorf("Linux descendant tracker ownership transferred to reaper after stop deadline")
		}
	}
	containment.mu.Lock()
	for pid, tracked := range containment.descendants {
		if err := unix.Close(tracked.pidfd); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close stable process handle for %d: %w", pid, err))
		}
	}
	containment.descendants = nil
	containment.mu.Unlock()
	return resultErr
}

func (containment *commandContainment) closeTrackedHandlesAfterStop() {
	<-containment.done
	containment.mu.Lock()
	defer containment.mu.Unlock()
	for _, tracked := range containment.descendants {
		_ = unix.Close(tracked.pidfd)
	}
	containment.descendants = nil
}

var readCgroupProcs = os.ReadFile
var writeCgroupKill = os.WriteFile

func (containment *commandContainment) terminateCgroup() terminationResult {
	return containment.terminateCgroupWithin(time.Second)
}

func (containment *commandContainment) terminateCgroupWithin(timeout time.Duration) terminationResult {
	procsPath := filepath.Join(containment.cgroupPath, "cgroup.procs")
	members, inspectErr := readCgroupProcs(procsPath)
	forced, memberInspectErr, releaseMemberIdentities := containment.cgroupForcedMemberCausality(procsPath, members, inspectErr)
	defer releaseMemberIdentities()
	inspectErr = memberInspectErr
	prepare := containment.prepareLeaderTermination
	if prepare == nil {
		prepare = prepareLinuxLeaderTermination
	}
	leaderPrepared, leaderInspectErr := prepare(containment.cmd.Process.Pid, timeout)
	inspectErr = errors.Join(inspectErr, leaderInspectErr)
	killErr := writeCgroupKill(filepath.Join(containment.cgroupPath, "cgroup.kill"), []byte("1"), 0)
	leaderTerminationInitiated := leaderPrepared && killErr == nil
	if errors.Is(killErr, os.ErrNotExist) {
		killErr = nil
	}
	deadline := time.Now().Add(timeout)
	for {
		exited, observeErr := containment.exited()
		empty, emptyErr := containment.cgroupEmpty()
		if exited && empty {
			return terminationResult{leaderStopped: true, leaderTerminationInitiated: leaderTerminationInitiated, forcedMembers: forced, err: errors.Join(inspectErr, killErr, observeErr, emptyErr)}
		}
		if time.Now().After(deadline) {
			return terminationResult{leaderStopped: exited, leaderTerminationInitiated: leaderTerminationInitiated, forcedMembers: forced, err: errors.Join(inspectErr, killErr, observeErr, emptyErr, fmt.Errorf("duplex cgroup did not empty before cleanup deadline"))}
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (containment *commandContainment) cgroupForcedMemberCausality(procsPath string, members []byte, inspectErr error) (bool, error, func()) {
	if containment.inspectLive != nil {
		forced, err := cgroupForcedMemberCausalityWithInspect(containment.cmd.Process.Pid, members, inspectErr, containment.inspectLive)
		return forced, err, func() {}
	}
	return stableCgroupForcedMemberCausality(containment.cmd.Process.Pid, procsPath, members, inspectErr)
}

// stableCgroupForcedMemberCausality binds every pre-signal liveness decision to
// a pidfd and procfs start time. cgroup.procs contains numeric PIDs, so using a
// later unbound /proc lookup could attribute cleanup either to a naturally
// exited zombie or to a recycled identity. The second membership snapshot
// closes the list-to-pidfd race; retained pidfds keep each classified identity
// stable through cgroup.kill.
func stableCgroupForcedMemberCausality(leaderPID int, procsPath string, members []byte, inspectErr error) (bool, error, func()) {
	type candidate struct {
		identity linuxProcessIdentity
		pidfd    int
	}
	candidates := make(map[int]candidate)
	release := func() {
		for _, candidate := range candidates {
			_ = unix.Close(candidate.pidfd)
		}
	}
	if inspectErr != nil {
		return true, fmt.Errorf("inspect cgroup members before forced cleanup: %w", inspectErr), release
	}
	for _, raw := range strings.Fields(string(members)) {
		pid, err := strconv.Atoi(raw)
		if err != nil {
			return true, fmt.Errorf("parse cgroup member %q before forced cleanup: %w", raw, err), release
		}
		if pid == leaderPID {
			continue
		}
		identity, err := readLinuxProcessIdentity(pid)
		if linuxProcessGone(err) {
			continue
		}
		if err != nil {
			return true, fmt.Errorf("inspect cgroup member %d identity before forced cleanup: %w", pid, err), release
		}
		pidfd, err := unix.PidfdOpen(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return true, fmt.Errorf("open stable cgroup member handle for %d: %w", pid, err), release
		}
		candidates[pid] = candidate{identity: identity, pidfd: pidfd}
	}
	confirmedMembers, err := readCgroupProcs(procsPath)
	if err != nil {
		return true, fmt.Errorf("confirm stable cgroup members before forced cleanup: %w", err), release
	}
	confirmed := make(map[int]bool)
	for _, raw := range strings.Fields(string(confirmedMembers)) {
		pid, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			return true, fmt.Errorf("parse confirmed cgroup member %q before forced cleanup: %w", raw, parseErr), release
		}
		confirmed[pid] = true
	}
	forced := false
	for pid, candidate := range candidates {
		if !confirmed[pid] {
			continue
		}
		current, err := readLinuxProcessIdentity(pid)
		if linuxProcessGone(err) || !linuxPidfdAlive(candidate.pidfd) {
			continue
		}
		if err != nil {
			return true, fmt.Errorf("confirm cgroup member %d identity before forced cleanup: %w", pid, err), release
		}
		if current.startTime != candidate.identity.startTime {
			return true, fmt.Errorf("cgroup member %d identity changed before forced cleanup", pid), release
		}
		if !current.zombie {
			forced = true
		}
	}
	return forced, nil, release
}

// prepareLinuxLeaderTermination establishes a causal boundary before
// cgroup.kill. Merely observing a live /proc entry is insufficient: a leader
// can already be inside natural exit and receive SIGKILL before it becomes a
// waitable zombie. A leader that accepts SIGSTOP and reaches a stopped state is
// still controllably live after planned teardown was requested; a leader that
// becomes a zombie first must be reported as a premature natural exit.
func prepareLinuxLeaderTermination(pid int, timeout time.Duration) (bool, error) {
	identity, err := readLinuxProcessIdentity(pid)
	if linuxProcessGone(err) || (err == nil && identity.zombie) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect process leader before termination boundary: %w", err)
	}
	pidfd, err := unix.PidfdOpen(pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open stable process leader handle before termination boundary: %w", err)
	}
	defer unix.Close(pidfd)
	if err := unix.PidfdSendSignal(pidfd, unix.SIGSTOP, nil, 0); errors.Is(err, syscall.ESRCH) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("stop process leader at termination boundary: %w", err)
	}
	deadline := time.Now().Add(timeout)
	for {
		current, err := readLinuxProcessIdentity(pid)
		if linuxProcessGone(err) || (err == nil && current.startTime == identity.startTime && current.zombie) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("observe process leader termination boundary: %w", err)
		}
		if current.startTime != identity.startTime {
			return false, fmt.Errorf("process leader identity changed at termination boundary")
		}
		if current.stopped {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, fmt.Errorf("process leader did not stop before termination boundary deadline")
		}
		time.Sleep(time.Millisecond)
	}
}

func cgroupForcedMemberCausality(leaderPID int, members []byte, inspectErr error) (bool, error) {
	return cgroupForcedMemberCausalityWithInspect(leaderPID, members, inspectErr, linuxProcessLive)
}

func cgroupForcedMemberCausalityWithInspect(leaderPID int, members []byte, inspectErr error, inspect func(int) (bool, error)) (bool, error) {
	if inspectErr != nil {
		// Once cgroup.kill is requested, inability to inventory the scope makes
		// descendant causality uncertain. Fail closed so a natural-looking leader
		// status cannot turn possibly forced cleanup into success.
		return true, fmt.Errorf("inspect cgroup members before forced cleanup: %w", inspectErr)
	}
	for _, raw := range strings.Fields(string(members)) {
		pid, err := strconv.Atoi(raw)
		if err != nil {
			return true, fmt.Errorf("parse cgroup member %q before forced cleanup: %w", raw, err)
		}
		if pid == leaderPID {
			continue
		}
		live, err := inspect(pid)
		if err != nil {
			return true, fmt.Errorf("classify cgroup member %d before forced cleanup: %w", pid, err)
		}
		if live {
			return true, nil
		}
	}
	return false, nil
}

func (containment *commandContainment) processLive(pid int) (bool, error) {
	if containment.inspectLive != nil {
		return containment.inspectLive(pid)
	}
	return linuxProcessLive(pid)
}

// linuxProcessLive classifies the exact, currently installed procfs identity.
// A zombie is already naturally exited and must not be attributed to a later
// cleanup signal; disappearance during inspection likewise proves there is no
// remaining identity to signal.
func linuxProcessLive(pid int) (bool, error) {
	identity, err := readLinuxProcessIdentity(pid)
	if linuxProcessGone(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !identity.zombie, nil
}

func (containment *commandContainment) settleCgroup(ctx context.Context, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		empty, err := containment.cgroupEmpty()
		if err != nil || empty {
			return empty, err
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (containment *commandContainment) cgroupEmpty() (bool, error) {
	if containment.observeEmpty != nil {
		return containment.observeEmpty()
	}
	value, err := os.ReadFile(filepath.Join(containment.cgroupPath, "cgroup.events"))
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(value), "\n") {
		if strings.TrimSpace(line) == "populated 0" {
			return true, nil
		}
	}
	return false, nil
}

type linuxProcessIdentity struct {
	pid       int
	ppid      int
	pgid      int
	startTime uint64
	zombie    bool
	stopped   bool
}

func readLinuxProcessIdentity(pid int) (linuxProcessIdentity, error) {
	value, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return linuxProcessIdentity{}, err
	}
	closing := strings.LastIndexByte(string(value), ')')
	if closing < 0 || closing+2 >= len(value) {
		return linuxProcessIdentity{}, fmt.Errorf("malformed /proc stat for pid %d", pid)
	}
	fields := strings.Fields(string(value[closing+2:]))
	if len(fields) < 20 {
		return linuxProcessIdentity{}, fmt.Errorf("short /proc stat for pid %d", pid)
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return linuxProcessIdentity{}, err
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return linuxProcessIdentity{}, err
	}
	pgid, err := strconv.Atoi(fields[2])
	if err != nil {
		return linuxProcessIdentity{}, err
	}
	return linuxProcessIdentity{pid: pid, ppid: ppid, pgid: pgid, startTime: startTime, zombie: fields[0] == "Z", stopped: fields[0] == "T" || fields[0] == "t"}, nil
}

func liveLinuxProcessGroupMembers(pgid int) (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, fmt.Errorf("inspect Linux process table for group containment: %w", err)
	}
	members := 0
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == pgid {
			continue
		}
		identity, err := readLinuxProcessIdentity(pid)
		if err != nil {
			continue // unrelated processes routinely exit during a table snapshot
		}
		if identity.pgid == pgid && !identity.zombie {
			members++
		}
	}
	return members, nil
}

type linuxTrackedProcess struct {
	startTime uint64
	pidfd     int
}

func linuxPidfdAlive(pidfd int) bool {
	err := unix.PidfdSendSignal(pidfd, 0, nil, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func linuxProcessGone(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ESRCH)
}

// liveLinuxTrackedProcess distinguishes a live descendant from an exited but
// unreaped zombie. pidfd signal checks alone report zombies as present, which
// would incorrectly attribute forced cleanup to a helper that exited naturally.
// The start time binds the /proc observation to the identity represented by the
// pidfd; disappearance after the pidfd check is an ordinary exit race.
func liveLinuxTrackedProcess(pid int, tracked linuxTrackedProcess) (bool, error) {
	if !linuxPidfdAlive(tracked.pidfd) {
		return false, nil
	}
	identity, err := readLinuxProcessIdentity(pid)
	if linuxProcessGone(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if identity.startTime != tracked.startTime {
		return false, fmt.Errorf("process start time changed")
	}
	return !identity.zombie, nil
}

func linuxDirectChildIdentities(pid int) (map[int]uint64, error) {
	childrenByIdentity := make(map[int]uint64)
	tasks, err := os.ReadDir(fmt.Sprintf("/proc/%d/task", pid))
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		children, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%s/children", pid, task.Name()))
		if err != nil {
			if linuxProcessGone(err) {
				continue
			}
			return nil, err
		}
		for _, rawChild := range strings.Fields(string(children)) {
			child, err := strconv.Atoi(rawChild)
			if err != nil {
				continue
			}
			identity, err := readLinuxProcessIdentity(child)
			if linuxProcessGone(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			childrenByIdentity[child] = identity.startTime
		}
	}
	return childrenByIdentity, nil
}

func (containment *commandContainment) trackDescendants() {
	defer close(containment.done)
	ticker := time.NewTicker(linuxStartupDescendantScanInterval)
	defer ticker.Stop()
	startup := time.NewTimer(linuxStartupDescendantScanWindow)
	defer startup.Stop()
	for {
		select {
		case <-ticker.C:
			if err := containment.scanDescendants(); err != nil {
				containment.mu.Lock()
				containment.scanErr = errors.Join(containment.scanErr, err)
				containment.mu.Unlock()
			}
		case <-startup.C:
			ticker.Reset(linuxDescendantScanInterval)
		case <-containment.stop:
			return
		}
	}
}

func (containment *commandContainment) scanDescendants() error {
	containment.mu.Lock()
	defer containment.mu.Unlock()
	queue := make([]int, 0, len(containment.descendants)+1)
	queue = append(queue, containment.cmd.Process.Pid)
	for pid, tracked := range containment.descendants {
		if identity, err := readLinuxProcessIdentity(pid); err == nil && identity.startTime == tracked.startTime && !identity.zombie {
			queue = append(queue, pid)
		}
	}
	var scanErr error
	seen := make(map[int]struct{}, len(queue))
	for len(queue) != 0 {
		pid := queue[0]
		queue = queue[1:]
		if _, duplicate := seen[pid]; duplicate {
			continue
		}
		seen[pid] = struct{}{}
		tasks, err := os.ReadDir(fmt.Sprintf("/proc/%d/task", pid))
		if err != nil {
			if tracked, ok := containment.descendants[pid]; ok {
				live, classifyErr := liveLinuxTrackedProcess(pid, tracked)
				if classifyErr != nil {
					scanErr = errors.Join(scanErr, fmt.Errorf("classify tracked process %d after task inspection failed: %w", pid, classifyErr))
				} else if live {
					scanErr = errors.Join(scanErr, fmt.Errorf("inspect tasks for live tracked process %d: %w", pid, err))
				}
			}
			continue
		}
		for _, task := range tasks {
			children, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%s/children", pid, task.Name()))
			if err != nil {
				// Threads can disappear between ReadDir and opening their children
				// file. The remaining live threads are still inspected in this same
				// snapshot, so this race is not an ancestry blind spot.
				if linuxProcessGone(err) {
					continue
				}
				if tracked, ok := containment.descendants[pid]; ok {
					live, classifyErr := liveLinuxTrackedProcess(pid, tracked)
					if classifyErr != nil {
						scanErr = errors.Join(scanErr, fmt.Errorf("classify tracked process %d after child inspection failed: %w", pid, classifyErr))
					} else if live {
						scanErr = errors.Join(scanErr, fmt.Errorf("inspect children for live tracked process %d: %w", pid, err))
					}
				}
				continue
			}
			for _, rawChild := range strings.Fields(string(children)) {
				child, err := strconv.Atoi(rawChild)
				if err != nil {
					continue
				}
				identity, err := readLinuxProcessIdentity(child)
				if err != nil {
					// A child can exit after the kernel children snapshot but
					// before /proc/<pid>/stat is opened. There is no process left
					// to contain in that case, so this ordinary exit race is not a
					// loss of containment evidence.
					if linuxProcessGone(err) {
						continue
					}
					scanErr = errors.Join(scanErr, fmt.Errorf("read observed child %d of %d: %w", child, pid, err))
					continue
				}
				pidfd, err := unix.PidfdOpen(child, 0)
				if err != nil {
					// Kernels can report ESRCH or EINVAL when that same observed
					// child exits before the stable handle is acquired. Accept the
					// race only after /proc proves the observed identity is no
					// longer live; every failure for a surviving child remains
					// fail-closed.
					current, currentErr := readLinuxProcessIdentity(child)
					if linuxProcessGone(currentErr) || (currentErr == nil && (current.zombie || current.startTime != identity.startTime)) {
						continue
					}
					scanErr = errors.Join(scanErr, fmt.Errorf("open stable handle for observed child %d: %w", child, err))
					continue
				}
				revalidated, err := readLinuxProcessIdentity(child)
				if err != nil || revalidated.startTime != identity.startTime {
					_ = unix.Close(pidfd)
					if err != nil {
						// The pidfd pins the identity against reuse. If /proc now
						// reports it gone, the observed child completed naturally;
						// there is no descendant left to track or signal.
						if linuxProcessGone(err) {
							continue
						}
						scanErr = errors.Join(scanErr, fmt.Errorf("revalidate observed child %d identity: %w", child, err))
					} else {
						scanErr = errors.Join(scanErr, fmt.Errorf("revalidate observed child %d identity: process start time changed", child))
					}
					continue
				}
				if revalidated.ppid != pid {
					parent, parentErr := readLinuxProcessIdentity(pid)
					if parentErr == nil && !parent.zombie {
						current, currentErr := readLinuxProcessIdentity(child)
						if linuxProcessGone(currentErr) || (currentErr == nil && (current.zombie || current.startTime != identity.startTime)) {
							_ = unix.Close(pidfd)
							continue
						}
						_ = unix.Close(pidfd)
						scanErr = errors.Join(scanErr, fmt.Errorf("revalidate observed child %d parentage: live parent changed from %d to %d", child, pid, revalidated.ppid))
						continue
					}
				}
				// Re-read parentage after acquiring the pidfd. A changed parent is
				// safe only because the kernel children file already established the
				// relationship and the stable handle proves the same process survived
				// while its parent exited/reparented it. Numeric signaling is never used.
				if previous, exists := containment.descendants[child]; exists {
					_ = unix.Close(pidfd)
					if previous.startTime != revalidated.startTime {
						scanErr = errors.Join(scanErr, fmt.Errorf("tracked child PID %d changed identity", child))
						continue
					}
				} else {
					containment.descendants[child] = linuxTrackedProcess{startTime: revalidated.startTime, pidfd: pidfd}
				}
				queue = append(queue, child)
			}
		}
	}
	return scanErr
}
