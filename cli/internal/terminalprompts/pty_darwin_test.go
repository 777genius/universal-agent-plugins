//go:build darwin

package terminalprompts

import (
	"os"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func darwinHandoffPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { master.Close() })
	var name [128]byte
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0])))
	if errno != 0 {
		t.Fatal(errno)
	}
	for _, op := range []uint{unix.TIOCPTYGRANT, unix.TIOCPTYUNLK} {
		if err := unix.IoctlSetInt(int(master.Fd()), op, 0); err != nil {
			t.Fatal(err)
		}
	}
	slave, err := os.OpenFile(strings.TrimRight(string(name[:]), "\x00"), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { slave.Close() })
	state, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TIOCGETA)
	if err != nil {
		t.Fatal(err)
	}
	state.Lflag |= unix.ICANON | unix.ECHO | unix.ISIG
	state.Iflag |= unix.ICRNL
	state.Cc[unix.VEOF] = 4
	if err := unix.IoctlSetTermios(int(slave.Fd()), unix.TIOCSETA, state); err != nil {
		t.Fatal(err)
	}
	return master, slave
}
