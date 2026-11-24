//go:build darwin

package installerui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWrappedAnnotationPTY(t *testing.T) {
	for _, geometry := range []struct{ cols, rows uint16 }{{80, 24}, {44, 24}, {40, 12}, {44, 10}, {40, 10}} {
		t.Run(fmt.Sprintf("%dx%d", geometry.cols, geometry.rows), func(t *testing.T) {
			t.Setenv("TERM", "xterm-256color")
			master, slave := darwinHandoffPTY(t)
			if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: geometry.cols, Row: geometry.rows}); err != nil {
				t.Fatal(err)
			}
			terminal, err := NewTerminal(TerminalConfig{Input: slave, Output: slave, Mode: ModeRich, NoColor: true})
			if err != nil {
				t.Fatal(err)
			}
			screen := newTerminalScreen(int(geometry.rows), int(geometry.cols))
			phase := 0
			var evidence string
			finish := filterablePTYDriver(master, screen, func(view string) {
				if phase == 0 && strings.Contains(view, "submit") {
					phase = 1
					_, _ = io.WriteString(master, "\x1b[B")
				} else if phase == 1 && strings.Contains(view, "> [•] Kiro") && strings.Contains(view, "already") && strings.Contains(view, "your list") && strings.Contains(view, "manually;") && strings.Contains(view, "unchecked") && strings.Contains(view, "esc/ctrl+c/ctrl+d cancel") {
					phase = 2
					evidence = view
					_, _ = io.WriteString(master, "\r")
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			req := unavailableFixture()
			req.Annotations["kiro"] = Annotation{Note: "prepare only; already in your list", Info: "verify manually; kept even if unchecked"}
			got, err := terminal.SelectMany(ctx, req)
			raw := finish()
			if err != nil || phase != 2 || len(got.IDs) != 2 {
				t.Fatalf("annotation lost: result=%+v err=%v phase=%d\nscreen=%s\nraw=%q", got, err, phase, screen.text(), raw)
			}
			t.Logf("real terminal screen:\n%s", evidence)
		})
	}
}
