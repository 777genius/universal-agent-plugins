//go:build windows

package managedstdio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var assignProcessToJobObject = windows.AssignProcessToJobObject
var terminateSessionJob = windows.TerminateJobObject
var closeSessionJobAfterTerminationFailure = windows.CloseHandle
var resumeSessionChild = resumeSessionProcess

// Windows cannot replace the current process with execve. A private Job
// Object gives the managed helper equivalent process-tree ownership: the
// launched runtime is created suspended, attached before it can execute, then
// resumed and waited on with inherited stdio.
func Supported() bool { return true }

func replaceProcess(command string, args []string, cwd string) error {
	return replaceProcessEnvironment(command, args, cwd, os.Environ())
}

func replaceProcessEnvironment(command string, args []string, cwd string, environment []string) error {
	return replaceSessionContext(context.Background(), command, args, cwd, environment)
}

func replaceSessionContext(ctx context.Context, command string, args []string, cwd string, environment []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if command == "" || !filepathIsAbs(cwd) {
		return errors.New("managed stdio requires an absolute command and cwd")
	}
	cmd := exec.Command(command, args[1:]...)
	cmd.Dir = cwd
	cmd.Env = environment
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create managed stdio job: %w", err)
	}
	defer func() {
		if job != 0 {
			_ = windows.CloseHandle(job)
		}
	}()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return fmt.Errorf("configure managed stdio job: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("open managed stdio process: %w", err)
	}
	defer windows.CloseHandle(process)
	if err := assignProcessToJobObject(job, process); err != nil {
		// Assignment failed, so the suspended process is not owned by the job
		// and TerminateJobObject cannot stop it. Kill it directly before Wait;
		// otherwise this error path can wait forever on a never-resumed thread.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("assign managed stdio process: %w", err)
	}
	// Consent may be withdrawn after the suspended process joins our Job Object.
	// Terminate and reap it through the existing cancellation boundary; never
	// resume a child after consent has been withdrawn.
	if ctx.Err() != nil {
		return waitSessionContext(ctx, cmd, &job)
	}
	if err := resumeSessionChild(cmd.Process.Pid); err != nil {
		_ = windows.TerminateJobObject(job, 1)
		_ = cmd.Wait()
		return err
	}
	return waitSessionContext(ctx, cmd, &job)
}

func waitSessionContext(ctx context.Context, cmd *exec.Cmd, job *windows.Handle) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		select {
		case err := <-done:
			return err
		default:
		}
		terminateErr := terminateSessionJob(*job, 1)
		if terminateErr != nil {
			// Last-handle close enforces KILL_ON_JOB_CLOSE if termination fails.
			closeErr := closeSessionJobAfterTerminationFailure(*job)
			if closeErr == nil {
				*job = 0
			} else {
				// Both termination and last-handle close failed. Without a
				// direct kill, Wait can block forever on the suspended child.
				// Preserve the close error: descendants are not proven reaped.
				if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
					closeErr = errors.Join(closeErr, fmt.Errorf("kill native session child after failed job close: %w", killErr))
				}
			}
			terminateErr = errors.Join(terminateErr, closeErr)
		}
		return errors.Join(ctx.Err(), terminateErr, <-done)
	}
}

func firstProcessThread(pid uint32) (windows.Handle, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return 0, fmt.Errorf("snapshot managed stdio threads: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, openErr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if openErr != nil {
			return 0, fmt.Errorf("open managed stdio thread: %w", openErr)
		}
		return thread, nil
	}
	if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return 0, errors.New("managed stdio process thread not found")
	}
	return 0, fmt.Errorf("enumerate managed stdio threads: %w", err)
}

// Use the same Windows path semantics as StartSessionContext. A drive-letter
// heuristic wrongly refuses absolute UNC and extended-length project roots.
func filepathIsAbs(path string) bool {
	return filepath.IsAbs(path)
}

func resumeSessionProcess(pid int) (resultErr error) {
	if pid <= 0 || uint64(pid) > uint64(^uint32(0)) {
		return fmt.Errorf("invalid native process ID")
	}
	thread, err := firstProcessThread(uint32(pid))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, windows.CloseHandle(thread)) }()
	if _, err := windows.ResumeThread(thread); err != nil {
		return fmt.Errorf("resume managed stdio process: %w", err)
	}
	return nil
}
