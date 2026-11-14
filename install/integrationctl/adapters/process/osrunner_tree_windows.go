//go:build windows

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type commandContainment struct {
	cmd      *exec.Cmd
	job      windows.Handle
	process  windows.Handle
	mu       sync.Mutex
	attached bool
	once     sync.Once
	kill     func() error
	confirm  func(time.Duration) (bool, error)
}

var terminateWindowsJob = windows.TerminateJobObject
var closeWindowsHandle = windows.CloseHandle

func explicitContainmentSupervision() bool { return true }

func duplexContainmentPreflight() error {
	// A Job can only be proven usable after a real child has been created,
	// assigned, resumed, and terminated. Automatic Kiro activation also needs a
	// race-free real-pipe settlement primitive, which is not implemented on
	// Windows. Reject the automatic path before any package/native mutation.
	return fmt.Errorf("safe duplex ACP containment and pipe settlement are unavailable on Windows; manual activation required")
}

func naturalExitNeedsContainmentCleanup() bool { return true }

func plannedTerminationExitExpected(err error) bool {
	exitErr, ok := err.(*exec.ExitError)
	return ok && exitErr.ExitCode() == int(cleanupExitCode)
}

const cleanupExitCode uint32 = 0xc0de0001

func newCommandContainment(cmd *exec.Cmd) (*commandContainment, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create process job: %w", err)
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		closeErr := closeWindowsHandle(job)
		return nil, errors.Join(fmt.Errorf("configure process job: %w", err), closeErr)
	}
	containment := &commandContainment{cmd: cmd, job: job}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	return containment, nil
}

func newOrdinaryCommandContainment(cmd *exec.Cmd, _ time.Duration) (*commandContainment, error) {
	return newCommandContainment(cmd)
}

func newDuplexCommandContainment(cmd *exec.Cmd) (*commandContainment, error) {
	return newCommandContainment(cmd)
}

func (containment *commandContainment) attach(cmd *exec.Cmd) (resultErr error) {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fmt.Errorf("open suspended process: %w", err)
	}
	if err := windows.AssignProcessToJobObject(containment.job, process); err != nil {
		closeErr := closeWindowsHandle(process)
		return errors.Join(fmt.Errorf("assign process job: %w", err), closeErr)
	}
	containment.mu.Lock()
	containment.attached = true
	containment.process = process
	containment.mu.Unlock()
	thread, err := suspendedProcessThread(uint32(cmd.Process.Pid))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, closeWindowsHandle(thread)) }()
	if _, err := windows.ResumeThread(thread); err != nil {
		return fmt.Errorf("resume process thread: %w", err)
	}
	return nil
}

func (*commandContainment) limitToProcessGroup() {}

func suspendedProcessThread(pid uint32) (windows.Handle, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return 0, fmt.Errorf("snapshot suspended process threads: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, openErr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if openErr != nil {
			return 0, fmt.Errorf("open suspended process thread: %w", openErr)
		}
		return thread, nil
	}
	if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return 0, fmt.Errorf("suspended process thread not found")
	}
	return 0, fmt.Errorf("enumerate suspended process threads: %w", err)
}

func (containment *commandContainment) terminate() terminationResult {
	containment.mu.Lock()
	attached := containment.attached
	containment.mu.Unlock()
	if attached {
		forcedMembers := containment.activeProcesses() > 0
		if err := terminateWindowsJob(containment.job, cleanupExitCode); err == nil {
			stopped, confirmErr := containment.confirmTermination(time.Second)
			if !stopped && confirmErr == nil {
				confirmErr = fmt.Errorf("process job remained active after cleanup deadline")
			}
			return terminationResult{leaderStopped: stopped, forcedMembers: forcedMembers, err: confirmErr}
		} else {
			killErr := containment.killLeader()
			if errors.Is(killErr, os.ErrProcessDone) {
				killErr = nil
			}
			stopped, confirmErr := containment.confirmTermination(time.Second)
			if !stopped && confirmErr == nil {
				confirmErr = fmt.Errorf("leader fallback kill was requested but job shutdown was not observed")
			}
			return terminationResult{leaderStopped: stopped, err: errors.Join(fmt.Errorf("terminate process job: %w", err), killErr, confirmErr)}
		}
	}
	killErr := containment.killLeader()
	if errors.Is(killErr, os.ErrProcessDone) {
		killErr = nil
	}
	return terminationResult{err: errors.Join(killErr, fmt.Errorf("unattached process shutdown could not be observed"))}
}

func (containment *commandContainment) activeProcesses() uint32 {
	var accounting jobAccounting
	if err := windows.QueryInformationJobObject(containment.job, windows.JobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); err != nil {
		return 1 // fail closed: containment could not prove the job was empty
	}
	return accounting.ActiveProcesses
}

type jobAccounting struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

func (containment *commandContainment) confirmTermination(timeout time.Duration) (bool, error) {
	if containment.confirm != nil {
		return containment.confirm(timeout)
	}
	deadline := time.Now().Add(timeout)
	for {
		var accounting jobAccounting
		if err := windows.QueryInformationJobObject(containment.job, windows.JobObjectBasicAccountingInformation,
			uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); err != nil {
			return false, fmt.Errorf("confirm process job termination: %w", err)
		}
		leaderStopped, err := containment.exited()
		if err != nil {
			return false, fmt.Errorf("confirm process leader termination: %w", err)
		}
		if leaderStopped && accounting.ActiveProcesses == 0 {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (containment *commandContainment) killLeader() error {
	if containment.kill != nil {
		return containment.kill()
	}
	if containment.cmd.Process == nil {
		return os.ErrProcessDone
	}
	if err := containment.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func (containment *commandContainment) exited() (bool, error) {
	containment.mu.Lock()
	process := containment.process
	containment.mu.Unlock()
	if process == 0 {
		return false, nil
	}
	event, err := windows.WaitForSingleObject(process, 0)
	if err != nil {
		return false, err
	}
	return event == windows.WAIT_OBJECT_0, nil
}

func (containment *commandContainment) settleNaturalExit(ctx context.Context, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for containment.activeProcesses() > 0 {
		if time.Now().After(deadline) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
	return true, nil
}

func (containment *commandContainment) close() (resultErr error) {
	containment.once.Do(func() {
		containment.mu.Lock()
		process := containment.process
		containment.process = 0
		containment.mu.Unlock()
		if process != 0 {
			resultErr = errors.Join(resultErr, closeWindowsHandle(process))
		}
		resultErr = errors.Join(resultErr, closeWindowsHandle(containment.job))
	})
	return resultErr
}
