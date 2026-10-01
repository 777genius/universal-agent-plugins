//go:build darwin

// Package terminalreader owns cancellable reads without owning caller handles.
package terminalreader

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/muesli/cancelreader"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// Darwin kqueue can report /dev/tty readable without available input. A file
// inherited as stdin retains the name /dev/stdin, bypassing cancelreader's
// filename-based select fallback. Poll the actual terminal descriptor instead;
// physical TTYs support poll; Darwin's virtual /dev/tty instead needs select.
// Neither backend changes caller flags or closes the borrowed handle.
func New(f *os.File) (cancelreader.CancelReader, error) {
	if !term.IsTerminal(int(f.Fd())) {
		return cancelreader.NewReader(f)
	}
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	return &reader{file: f, wake: r, signal: w}, nil
}

type reader struct {
	file, wake, signal *os.File
	canceled           atomic.Bool
	once               sync.Once
	cancelOK           bool
}

func (r *reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if r.canceled.Load() {
			return 0, cancelreader.ErrCanceled
		}
		fds := []unix.PollFd{{Fd: int32(r.file.Fd()), Events: unix.POLLIN}, {Fd: int32(r.wake.Fd()), Events: unix.POLLIN}}
		_, err := unix.Poll(fds, -1)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("poll terminal input: %w", err)
		}
		if r.canceled.Load() || fds[1].Revents != 0 {
			return 0, cancelreader.ErrCanceled
		}
		if fds[0].Revents&unix.POLLNVAL != 0 {
			// Darwin reports POLLNVAL for its valid virtual /dev/tty device.
			// A non-reading select fallback also preserves consent snapshots.
			return r.readSelect(p)
		}
		if fds[0].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0 {
			return r.file.Read(p)
		}
	}
}

func (r *reader) Cancel() bool {
	r.once.Do(func() {
		r.canceled.Store(true)
		_, err := r.signal.Write([]byte{1})
		r.cancelOK = err == nil
	})
	return r.cancelOK
}

// The session joins active reads before closing only these owned wakeup pipes.
func (r *reader) Close() error { return errors.Join(r.signal.Close(), r.wake.Close()) }

func (r *reader) readSelect(p []byte) (int, error) {
	input, wake := int(r.file.Fd()), int(r.wake.Fd())
	if input < 0 || wake < 0 {
		return 0, os.ErrClosed
	}
	if input >= unix.FD_SETSIZE || wake >= unix.FD_SETSIZE {
		return 0, fmt.Errorf("virtual terminal descriptor exceeds select limit")
	}
	for {
		if r.canceled.Load() {
			return 0, cancelreader.ErrCanceled
		}
		var fds unix.FdSet
		fds.Set(input)
		fds.Set(wake)
		_, err := unix.Select(max(input, wake)+1, &fds, nil, nil, nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("select terminal input: %w", err)
		}
		if r.canceled.Load() || fds.IsSet(wake) {
			return 0, cancelreader.ErrCanceled
		}
		if fds.IsSet(input) {
			return r.file.Read(p)
		}
	}
}
