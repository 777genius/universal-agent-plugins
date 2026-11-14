//go:build linux

package jsonmaint

import (
	"os"

	"golang.org/x/sys/unix"
)

func exchange(parent *os.File, a, b string) error {
	return unix.Renameat2(int(parent.Fd()), a, int(parent.Fd()), b, unix.RENAME_EXCHANGE)
}
