//go:build darwin

package jsonmaint

import (
	"os"

	"golang.org/x/sys/unix"
)

func exchange(parent *os.File, a, b string) error {
	return unix.RenameatxNp(int(parent.Fd()), a, int(parent.Fd()), b, unix.RENAME_SWAP)
}
