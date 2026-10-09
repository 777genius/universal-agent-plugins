package installerui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"

	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

func (p terminalRenderer) confirm(ctx context.Context, req ConfirmRequest, queued []byte) (Confirmation, error) {
	accepted := req.Default && len(queued) == 0
	field := huh.NewConfirm().Title(req.Title).Affirmative("Yes").Negative("No").Inline(true).Value(&accepted)
	layout := &confirmationLayout{summary: strings.Join(req.Summary, "\n"), title: req.Title, noColor: p.NoColor, viewport: viewport.New(viewport.WithWidth(76), viewport.WithHeight(10))}
	_ = layout.resize(80, 24) // Private injected qualification has no terminal geometry.
	form := huh.NewForm(huh.NewGroup(field)).WithLayout(layout)
	if err := p.initialSize(layout.resize); err != nil {
		return Confirmation{}, err
	}
	// Injected nonterminal forms lack a stable question; queued gestures may also
	// complete before rendering. Keep one checked visible question in those cases.
	if len(queued) > 0 || !promptOutputTerminal(p.Output) {
		if err := promptio.WriteText(p.Output, req.Title+"\n"); err != nil {
			return Confirmation{}, err
		}
	}
	// The snapshot already decided decline or cancellation. Do not combine its
	// synthetic gesture with the next owner's still-queued terminal suffix.
	if len(queued) > 0 {
		if err := ctx.Err(); err != nil {
			return Confirmation{}, err
		}
		return Confirmation{Cancelled: queued[0] != '\r'}, nil //nolint:misspell // Public cancellation field.
	}
	err := p.run(ctx, form, formInput{queued: queued, resize: layout.resize, onKey: layout.key})
	if _, canceled := err.(formCanceled); canceled { //nolint:errorlint // Only sole user cancellation is clean; wrapped cleanup errors must fail.
		return Confirmation{Cancelled: true}, nil //nolint:misspell // Preserve the existing public cancellation API.
	}
	if err != nil {
		return Confirmation{}, err
	}
	return Confirmation{Accepted: accepted}, nil
}

type confirmationLayout struct {
	summary, title string
	viewport       viewport.Model
	noColor        bool
	width          int
}

const confirmationSummaryHelp = "↑/↓ PgUp/PgDn: review summary"

func (l *confirmationLayout) resize(w, h int) error {
	titleHeight := lenLines(ansi.Wrap(l.title, max(1, w-4), ""))
	reserved := titleHeight + 5
	if l.summary != "" {
		// The frame adds two rows. Wrap the hint as well, so narrow terminals
		// keep the confirmation buttons and help below the summary visible.
		reserved += 2 + lenLines(ansi.Wrap(confirmationSummaryHelp, max(1, w), ""))
	}
	if w < 16 || h < reserved+1 {
		return fmt.Errorf("terminal too small for confirmation controls")
	}
	l.width = w
	l.viewport.SetWidth(w - 4) // Two border cells and two padding cells.
	l.viewport.SetHeight(h - reserved)
	l.viewport.SetContent(formatConfirmationSummary(l.summary, w-4, l.noColor))
	return nil
}

// Summary rows have already passed normalizeConfirm. Decoration never decodes
// quoted authority values or replaces their text; wrapping retains every rune.
func formatConfirmationSummary(summary string, width int, noColor bool) string {
	heading := lipgloss.NewStyle()
	if !noColor {
		heading = heading.Bold(true).Foreground(terminaltheme.Color(terminaltheme.Label))
	}
	rows := strings.Split(summary, "\n")
	blockStart := true
	for i, row := range rows {
		wrapped := wrapConfirmationSummaryRow(row, width)
		if blockStart && row != "" && row == strings.TrimLeft(row, " ") {
			wrapped = heading.Render(wrapped)
		}
		rows[i] = wrapped
		blockStart = row == ""
	}
	return strings.Join(rows, "\n")
}

// Keep ordinary words together without discarding whitespace at line breaks.
// Oversized authority tokens still wrap fully, including their final suffix.
func wrapConfirmationSummaryRow(row string, width int) string {
	var result strings.Builder
	lineWidth := 0
	for _, token := range strings.SplitAfter(row, " ") {
		tokenWidth := ansi.StringWidth(token)
		if lineWidth > 0 && lineWidth+tokenWidth > width {
			result.WriteByte('\n')
			lineWidth = 0
		}
		wrapped := ansi.Hardwrap(token, width, true)
		result.WriteString(wrapped)
		if i := strings.LastIndexByte(wrapped, '\n'); i >= 0 {
			lineWidth = ansi.StringWidth(wrapped[i+1:])
		} else {
			lineWidth += tokenWidth
		}
	}
	return result.String()
}

func (l *confirmationLayout) key(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "up", "pgup":
		l.viewport.ScrollUp(max(1, l.viewport.Height()/2))
	case "down", "pgdown":
		l.viewport.ScrollDown(max(1, l.viewport.Height()/2))
	case "home":
		l.viewport.GotoTop()
	case "end":
		l.viewport.GotoBottom()
	}
}

func (l *confirmationLayout) View(f *huh.Form) string {
	base := huh.LayoutDefault.View(f)
	if l.summary == "" {
		return base
	}
	frame := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	help := lipgloss.NewStyle()
	if !l.noColor {
		frame = frame.BorderForeground(terminaltheme.Color(terminaltheme.Muted))
		help = help.Foreground(terminaltheme.Color(terminaltheme.Muted))
	}
	return lipgloss.JoinVertical(lipgloss.Left, frame.Render(l.viewport.View()), help.Render(ansi.Wrap(confirmationSummaryHelp, l.width, "")), base)
}

func (*confirmationLayout) GroupWidth(_ *huh.Form, _ *huh.Group, w int) int { return w }
func lenLines(s string) int                                                 { return strings.Count(s, "\n") + 1 }

func promptOutputTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
