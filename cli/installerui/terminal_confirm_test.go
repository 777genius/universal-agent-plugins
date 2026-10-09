package installerui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/ansi"
)

// Losing spaces, escaped quotes or the end of an authority path during
// decoration must fail, even when the path wraps across many screen lines.
func TestConfirmationSummaryPreservesAuthorityText(t *testing.T) {
	row := `  "scope=/test/` + strings.Repeat(`folder with  spaces/`, 30) + `chosen-末尾\"quoted\""`
	for _, noColor := range []bool{false, true} {
		got := formatConfirmationSummary(row, 12, noColor)
		if restored := strings.ReplaceAll(ansi.Strip(got), "\n", ""); restored != row {
			t.Fatalf("authority changed (NoColor=%v): %q", noColor, restored)
		}
		for _, line := range strings.Split(got, "\n") {
			if ansi.StringWidth(line) > 12 {
				t.Fatalf("authority exceeds viewport: %q", line)
			}
		}
	}
	summary := "Fixture summary\n  action=install\n\nSelected units\n  unit=alpha"
	styled := formatConfirmationSummary(summary, 80, false)
	plain := formatConfirmationSummary(summary, 80, true)
	if ansi.Strip(styled) != summary || plain != summary || styled == plain {
		t.Fatalf("summary decoration or NoColor contract: styled=%q plain=%q", styled, plain)
	}
	for _, heading := range []string{"Fixture summary", "Selected units"} {
		for _, line := range strings.Split(styled, "\n") {
			if ansi.Strip(line) == heading && line == heading {
				t.Fatalf("section heading has no emphasis: %q", heading)
			}
		}
	}
}

// Adding a frame or changing size must not place controls outside the screen,
// hide the final authority suffix, or make the review hint overflow its width.
func TestConfirmationSummaryResizeAndScrollKeepsControlsVisible(t *testing.T) {
	accepted := false
	field := huh.NewConfirm().Title("Apply?").Affirmative("Yes").Negative("No").Inline(true).Value(&accepted)
	layout := &confirmationLayout{
		title: "Apply?", summary: "Fixture summary\n\nSelected units\n  " + strings.Repeat("scope/", 150) + "/chosen-final-suffix",
		noColor: true, viewport: viewport.New(),
	}
	form := huh.NewForm(huh.NewGroup(field)).WithLayout(layout)
	for _, size := range [][2]int{{80, 24}, {16, 16}, {32, 12}, {80, 24}} {
		w, h := size[0], size[1]
		if err := layout.resize(w, h); err != nil {
			t.Fatal(err)
		}
		_, _ = form.Update(tea.WindowSizeMsg{Width: w, Height: h})
		layout.key(tea.KeyPressMsg{Code: tea.KeyEnd})
		view := ansi.Strip(layout.View(form))
		if len(strings.Split(view, "\n")) > h {
			t.Fatalf("view exceeds %dx%d:\n%s", w, h, view)
		}
		var reviewText strings.Builder
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > w {
				t.Fatalf("view exceeds width %d: %q\n%s", w, line, view)
			}
			if strings.HasPrefix(line, "│ ") && strings.HasSuffix(line, " │") {
				reviewText.WriteString(strings.TrimRight(strings.TrimSuffix(strings.TrimPrefix(line, "│ "), " │"), " "))
			}
		}
		if !strings.Contains(reviewText.String(), "chosen-final-suffix") {
			t.Fatalf("end scroll clipped authority suffix:\n%s", view)
		}
		for _, control := range []string{"Apply?", "Yes", "No"} {
			if !strings.Contains(view, control) {
				t.Fatalf("control %q disappeared after resize to %dx%d:\n%s", control, w, h, view)
			}
		}
		if !strings.Contains(view, "╭") || !strings.Contains(view, "╰") {
			t.Fatalf("summary lacks visible frame:\n%s", view)
		}
	}
}
