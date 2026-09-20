//go:build linux

package terminalprompts

import (
	"fmt"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/sys/unix"
)

func TestFormWindowSizePTY(t *testing.T) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()
	var messages []tea.Msg
	want := []tea.WindowSizeMsg{{Width: 100, Height: 30}, {Width: 43, Height: 12}}
	for _, size := range want {
		if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: uint16(size.Height), Col: uint16(size.Width)}); err != nil {
			t.Fatal(err)
		}
		msg, err := formWindowSize(slave, tea.RequestWindowSize())
		if err != nil {
			t.Fatal(err)
		}
		messages = append(messages, msg)
	}
	// The query has completed before handoff. Bubble Tea receives only the
	// concrete dimensions, so processing the message cannot touch a closed FD.
	if err := slave.Close(); err != nil {
		t.Fatal(err)
	}
	for i, msg := range messages {
		if msg != want[i] {
			t.Errorf("resize %d: got %#v; want %#v", i, msg, want[i])
		}
	}
}
