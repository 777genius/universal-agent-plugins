//go:build darwin

package scaffold

import (
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

// Darwin renameatx_np with RENAME_EXCL checks absence in the kernel. Use the
// maintained x/sys libc binding (including arm64), not raw syscall dispatch.
func renameExclusive(from *os.File, old string, to *os.File, new string) error {
	err := unix.RenameatxNp(int(from.Fd()), old, int(to.Fd()), new, unix.RENAME_EXCL)
	runtime.KeepAlive(from)
	runtime.KeepAlive(to)
	if err != nil {
		return &os.LinkError{Op: "rename-exclusive", Old: old, New: new, Err: err}
	}
	return nil
}
