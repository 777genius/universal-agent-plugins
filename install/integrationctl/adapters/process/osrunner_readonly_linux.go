//go:build linux

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"golang.org/x/sys/unix"
)

// readonlyDuplexFactory changes only the containment constructor of this local
// copy. The caller's prepare must admit and construct the complete readonly
// command view. An export lease alone supplies no filesystem or native proof.
func readonlyDuplexFactory(ctx context.Context, pipes duplexPipeFactory, prepare func(context.Context, *exec.Cmd) (*readonlyExportLease, error)) duplexPipeFactory {
	baseFactory := pipes.containment
	pipes.containment = func(cmd *exec.Cmd) (duplexCommandContainment, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if baseFactory == nil || prepare == nil {
			return nil, errors.New("readonly duplex preparation is required")
		}
		base, err := baseFactory(cmd)
		if err != nil {
			if base != nil {
				err = errors.Join(err, base.close())
			}
			return nil, err
		}
		if base == nil {
			return nil, errors.New("readonly duplex containment is required")
		}
		if err := ctx.Err(); err != nil {
			return nil, errors.Join(err, base.close())
		}
		lease, err := prepare(ctx, cmd)
		if err == nil && !lease.ready(cmd) {
			err = errors.New("readonly duplex export ownership differs")
		}
		err = errors.Join(err, ctx.Err())
		if err != nil {
			return nil, errors.Join(err, lease.release(), base.close())
		}
		return &readonlyDuplexContainment{base: base, lease: lease}, nil
	}
	return pipes
}

type readonlyDuplexContainment struct {
	base  duplexCommandContainment
	lease *readonlyExportLease
}

func (containment *readonlyDuplexContainment) attach(cmd *exec.Cmd) error {
	// The cgroup attach returns early. This wrapper always releases its parent
	// export copies, including when attach itself reports an error.
	return errors.Join(containment.base.attach(cmd), containment.lease.release())
}

func (containment *readonlyDuplexContainment) terminate() terminationResult {
	return containment.base.terminate()
}

func (containment *readonlyDuplexContainment) exited() (bool, error) {
	return containment.base.exited()
}

func (containment *readonlyDuplexContainment) close() error {
	// A successful cgroup close clears its path. A subsequent retry can reach
	// the base's generic ExtraFiles loop, so detach our entries first, every
	// time. release retains any error without closing the descriptors twice.
	return errors.Join(containment.lease.release(), containment.base.close())
}

const readonlyExportLimit = 16

// A lease owns only fresh export copies. Original caller authority handles are
// borrowed by export and never stored as owned files. Child copies must be
// consumed by the admitted view constructor before native exec.
type readonlyExportLease struct {
	cmd       *exec.Cmd
	mu        sync.Mutex
	files     []*os.File
	released  bool
	closeErr  error
	closeFile func(*os.File) error
}

func newReadonlyExportLease(cmd *exec.Cmd) *readonlyExportLease {
	return &readonlyExportLease{cmd: cmd, closeFile: (*os.File).Close}
}

func (lease *readonlyExportLease) export(original *os.File) (int, error) {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if original == nil || lease.cmd == nil || lease.released || lease.cmd.Process != nil || len(lease.files) >= readonlyExportLimit {
		return 0, errors.New("readonly export is unavailable")
	}
	fd, err := unix.FcntlInt(original.Fd(), unix.F_DUPFD_CLOEXEC, 3)
	runtime.KeepAlive(original)
	if err != nil {
		return 0, fmt.Errorf("duplicate readonly export: %w", err)
	}
	exportFile := os.NewFile(uintptr(fd), "readonly-view-export")
	lease.files = append(lease.files, exportFile)
	childFD := 3 + len(lease.cmd.ExtraFiles)
	lease.cmd.ExtraFiles = append(lease.cmd.ExtraFiles, exportFile)
	return childFD, nil
}

func (lease *readonlyExportLease) ready(cmd *exec.Cmd) bool {
	if lease == nil {
		return false
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if cmd == nil || lease.cmd != cmd || cmd.Process != nil || lease.released || len(lease.files) == 0 || len(cmd.ExtraFiles) != len(lease.files) {
		return false
	}
	for i, file := range lease.files {
		if file != cmd.ExtraFiles[i] {
			return false
		}
	}
	return true
}

func (lease *readonlyExportLease) release() error {
	if lease == nil {
		return nil
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.released {
		return lease.closeErr
	}
	lease.released = true
	owned := make(map[*os.File]bool, len(lease.files))
	for _, file := range lease.files {
		owned[file] = true
	}
	if lease.cmd != nil {
		var remaining []*os.File
		for _, file := range lease.cmd.ExtraFiles {
			if !owned[file] {
				remaining = append(remaining, file)
			}
		}
		lease.cmd.ExtraFiles = remaining
	}
	for _, file := range lease.files {
		lease.closeErr = errors.Join(lease.closeErr, lease.closeFile(file))
	}
	lease.files = nil
	return lease.closeErr
}
