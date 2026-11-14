//go:build linux

package terminalprompts

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"golang.org/x/sys/unix"
)

func TestColorPerStreamAndVisiblePrompt(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 30, Col: 100}); err != nil {
		t.Fatal(err)
	}
	var redirected bytes.Buffer
	format := "human"
	policy := terminaltheme.Policy{}
	stdout := terminaltheme.Wrap(&redirected, &policy, &format)
	stderr := terminaltheme.Wrap(slave, &policy, &format)
	if terminaltheme.For(stdout).Enabled || !terminaltheme.For(stderr).Enabled {
		t.Fatal("streams share detection")
	}
	p, visible, err := New(slave, stdout, stderr, false, !terminaltheme.For(stderr).Enabled)
	if err != nil || visible != stderr {
		t.Fatal("lost visible review stream", err)
	}
	plain, ok := p.(PlainPrompter)
	if !ok || !terminaltheme.For(plain.Output).Enabled {
		t.Fatal("redirected stdout must retain colored plain stderr")
	}
	policy.Mode = "never"
	policy.Explicit = true
	p, visible, err = New(slave, stderr, stderr, false, !terminaltheme.For(stderr).Enabled)
	rich, ok := p.(HuhPrompter)
	if err != nil || !ok || !rich.NoColor || visible != stderr {
		t.Fatal("never must preserve rich interaction", err)
	}
}
