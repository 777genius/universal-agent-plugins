package installerui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestConfirmationDetailsShareExistingAuthorityBudget(t *testing.T) {
	for _, req := range []confirmationReview{
		{ConfirmRequest: ConfirmRequest{Title: "Apply?", Summary: make([]string, 64)}, Details: make([]string, 65)},
		{ConfirmRequest: ConfirmRequest{Title: "Apply?"}, Details: []string{strings.Repeat("x", 4097)}},
		{ConfirmRequest: ConfirmRequest{Title: "Apply?"}, Details: []string{string([]byte{255})}},
		{ConfirmRequest: ConfirmRequest{Title: "Apply?", Summary: strings.Split(strings.Repeat(strings.Repeat("x", 4096)+"\n", 8), "\n")}, Details: strings.Split(strings.Repeat(strings.Repeat("x", 4096)+"\n", 8), "\n")},
	} {
		if _, err := normalizeConfirmationReview(req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("details bypassed authority budget: %v", err)
		}
	}
	path := strings.Repeat("x", 1000) + "/exact-destination"
	req := confirmationReview{ConfirmRequest: ConfirmRequest{Title: "Apply?", Summary: []string{"One Skill"}}, Details: []string{"\x1b" + path}}
	got, err := normalizeConfirmationReview(req)
	if err != nil || len(got.Details) != 1 || got.Details[0] != path {
		t.Fatalf("authority path was clipped or not cleaned: %+v %v", got, err)
	}
	got.Details[0] = "changed"
	if req.Details[0] != "\x1b"+path {
		t.Fatal("normalization mutated the caller's details")
	}
}

func TestConfirmationStylesEveryActionHeadingUniformly(t *testing.T) {
	rows := []string{"2 Skills · 5 agents · 10 bindings", "10 install", "", "install docx", "  → Claude Code, Codex", "install frontend-design", "  → Claude Code, Codex", "update pdf", "remove old", "unchanged keep"}
	summary := strings.Join(rows, "\n")
	styled := formatConfirmationSummary(summary, 100, false)
	if ansi.Strip(styled) != summary {
		t.Fatal("heading decoration changed review authority text")
	}
	for _, line := range strings.Split(styled, "\n") {
		if confirmationActionHeading(ansi.Strip(line)) && !strings.Contains(line, "\x1b[") {
			t.Fatalf("action heading was rendered without the shared label style: %q", line)
		}
	}
	if got := formatConfirmationSummary(summary, 100, true); strings.Contains(got, "\x1b[") || got != summary {
		t.Fatalf("no-color authority changed: %q", got)
	}
}

func TestDetailsShortcutHandlesKeyboardLayoutsWithoutStealingCancel(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{{Code: 'd'}, {Code: 'D'}, {Code: 'в'}, {Code: 'В'}, {Code: 'ф', BaseCode: 'd'}, {Code: 'd', Mod: tea.ModCapsLock}} {
		if !detailsToggleKey(key) {
			t.Fatalf("physical details shortcut unavailable: %+v", key)
		}
	}
	for _, key := range []tea.KeyPressMsg{{Code: 'd', Mod: tea.ModCtrl}, {Code: 'd', Mod: tea.ModAlt}, {Code: 'd', IsRepeat: true}, {Code: 'x'}} {
		if detailsToggleKey(key) {
			t.Fatalf("details shortcut captured a modified/repeated/unrelated key: %+v", key)
		}
	}
}

func TestPlainConfirmationDisplaysSummaryAndFullDetails(t *testing.T) {
	var output strings.Builder
	ui, err := New(Config{Input: strings.NewReader("n\n"), Output: &output})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ui.ConfirmWithDetails(context.Background(), ConfirmRequest{Title: "Apply?", Summary: []string{"One Skill for two agents"}, Default: true}, []string{"Claude: /exact/claude/path", "Codex: /exact/codex/path"})
	if err != nil || result.Accepted || !strings.Contains(output.String(), "One Skill for two agents\nClaude: /exact/claude/path\nCodex: /exact/codex/path\n") {
		t.Fatalf("plain authority details disappeared or changed consent: %+v %v %q", result, err, output.String())
	}
}
