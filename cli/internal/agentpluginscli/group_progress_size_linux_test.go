//go:build linux

package agentpluginscli

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/sys/unix"
)

func TestBoardPTYCellSizingAndResize(t *testing.T) {
	for _, tc := range []struct {
		name, label string
		cols, rows  int
		live        bool
	}{
		{"wide", "界界e\u0301", 100, 30, true}, {"narrow", "Cursor", 40, 30, false},
		{"long-wide", "界界界界界界界界界界界界界界界界界界界界", 80, 30, false},
		{"short-height", "Cursor", 100, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			master, slave := consentPTY(t)
			size := func(cols, rows int) {
				t.Helper()
				if e := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(cols), Row: uint16(rows)}); e != nil {
					t.Fatal(e)
				}
			}
			read := func() string {
				t.Helper()
				var out strings.Builder
				for {
					fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
					n, e := unix.Poll(fds, 20)
					if e == unix.EINTR {
						continue
					}
					if e != nil {
						t.Fatal(e)
					}
					if n == 0 {
						return out.String()
					}
					var b [4096]byte
					n, e = master.Read(b[:])
					if e != nil {
						t.Fatal(e)
					}
					out.Write(b[:n])
				}
			}
			size(tc.cols, tc.rows)
			board := newGroupProgressBoard(slave, []domain.DetectedClient{{ClientID: domain.ClientCursor, DisplayName: tc.label}, {ClientID: domain.ClientCodex, DisplayName: "abcde"}})
			initial := read()
			board.observe(usecase.GroupProgressEvent{ClientID: domain.ClientCursor, Phase: usecase.GroupProgressPreparing})
			next := read()
			if strings.Contains(next, "\x1b[2A") != tc.live {
				t.Errorf("live=%v: %q", tc.live, next)
			}
			if tc.live {
				lines := strings.Split(ansi.Strip(initial), "\r\n")
				col := func(line string) int {
					i := strings.Index(line, "· preparing")
					if i < 0 {
						t.Fatalf("missing pipeline: %q", line)
					}
					return ansi.StringWidth(line[:i])
				}
				if col(lines[0]) != col(lines[1]) {
					t.Errorf("display cells misaligned: %q", lines)
				}
			} else if strings.Contains(initial+next, "\x1b[") {
				t.Errorf("fallback moved cursor: %q", initial+next)
			}
			size(20, 6)
			board.observe(usecase.GroupProgressEvent{ClientID: domain.ClientCursor, Phase: usecase.GroupProgressConfiguring})
			shrunk := read()
			if strings.Contains(shrunk, "\x1b[") {
				t.Errorf("resize reused stale coordinates: %q", shrunk)
			}
			size(120, 30)
			board.finish(nil)
			if final := read(); strings.Contains(final, "\x1b[") {
				t.Errorf("fallback resumed stale cursor: %q", final)
			}
		})
	}
}
