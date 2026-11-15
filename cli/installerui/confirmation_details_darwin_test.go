//go:build darwin

package installerui

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestConfirmationDetailsTogglePTY(t *testing.T) {
	for _, geometry := range []struct{ cols, rows uint16 }{{80, 24}, {40, 12}} {
		for _, outcome := range []string{"yes", "no", "cancel"} {
			t.Run(fmt.Sprintf("%dx%d/%s", geometry.cols, geometry.rows, outcome), func(t *testing.T) {
				t.Setenv("TERM", "xterm-256color")
				master, slave := darwinHandoffPTY(t)
				if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: geometry.cols, Row: geometry.rows}); err != nil {
					t.Fatal(err)
				}
				terminal, err := NewTerminal(TerminalConfig{Input: slave, Output: slave, Mode: ModeRich, NoColor: true})
				if err != nil {
					t.Fatal(err)
				}
				request := ConfirmRequest{Title: "Install these Skills?", Summary: []string{"Skills: canvas-design", "Agents: Claude, Codex", "Warning: trust required"}, Default: outcome != "yes"}
				details := make([]string, 0, 30)
				for i := range 30 {
					details = append(details, fmt.Sprintf("Action%02d /exact/agent/skill%02d", i, i))
				}
				screen := newTerminalScreen(int(geometry.rows), int(geometry.cols))
				phase := 0
				missingControls := false
				finish := filterablePTYDriver(master, screen, func(view string) {
					buttons := strings.Contains(view, "Yes") && strings.Contains(view, "No")
					switch phase {
					case 0:
						if buttons && strings.Contains(view, "d show paths") && strings.Contains(view, "canvas-design") {
							missingControls = !strings.Contains(view, "Warning: trust required") || strings.Contains(view, "/exact/agent/")
							phase = 1
							_, _ = io.WriteString(master, " d") // Set Yes/No once, then inspect without changing it.
						}
					case 1:
						if strings.Contains(view, "skill00") && strings.Contains(view, "d hide paths") {
							missingControls = missingControls || !buttons
							phase = 2
							_, _ = io.WriteString(master, "\x1b[F")
						}
					case 2:
						if strings.Contains(view, "skill29") && strings.Contains(view, "d hide paths") {
							missingControls = missingControls || !buttons
							phase = 3
							_, _ = io.WriteString(master, "d")
						}
					case 3:
						if strings.Contains(view, "canvas-design") && strings.Contains(view, "d show paths") {
							missingControls = missingControls || !buttons
							phase = 4
							_, _ = io.WriteString(master, "d")
						}
					case 4:
						if strings.Contains(view, "skill00") && strings.Contains(view, "d hide paths") && !strings.Contains(view, "skill29") {
							missingControls = missingControls || !buttons
							phase = 5
							keys := "\r"
							if outcome == "cancel" {
								keys = "\x03"
							}
							_, _ = io.WriteString(master, keys)
						}
					}
				})
				ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
				defer cancel()
				got, err := terminal.ConfirmWithDetails(ctx, request, details)
				transcript := finish()
				if err != nil || phase != 5 || missingControls || got.Accepted != (outcome == "yes") || got.Cancelled != (outcome == "cancel") { //nolint:misspell // Existing public field.
					t.Fatalf("details PTY result=%+v err=%v phase=%d missingControls=%t\nscreen=%s\nraw=%q", got, err, phase, missingControls, screen.text(), transcript)
				}
			})
		}
	}
}

func TestDetailsShortcutAfterSkillsAndAgentPromptsPTY(t *testing.T) {
	for _, geometry := range []struct{ cols, rows uint16 }{{80, 24}, {40, 12}} {
		for _, shortcut := range []struct{ name, keys string }{{"latin", "d"}, {"russian-layout", "в"}, {"caps", "D"}, {"enhanced", "\x1b[100u"}} {
			t.Run(fmt.Sprintf("%dx%d/%s", geometry.cols, geometry.rows, shortcut.name), func(t *testing.T) {
				t.Setenv("TERM", "xterm-256color")
				master, slave := darwinHandoffPTY(t)
				if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: geometry.cols, Row: geometry.rows}); err != nil {
					t.Fatal(err)
				}
				terminal, err := NewTerminal(TerminalConfig{Input: slave, Output: slave, Mode: ModeRich, NoColor: false})
				if err != nil {
					t.Fatal(err)
				}
				review := ConfirmRequest{Title: "Install these Skills?", Summary: []string{"2 Skills · 5 agents · 10 bindings", "10 install", "", "install docx", "  → Claude Code, Codex", "install frontend-design", "  → Claude Code, Codex"}}
				details := []string{"install docx → Claude Code (/project/claude/docx)", "install frontend-design → Codex (/project/codex/frontend-design)"}
				screen := newTerminalScreen(int(geometry.rows), int(geometry.cols))
				phase := 0
				finish := filterablePTYDriver(master, screen, func(view string) {
					switch phase {
					case 0:
						if strings.Contains(view, "Choose Skills") && strings.Contains(view, "submit") {
							phase = 1
							_, _ = io.WriteString(master, " \x1b[B \r")
						}
					case 1:
						if strings.Contains(view, "Choose agents") && strings.Contains(view, "submit") {
							phase = 2
							_, _ = io.WriteString(master, "\r")
						}
					case 2:
						if strings.Contains(view, "10 bindings") && strings.Contains(view, "d show paths") && strings.Contains(view, "Yes") {
							phase = 3
							_, _ = io.WriteString(master, shortcut.keys)
						}
					case 3:
						if strings.Contains(view, "/project/claude/docx") && strings.Contains(view, "d hide paths") {
							phase = 4
							_, _ = io.WriteString(master, shortcut.keys)
						}
					case 4:
						if strings.Contains(view, "10 bindings") && strings.Contains(view, "d show paths") {
							phase = 5
							_, _ = io.WriteString(master, "\r")
						}
					}
				})
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
				defer cancel()
				skills, err := terminal.SelectMany(ctx, MultiSelectRequest{SelectRequest: SelectRequest{Title: "Choose Skills", Options: []Option{{ID: "docx", Label: "docx"}, {ID: "frontend-design", Label: "frontend-design"}}}, MinSelected: 1, Filterable: true})
				if err != nil || !reflect.DeepEqual(skills.IDs, []string{"docx", "frontend-design"}) {
					t.Fatalf("Skills prompt: %+v %v", skills, err)
				}
				agents := []string{"claude", "codex", "gemini", "cursor", "opencode"}
				choices := make([]Option, len(agents))
				for i, agent := range agents {
					choices[i] = Option{ID: agent, Label: agent}
				}
				selectedAgents, err := terminal.SelectMany(ctx, MultiSelectRequest{SelectRequest: SelectRequest{Title: "Choose agents", Options: choices, Defaults: agents}, MinSelected: 1})
				if err != nil || !reflect.DeepEqual(selectedAgents.IDs, agents) {
					t.Fatalf("agent prompt: %+v %v", selectedAgents, err)
				}
				result, err := terminal.ConfirmWithDetails(ctx, review, details)
				transcript := finish()
				if err != nil || result.Accepted || phase != 5 {
					t.Fatalf("details shortcut after preceding prompts: %+v err=%v phase=%d screen=%s raw=%q", result, err, phase, screen.text(), transcript)
				}
			})
		}
	}
}
