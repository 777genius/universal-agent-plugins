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
)

func (p terminalRenderer) confirm(ctx context.Context, req ConfirmRequest, queued []byte) (Confirmation, error) {
	accepted := req.Default && len(queued) == 0
	field := huh.NewConfirm().Title(req.Title).Affirmative("Yes").Negative("No").Inline(true).Value(&accepted)
	layout := &confirmationLayout{summary: strings.Join(req.Summary, "\n"), title: req.Title, viewport: viewport.New(viewport.WithWidth(76), viewport.WithHeight(10))}
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
}

func (l *confirmationLayout) resize(w, h int) error {
	titleHeight := lenLines(ansi.Wrap(l.title, max(1, w-4), ""))
	if w < 16 || h < titleHeight+6 {
		return fmt.Errorf("terminal too small for confirmation controls")
	}
	l.viewport.SetWidth(w)
	l.viewport.SetHeight(h - titleHeight - 5)
	l.viewport.SetContent(ansi.Wrap(l.summary, max(1, w-2), ""))
	return nil
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
	return lipgloss.JoinVertical(lipgloss.Left, l.viewport.View(), "↑/↓ PgUp/PgDn: review summary", base)
}

func (*confirmationLayout) GroupWidth(_ *huh.Form, _ *huh.Group, w int) int { return w }
func lenLines(s string) int                                                 { return strings.Count(s, "\n") + 1 }

func promptOutputTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
