//go:build linux

package process

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const linuxProcStatLimit = 4 << 10
const linuxProcChildrenLimit = 64 << 10
const linuxProcTaskEntriesLimit = linuxSupervisorTraversalMaxVisits

type linuxTaskProcOps struct {
	readFile   func(string, int) ([]byte, error)
	taskExists func(string) (bool, error)
}

// A release snapshot remains an error until the exact task directory is
// independently absent. Its stable identity also preserves replacement checks.
type linuxTaskReleaseIdentityError struct {
	identity linuxProcessIdentity
}

func (*linuxTaskReleaseIdentityError) Error() string { return "malformed proc task stable identity" }

// Retain the original observation without allowing one of its errnos to stand
// for proven disappearance at an outer traversal boundary. No Unwrap or Is.
type linuxTaskIdentityUncertainty struct {
	cause error
}

func (failure *linuxTaskIdentityUncertainty) Error() string { return failure.cause.Error() }

func ordinaryContainmentPreflight() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return ordinaryContainmentPreflightWith(os.Getpid(), unix.Gettid(), linuxDefaultTaskProcOps())
}

func ordinaryContainmentPreflightWith(pid, tid int, ops linuxTaskProcOps) error {
	_, present, err := linuxTaskChildrenWith(pid, tid, ops)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("current locked OS thread disappeared during ordinary containment preflight")
	}
	return nil
}

func linuxDefaultTaskProcOps() linuxTaskProcOps {
	return linuxTaskProcOps{readFile: readBoundedLinuxProcFile, taskExists: linuxTaskDirectoryExists}
}

func readBoundedLinuxProcFile(path string, limit int) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("proc observation is not a regular file: %s", path)
	}
	body, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > limit {
		return nil, fmt.Errorf("proc observation exceeded %d bytes: %s", limit, path)
	}
	return body, nil
}

func linuxTaskDirectoryExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if linuxProcessGone(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("proc task is not a directory: %s", path)
	}
	return true, nil
}

func linuxTaskChildren(pid, tid int) ([]byte, bool, error) {
	return linuxTaskChildrenWith(pid, tid, linuxDefaultTaskProcOps())
}

func linuxTaskChildrenWith(pid, tid int, ops linuxTaskProcOps) ([]byte, bool, error) {
	task := fmt.Sprintf("/proc/%d/task/%d", pid, tid)
	before, err := linuxReadTaskIdentity(task, tid, ops)
	if err != nil {
		gone, observeErr := linuxTaskGoneAfterIdentityError(task, err, ops)
		return nil, !gone, observeErr
	}
	if before.zombie {
		return nil, false, nil
	}
	body, err := ops.readFile(task+"/children", linuxProcChildrenLimit)
	if err == nil {
		if err := validateLinuxProcChildren(body); err != nil {
			return nil, true, err
		}
		return body, true, nil
	}
	if !linuxProcessGone(err) {
		return nil, true, fmt.Errorf("read live proc task children: %w", err)
	}
	after, identityErr := linuxReadTaskIdentity(task, tid, ops)
	if identityErr != nil {
		//nolint:errorlint // Only the direct private parser marker carries an admitted release identity.
		if released, ok := identityErr.(*linuxTaskReleaseIdentityError); ok && released != nil &&
			(before.pid != released.identity.pid || before.startTime != released.identity.startTime) {
			return nil, true, fmt.Errorf("proc task changed identity while its children interface was missing")
		}
		gone, observeErr := linuxTaskGoneAfterIdentityError(task, identityErr, ops)
		return nil, !gone, observeErr
	}
	if before.pid != after.pid || before.startTime != after.startTime {
		return nil, true, fmt.Errorf("proc task changed identity while its children interface was missing")
	}
	if after.zombie {
		return nil, false, nil
	}
	// Do not wrap ENOENT: ancestor traversal treats that errno as a process
	// disappearance. This task was independently proved to remain alive.
	if os.IsNotExist(err) {
		return nil, true, fmt.Errorf("%w: live task %d/%d has no children interface (%s)", ErrOrdinaryContainmentUnsupported, pid, tid, err.Error())
	}
	return nil, true, fmt.Errorf("live task %d/%d children observation failed: %s", pid, tid, err.Error())
}

func linuxTaskGoneAfterIdentityError(task string, identityErr error, ops linuxTaskProcOps) (bool, error) {
	//nolint:errorlint // Wrapped or joined release markers must not authorize a disappearance probe.
	released, direct := identityErr.(*linuxTaskReleaseIdentityError)
	if !direct || released == nil {
		var nested *linuxTaskReleaseIdentityError
		if errors.As(identityErr, &nested) {
			return false, &linuxTaskIdentityUncertainty{cause: identityErr}
		}
		if !linuxProcessGone(identityErr) {
			return false, identityErr
		}
	}
	exists, err := ops.taskExists(task)
	if err != nil {
		if released != nil {
			return false, errors.Join(released, &linuxTaskIdentityUncertainty{cause: err})
		}
		return false, err
	}
	if !exists {
		return true, nil
	}
	if released != nil {
		return false, released
	}
	// A missing stat inside a still-present task is unknown observation, not
	// evidence that the task disappeared or that its children list was empty.
	return false, withOrdinaryCleanupOrigin("task_identity_unavailable", fmt.Errorf("existing proc task identity is unavailable: %s", identityErr.Error()))
}

func linuxReadTaskIdentity(task string, tid int, ops linuxTaskProcOps) (linuxProcessIdentity, error) {
	body, err := ops.readFile(task+"/stat", linuxProcStatLimit)
	if err != nil {
		return linuxProcessIdentity{}, err
	}
	return parseLinuxTaskIdentity(body, tid)
}

func parseLinuxTaskIdentity(body []byte, tid int) (linuxProcessIdentity, error) {
	raw := string(body)
	opening, closing := strings.IndexByte(raw, '('), strings.LastIndexByte(raw, ')')
	if opening <= 0 || closing < opening || strings.TrimSpace(raw[:opening]) != strconv.Itoa(tid) || tid <= 0 {
		return linuxProcessIdentity{}, fmt.Errorf("malformed proc task identity")
	}
	fields := strings.Fields(raw[closing+1:])
	if len(fields) < 20 || len(fields[0]) != 1 || strings.IndexByte("RSDZTtXxKWPI", fields[0][0]) < 0 {
		return linuxProcessIdentity{}, fmt.Errorf("malformed proc task state")
	}
	for _, field := range fields[1:] {
		if field == "" || strings.HasPrefix(field, "+") {
			return linuxProcessIdentity{}, fmt.Errorf("malformed proc task numeric field")
		}
		if _, err := strconv.ParseUint(strings.TrimPrefix(field, "-"), 10, 64); err != nil {
			return linuxProcessIdentity{}, fmt.Errorf("malformed proc task numeric field: %w", err)
		}
	}
	parent, parentErr := strconv.Atoi(fields[1])
	group, groupErr := strconv.Atoi(fields[2])
	started, startErr := strconv.ParseUint(fields[19], 10, 64)
	if parentErr != nil || groupErr != nil || startErr != nil || parent < 0 {
		return linuxProcessIdentity{}, fmt.Errorf("malformed proc task stable identity")
	}
	identity := linuxProcessIdentity{pid: tid, ppid: parent, pgid: group, startTime: started, zombie: strings.IndexByte("ZXx", fields[0][0]) >= 0}
	if group < 0 {
		if linuxTaskReleaseRecord(fields) {
			return linuxProcessIdentity{}, &linuxTaskReleaseIdentityError{identity: identity}
		}
		return linuxProcessIdentity{}, fmt.Errorf("malformed proc task stable identity")
	}
	return identity, nil
}

func linuxTaskReleaseRecord(fields []string) bool {
	// do_task_stat leaves these defaults when the task's sighand was released.
	// State is sampled earlier and does not itself establish disappearance.
	if len(fields) < 50 || fields[1] != "0" || fields[2] != "-1" || fields[3] != "-1" {
		return false
	}
	for _, field := range fields[1:] {
		magnitude := strings.TrimPrefix(field, "-")
		number, err := strconv.ParseUint(magnitude, 10, 64)
		if err != nil || strconv.FormatUint(number, 10) != magnitude || strings.HasPrefix(field, "-") && number == 0 {
			return false
		}
	}
	return true
}

func validateLinuxProcChildren(body []byte) error {
	if len(body) > linuxProcChildrenLimit {
		return fmt.Errorf("proc children observation exceeded %d bytes", linuxProcChildrenLimit)
	}
	for _, value := range strings.Fields(string(body)) {
		pid, err := strconv.Atoi(value)
		if err != nil || pid <= 0 || strconv.Itoa(pid) != value {
			return fmt.Errorf("malformed proc children identity %q", value)
		}
	}
	return nil
}

func readBoundedLinuxTaskDirectory(path string) ([]os.DirEntry, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	entries, err := file.ReadDir(linuxProcTaskEntriesLimit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > linuxProcTaskEntriesLimit {
		return nil, fmt.Errorf("proc task enumeration exceeded %d entries", linuxProcTaskEntriesLimit)
	}
	return entries, nil
}

func linuxTaskID(name string) (int, error) {
	tid, err := strconv.Atoi(name)
	if err != nil || tid <= 0 || strconv.Itoa(tid) != name {
		return 0, fmt.Errorf("invalid proc task identity %q", name)
	}
	return tid, nil
}
