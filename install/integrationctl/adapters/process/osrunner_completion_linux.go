//go:build linux

package process

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/unix"
)

type linuxOrdinarySupervisorSpec struct {
	Path         string
	Args         []string
	Env          []string
	Dir          string
	Grace        time.Duration
	ForwardStdin bool
}

func newCompletionCommandContainment(cmd *exec.Cmd, grace time.Duration) (*commandContainment, error) {
	return newOrdinaryCommandContainmentInput(cmd, grace, ordinaryContainmentPreflight, true)
}

func newOrdinaryCommandContainmentWithPreflight(cmd *exec.Cmd, grace time.Duration, preflight func() error) (*commandContainment, error) {
	return newOrdinaryCommandContainmentInput(cmd, grace, preflight, false)
}

func newOrdinaryCommandContainmentInput(cmd *exec.Cmd, grace time.Duration, preflight func() error, forwardStdin bool) (*commandContainment, error) {
	if err := preflight(); err != nil {
		return nil, err
	}
	request, status, err := wrapLinuxOrdinaryCommand(cmd, grace, forwardStdin)
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

func wrapLinuxOrdinaryCommand(cmd *exec.Cmd, grace time.Duration, forwardStdin bool) (*os.File, *os.File, error) {
	if len(cmd.ExtraFiles) != 0 {
		return nil, nil, fmt.Errorf("linux ordinary-command supervisor requires unallocated extra descriptors")
	}
	commandEnvironment := cmd.Env
	if commandEnvironment == nil {
		commandEnvironment = os.Environ()
	}
	spec := linuxOrdinarySupervisorSpec{Path: cmd.Path, Args: append([]string(nil), cmd.Args...), Env: append([]string(nil), commandEnvironment...), Dir: cmd.Dir, Grace: grace, ForwardStdin: forwardStdin}
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
	// A managed plugin may replace and unlink its own pathname during a repair.
	// The procfs self link still names the running inode in the forked child, so
	// the supervisor remains launchable throughout the replacement.
	const selfExecutable = "/proc/self/exe"
	cmd.Path = selfExecutable
	cmd.Args = []string{selfExecutable}
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
