//go:build darwin || linux

package agentpluginscli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Activity and progress must use the prompt's actual mode, not merely stderr TTY.
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
			app := App{Terminal: true, Input: slave, Output: command.OutOrStdout(), ErrorOutput: command.ErrOrStderr()}
			opts := &options{format: "human", plain: name == "plain"}
			stop := startCommandActivity(command, app, opts, "Checking agents")
			stop()
			checkCommandProgressBoards(t, app, opts, name == "rich")
			view := readCommandProgressOutput(t, master)
			if !strings.Contains(view, "Checking agents") || !strings.Contains(view, "Codex") {
				t.Fatalf("missing activity or progress label: %q", view)
			}
			if strings.Contains(view, "\x1b") != (name == "rich") {
				t.Fatalf("mode %s emitted wrong terminal controls: %q", name, view)
			}
			after, err := term.GetState(int(slave.Fd()))
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("passive activity or progress changed terminal input mode: %v", err)
			}
		})
	}
}

func checkCommandProgressBoards(t *testing.T, app App, opts *options, rich bool) {
	t.Helper()
	group := startGroupProgressBoard(app, opts, []domain.DetectedClient{{ClientID: domain.ClientCodex, DisplayName: "Codex"}})
	group.finish(nil)
	if (group != nil) != rich {
		t.Fatalf("group progress did not follow the prompt mode: board=%v rich=%v", group != nil, rich)
	}
	skills := startSkillsProgressBoard(app, opts, []SkillsAction{{Alias: "example", ClientID: string(domain.ClientCodex), Action: "install"}})
	if skills == nil {
		t.Fatal("Skills progress is missing")
	}
	defer skills.finish(SkillsBatchResult{})
	if skills.board.live != rich {
		t.Fatalf("Skills progress did not follow the prompt mode: live=%v rich=%v", skills.board.live, rich)
	}
}

// All renderers are joined before capture, so drain every available frame.
func readCommandProgressOutput(t *testing.T, master *os.File) string {
	t.Helper()
	fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
	var view strings.Builder
	for timeout := 1000; ; timeout = 0 {
		n, err := unix.Poll(fds, timeout)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return view.String()
		}
		var buffer [4096]byte
		n, err = master.Read(buffer[:])
		if err != nil {
			t.Fatal(err)
		}
		_, _ = view.Write(buffer[:n])
	}
}
