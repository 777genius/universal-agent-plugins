package installerui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

func unavailableFixture() MultiSelectRequest {
	return MultiSelectRequest{
		SelectRequest: SelectRequest{
			Title:    "Choose targets",
			Options:  []Option{{ID: "codex", Label: "OpenAI Codex (codex)"}, {ID: "kiro", Label: "Kiro (kiro)"}},
			Defaults: []string{"codex", "kiro"},
		},
		MinSelected:        1,
		Annotations:        map[string]Annotation{"kiro": {Note: "prepare only", Info: "verify manually"}},
		UnavailableHeading: "Not available for automatic install",
		Unavailable: []Unavailable{
			{Names: []string{"ChatGPT", "Visual Studio Code"}, Detail: "cannot be recorded in the dependency list; install it separately with --no-save"},
			{Names: []string{"Hermes Agent"}, Detail: "not found on this computer"},
		},
	}
}

func viewRows(t *testing.T, req MultiSelectRequest, width, height int) []string {
	t.Helper()
	req, err := normalizeMulti(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _, layout, _ := newSelectionForm(req, terminaltheme.Theme{})
	if err := layout.resize(width, height); err != nil {
		t.Fatal(err)
	}
	return strings.Split(ansi.Strip(layout.View(nil)), "\n")
}

func rowIndex(rows []string, needle string) int {
	for i, row := range rows {
		if strings.Contains(row, needle) {
			return i
		}
	}
	return -1
}

func TestUnavailableBlockSitsBelowListWithoutCheckboxes(t *testing.T) {
	rows := viewRows(t, unavailableFixture(), 100, 24)
	list, heading, controls := rowIndex(rows, "Kiro (kiro) (prepare only) - verify manually"), rowIndex(rows, "Not available for automatic install:"), rowIndex(rows, "↑/↓ move")
	if list < 0 || heading <= list || controls <= heading {
		t.Fatalf("order list=%d heading=%d controls=%d:\n%s", list, heading, controls, strings.Join(rows, "\n"))
	}
	for _, row := range rows[heading:controls] {
		if strings.ContainsAny(row, "✓•>") || strings.Contains(row, "[ ]") {
			t.Fatalf("an unavailable client looks selectable: %q", row)
		}
	}
	if rowIndex(rows, "Hermes Agent: not found on this computer") < 0 {
		t.Fatalf("missing grouped explanation:\n%s", strings.Join(rows, "\n"))
	}
}

func TestUnavailableBlockWrapsWithoutBreakingWords(t *testing.T) {
	const width = 40
	rows := viewRows(t, unavailableFixture(), width, 24)
	text := make([]string, 0, len(rows))
	for _, row := range rows {
		if len([]rune(strings.TrimRight(row, " "))) > width {
			t.Fatalf("row wider than the terminal: %q", row)
		}
		text = append(text, strings.TrimSpace(row))
	}
	joined := strings.Join(text, " ")
	for _, word := range []string{"recorded", "dependency", "separately", "--no-save", "Visual", "Studio"} {
		if !strings.Contains(joined, word) {
			t.Fatalf("word %q was broken or lost:\n%s", word, strings.Join(rows, "\n"))
		}
	}
}

func TestUnavailableBlockCollapsesOnShortTerminalsAndKeepsOptions(t *testing.T) {
	// Room for the options and controls but not the full block.
	rows := viewRows(t, unavailableFixture(), 80, 9)
	if rowIndex(rows, "3 not available (terminal too short to show why)") < 0 || rowIndex(rows, "Not available for automatic install:") >= 0 {
		t.Fatalf("expected the one-line summary:\n%s", strings.Join(rows, "\n"))
	}
	for _, option := range []string{"OpenAI Codex", "Kiro (kiro)"} {
		if rowIndex(rows, option) < 0 {
			t.Fatalf("option %q hidden by the summary:\n%s", option, strings.Join(rows, "\n"))
		}
	}
	// Barely enough for one option: no explanation row may take its place.
	rows = viewRows(t, unavailableFixture(), 80, 6)
	if rowIndex(rows, "not available") >= 0 || rowIndex(rows, "OpenAI Codex") < 0 {
		t.Fatalf("explanations displaced the options:\n%s", strings.Join(rows, "\n"))
	}
}

func TestUnavailableAndAnnotationTextIsSanitized(t *testing.T) {
	req := unavailableFixture()
	req.Annotations["kiro"] = Annotation{Note: "prepare\x1b[31m only\u202e", Info: "verify\x00 manually"}
	req.Unavailable[1] = Unavailable{Names: []string{"Hermes\x1b Agent"}, Detail: "gone\u202e"}
	got, err := normalizeMulti(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{got.Annotations["kiro"].Note, got.Annotations["kiro"].Info, got.Unavailable[1].Names[0], got.Unavailable[1].Detail} {
		if strings.ContainsAny(text, "\x1b\x00\u202e") {
			t.Fatalf("unsafe display text survived: %q", text)
		}
	}
	bad := unavailableFixture()
	bad.Annotations["missing"] = Annotation{Note: "x"}
	if _, err := normalizeMulti(bad); err == nil {
		t.Fatal("annotation for an unknown option accepted")
	}
	bad = unavailableFixture()
	bad.Unavailable = append(bad.Unavailable, Unavailable{Names: nil, Detail: "x"})
	if _, err := normalizeMulti(bad); err == nil {
		t.Fatal("unavailable entry without names accepted")
	}
}

func TestPlainSelectionPrintsExplanationsBetweenListAndPrompt(t *testing.T) {
	var out strings.Builder
	u, err := New(Config{Input: strings.NewReader("\n"), Output: &out})
	if err != nil {
		t.Fatal(err)
	}
	got, err := u.SelectManyWithNotes(context.Background(), unavailableFixture(), false)
	if err != nil || len(got.IDs) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	text := out.String()
	list, notes, prompt := strings.Index(text, "Kiro (kiro) (prepare only) - verify manually"), strings.Index(text, "Not available for automatic install:"), strings.Index(text, "Choose one or more")
	if list < 0 || notes <= list || prompt <= notes {
		t.Fatalf("plain order list=%d notes=%d prompt=%d:\n%s", list, notes, prompt, text)
	}
}

func TestPlainSelectionWithNotesEnforcesMinimum(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		minimum     int
		defaults    []string
		invalid     bool
		canceled    bool
	}{
		{name: "too few", input: "1\n", minimum: 2, invalid: true},
		{name: "empty enter", input: "\n", minimum: 1, invalid: true},
		{name: "valid explicit", input: "1,2\n", minimum: 2},
		{name: "valid defaults", input: "\n", minimum: 2, defaults: []string{"codex", "kiro"}},
		{name: "optional empty", input: "\n"},
		{name: "cancel", input: "q\n", minimum: 2, canceled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := unavailableFixture()
			req.MinSelected, req.Defaults = tc.minimum, tc.defaults
			var out strings.Builder
			u, err := New(Config{Input: strings.NewReader(tc.input), Output: &out})
			if err != nil {
				t.Fatal(err)
			}
			got, err := u.SelectManyWithNotes(context.Background(), req, false)
			if tc.invalid {
				if !errors.Is(err, ErrInvalidSelection) || got.Accepted {
					t.Fatalf("below-minimum selection accepted: %+v, %v", got, err)
				}
				return
			}
			if err != nil || got.Cancelled != tc.canceled || got.Accepted == tc.canceled { //nolint:misspell // Existing public result field.
				t.Fatalf("selection: %+v, %v", got, err)
			}
		})
	}
}

func TestAnnotatedOptionRowsMatchRenderedWidthBoundary(t *testing.T) {
	req := unavailableFixture()
	req.Options, req.Defaults = req.Options[1:], []string{"kiro"}
	req.Unavailable = nil
	_, _, layout, _ := newSelectionForm(req, terminaltheme.Theme{})
	if got := layout.optionRows(51); got != 2 {
		t.Fatalf("annotated option rows = %d, want 2", got)
	}
	rows := viewRows(t, req, 51, 24)
	if rowIndex(rows, "manually") < 0 {
		t.Fatalf("annotation clipped:\n%s", strings.Join(rows, "\n"))
	}
	layout.labels = []string{"Kiro (kiro)"}
	if got := layout.optionRows(51); got != 1 {
		t.Fatalf("short option rows = %d, want 1", got)
	}
}
