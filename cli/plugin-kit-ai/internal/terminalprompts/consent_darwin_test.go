//go:build darwin

package terminalprompts

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

const consentGetTermios = unix.TIOCGETA

func consentPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, slave := darwinHandoffPTY(t)
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 30, Col: 100}); err != nil {
		t.Fatal(err)
	}
	return master, slave
}
