package installerui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"

	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

func (p terminalRenderer) confirm(ctx context.Context, req confirmationReview, queued []byte) (Confirmation, error) {
	accepted := req.Default && len(queued) == 0
	field := huh.NewConfirm().Title(req.Title).Affirmative("Yes").Negative("No").Inline(true).Value(&accepted)
	layout := &confirmationLayout{summary: strings.Join(req.Summary, "\n"), details: strings.Join(req.Details, "\n"), title: req.Title, noColor: p.NoColor, field: field, viewport: viewport.New(viewport.WithWidth(76), viewport.WithHeight(10))}
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
	summary, details, title string
	showDetails             bool
	width, height           int
	viewport                viewport.Model
	noColor                 bool
	framed                  bool
	field                   *huh.Confirm
}

const confirmationSummaryHelp = "↑/↓ PgUp/PgDn: review summary"

func (l *confirmationLayout) resize(w, h int) error {
	titleHeight := lenLines(ansi.Wrap(l.title, max(1, w-4), ""))
	hasReview := l.summary != "" || l.details != ""
	// Preserve the upstream framed review when Details is absent. The opt-in
	// review can use inline buttons on small terminals to keep paths and consent visible.
	framed := l.summary != ""
	if l.details != "" {
		framed = w >= 50 && h >= 18
	}
	reserved := titleHeight + 5
	if framed {
		reserved += 4 + lenLines(ansi.Wrap(l.controls(), max(1, w), ""))
	} else if l.details != "" {
		reserved = titleHeight + 4 + lenLines(ansi.Wrap(l.controls(), max(1, w), ""))
	}
	if w < 16 || h < reserved+1 {
		return fmt.Errorf("terminal too small for confirmation controls")
	}
	l.width, l.height, l.framed = w, h, framed
	if l.field != nil && hasReview {
		l.field.Inline(!framed).WithButtonAlignment(lipgloss.Left)
	}
	viewportWidth, contentWidth := w-4, w-4
	if l.details != "" && !framed {
		viewportWidth, contentWidth = w, w-2
	}
	l.viewport.SetWidth(viewportWidth)
	content := l.summary
	if l.showDetails {
		content = l.details
	}
	wrapped := formatConfirmationSummary(content, contentWidth, l.noColor)
	rows := h - reserved
	if l.details != "" {
		rows = min(rows, lenLines(wrapped))
	}
	l.viewport.SetHeight(rows)
	l.viewport.SetContent(wrapped)
	return nil
}

// Summary rows have already passed review normalization. Decoration never decodes
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
		if (blockStart || confirmationActionHeading(row)) && row != "" && row == strings.TrimLeft(row, " ") {
			wrapped = heading.Render(wrapped)
		}
		rows[i] = wrapped
		blockStart = row == ""
	}
	return strings.Join(rows, "\n")
}

func confirmationActionHeading(row string) bool {
	kind, _, found := strings.Cut(row, " ")
	if !found {
		return false
	}
	switch kind {
	case "install", "update", "remove", "unchanged":
		return true
	default:
		return false
	}
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
	if detailsToggleKey(msg) && l.details != "" {
		l.showDetails = !l.showDetails
		_ = l.resize(l.width, l.height)
		l.viewport.GotoTop()
		return
	}
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

// Keep the advertised physical D shortcut usable with Caps Lock, enhanced
// keyboard events, and the Russian/Ukrainian layout. Modified cancel keys such
// as Ctrl+D still belong to the form; held-key repeats do not toggle twice.
func detailsToggleKey(msg tea.KeyPressMsg) bool {
	if msg.IsRepeat || msg.Mod & ^(tea.ModShift|tea.ModCapsLock|tea.ModNumLock) != 0 {
		return false
	}
	for _, code := range []rune{msg.Code, msg.BaseCode} {
		if unicode.ToLower(code) == 'd' || unicode.ToLower(code) == 'в' {
			return true
		}
	}
	return strings.EqualFold(msg.Text, "d") || strings.EqualFold(msg.Text, "в")
}

func (l *confirmationLayout) View(f *huh.Form) string {
	base := huh.LayoutDefault.View(f)
	if l.summary == "" && l.details == "" {
		return base
	}
	content := l.viewport.View()
	help := lipgloss.NewStyle()
	if l.framed {
		frame := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
		if !l.noColor {
			frame = frame.BorderForeground(terminaltheme.Color(terminaltheme.Muted))
		}
		content = frame.Render(content)
	}
	if !l.noColor {
		help = help.Foreground(terminaltheme.Color(terminaltheme.Muted))
	}
	return lipgloss.JoinVertical(lipgloss.Left, content, help.Render(ansi.Wrap(l.controls(), l.width, "")), base)
}

func (l *confirmationLayout) controls() string {
	text := confirmationSummaryHelp
	if l.details != "" {
		if l.showDetails {
			return "↑/↓ PgUp/PgDn scroll · d hide paths"
		}
		return "↑/↓ PgUp/PgDn scroll · d show paths"
	}
	return text
}

func (*confirmationLayout) GroupWidth(_ *huh.Form, _ *huh.Group, w int) int { return w }
func lenLines(s string) int                                                 { return strings.Count(s, "\n") + 1 }

func promptOutputTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
