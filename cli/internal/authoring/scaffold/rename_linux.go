//go:build linux

package scaffold

import (
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

// Same kernel contract as providers' narrow primitive, using anchored directory
// descriptors instead of AT_FDCWD. ENOSYS/EINVAL/EOPNOTSUPP fail closed; no rename
// fallback is safe. RENAME_NOREPLACE returns EEXIST even for an empty destination.
func renameExclusive(from *os.File, old string, to *os.File, new string) error {
	err := unix.Renameat2(int(from.Fd()), old, int(to.Fd()), new, unix.RENAME_NOREPLACE)
	runtime.KeepAlive(from)
	runtime.KeepAlive(to)
	if err != nil {
		return &os.LinkError{Op: "rename-noreplace", Old: old, New: new, Err: err}
	}
	return nil
}
