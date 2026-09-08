package terminalprompts

import (
	"os"

	"golang.org/x/sys/unix"
)

func queuedInputBytes(f *os.File) (int, error) {
	return unix.IoctlGetInt(int(f.Fd()), unix.TIOCINQ)
}
