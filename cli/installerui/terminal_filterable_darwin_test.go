//go:build darwin

package installerui

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Exercise the public terminal adapter on a real owned PTY. The evidence screen
// consumes actual renderer escape sequences, not a substitute menu renderer.
func TestFilterableCompactViewportPTY(t *testing.T) {
	for _, geometry := range []struct{ cols, rows uint16 }{{80, 24}, {40, 12}} {
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
			options := make([]Option, 20)
			for i := range options {
				options[i] = Option{ID: fmt.Sprintf("skill-%02d", i), Label: fmt.Sprintf("Skill-%02d · %sEND-MARKER", i, strings.Repeat("世界/long/", 20))}
			}
			screen := newTerminalScreen(int(geometry.rows), int(geometry.cols))
			phase := 0
			initialIssue := ""
			initialView := ""
			finish := filterablePTYDriver(master, screen, func(view string) {
				switch phase {
				case 0:
					if strings.Contains(view, "Choose Skills to install") && strings.Contains(view, "submit") {
						initialView = view
						if strings.Contains(view, "select at least") || strings.Contains(view, "invalid terminal selection") {
							initialIssue = "validation was displayed before a submit"
						}
						if !strings.Contains(view, "> [ ] Skill-00") || strings.Contains(view, "END-MARKER") || strings.Count(view, "Skill-") > 8 {
							initialIssue = "cursor, clipping, or bounded viewport failed"
						}
						phase = 1
						_, _ = io.WriteString(master, "\r")
					}
				case 1:
					if strings.Contains(view, "invalid terminal selection") && strings.Contains(view, "submit") {
						phase = 2
						_, _ = io.WriteString(master, "\x1b[B /Skill-19\r")
					}
				case 2:
					if strings.Contains(view, "Skill-19") && strings.Contains(view, "space toggle") {
						phase = 3
						_, _ = io.WriteString(master, " \r")
					}
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			got, err := terminal.SelectMany(ctx, MultiSelectRequest{SelectRequest: SelectRequest{Title: "Choose Skills to install", Options: options}, MinSelected: 1, Filterable: true})
			transcript := finish()
			if err != nil || initialIssue != "" || phase != 3 || !reflect.DeepEqual(got.IDs, []string{"skill-01", "skill-19"}) {
				t.Fatalf("compact PTY: result=%+v err=%v phase=%d issue=%s\ninitial=%s\nfinal=%s\nraw=%q", got, err, phase, initialIssue, initialView, screen.text(), transcript)
			}
			t.Logf("real %dx%d initial screen:\n%s", geometry.cols, geometry.rows, initialView)
		})
	}
}

func filterablePTYDriver(master *os.File, screen *terminalTestScreen, step func(string)) func() string {
	joined := make(chan string, 1)
	go func() {
		var transcript strings.Builder
		var chunk [8192]byte
		for {
			n, err := master.Read(chunk[:])
			if n > 0 {
				transcript.Write(chunk[:n])
				screen.feed(string(chunk[:n]))
				step(screen.text())
			}
			if err != nil {
				joined <- transcript.String()
				return
			}
		}
	}()
	return func() string {
		_ = master.Close()
		return <-joined
	}
}
