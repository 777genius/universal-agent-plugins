//go:build darwin || linux

package agentpluginscli

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

// Passive activity must use the prompt's actual mode, not merely stderr TTY.
// Plain fallbacks must contain no controls, while qualified rich mode animates.
func TestCommandActivityUsesQualifiedPromptMode(t *testing.T) {
	for _, name := range []string{"rich", "term-unset", "tiny", "stdout-redirect", "plain"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("TERM", "xterm-256color")
			if name == "term-unset" {
				t.Setenv("TERM", "")
			}
			master, slave := commandActivityPTY(t)
			size := &unix.Winsize{Col: 100, Row: 30}
			if name == "tiny" {
				size = &unix.Winsize{Col: 32, Row: 6}
			}
			if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, size); err != nil {
				t.Fatal(err)
			}
			before, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			var redirected bytes.Buffer
			var output io.Writer = slave
			if name == "stdout-redirect" {
				output = &redirected
			}
			policy := &terminaltheme.Policy{Mode: "never", Explicit: true}
			format := "human"
			command := &cobra.Command{}
			command.SetIn(slave)
			command.SetOut(terminaltheme.Wrap(output, policy, &format))
			command.SetErr(terminaltheme.Wrap(slave, policy, &format))
			stop := startCommandActivity(command, App{Terminal: true}, &options{format: "human", plain: name == "plain"}, "Checking agents")
			stop()
			fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
			if n, err := unix.Poll(fds, 1000); err != nil || n == 0 {
				t.Fatalf("activity was not visible: ready=%d err=%v", n, err)
			}
			var buffer [4096]byte
			n, err := master.Read(buffer[:])
			if err != nil {
				t.Fatal(err)
			}
			view := string(buffer[:n])
			if !strings.Contains(view, "Checking agents") {
				t.Fatalf("missing activity label: %q", view)
			}
			if strings.Contains(view, "\x1b") != (name == "rich") {
				t.Fatalf("mode %s emitted wrong terminal controls: %q", name, view)
			}
			after, err := term.GetState(int(slave.Fd()))
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("passive activity changed terminal input mode: %v", err)
			}
		})
	}
}
