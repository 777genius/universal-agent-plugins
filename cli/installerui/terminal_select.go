package installerui

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

func (p terminalRenderer) selectMany(ctx context.Context, req MultiSelectRequest) (Selection, error) {
	ids := append([]string(nil), req.Defaults...)
	choices := make([]huh.Option[string], len(req.Options))
	for i, option := range req.Options {
		choices[i] = huh.NewOption(option.Label, option.ID)
	}
	validate := func(ids []string) error { _, err := terminalSelection(req, ids); return err }
	field := huh.NewMultiSelect[string]().Title(req.Title).Options(choices...).Value(&ids).Filterable(false).Validate(validate)
	layout := &selectionLayout{field: field, ids: &ids, validate: validate, title: req.Title, count: len(choices)}
	form := huh.NewForm(huh.NewGroup(field).WithShowHelp(false).WithShowErrors(false)).WithLayout(layout)
	layout.form = form
	_ = layout.resize(80, 24) // Private injected qualification has no terminal geometry.
	resize := layout.resize
	if err := p.initialSize(resize); err != nil {
		return Selection{}, err
	}
	err := p.run(ctx, form, formInput{canSubmit: func() bool { return validate(ids) == nil }, resize: resize})
	if _, canceled := err.(formCanceled); canceled { //nolint:errorlint // Only sole user cancellation is clean; wrapped cleanup errors must fail.
		return Selection{Cancelled: true}, nil //nolint:misspell // Preserve the existing public cancellation API.
	}
	if err != nil {
		return Selection{}, err
	}
	return terminalSelection(req, ids)
}

func (p terminalRenderer) initialSize(resize func(int, int) error) error {
	f, ok := p.Output.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return nil
	}
	w, h, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return err
	}
	return resize(w, h)
}

// Huh v2.0.3 changes viewport dimensions without bringing its hovered option
// back into view. An unbound key invokes its existing cursor visibility logic;
// preserve a failed validation instead of clearing it during the resize.
func resizeSelectionField(field *huh.MultiSelect[string], width, height int, ids []string, validate func([]string) error) {
	hadError := field.Error() != nil
	field.WithWidth(width).WithHeight(height)
	_, _ = field.Update(tea.KeyPressMsg{Code: tea.KeyF24})
	if hadError && validate(ids) != nil {
		_, _ = field.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	}
}

// Keep both validation and controls visible: Huh's default footer hides help
// whenever validation fails. Checkbox rendering and navigation remain Huh's.
type selectionLayout struct {
	field                     *huh.MultiSelect[string]
	form                      *huh.Form
	ids                       *[]string
	validate                  func([]string) error
	title                     string
	count, width, errorHeight int
}

const selectionControls = "↑/↓ move · space toggle · enter submit\nesc/ctrl+c/ctrl+d cancel"

func (l *selectionLayout) resize(w, h int) error {
	titleHeight := lenLines(ansi.Wrap(l.title, max(1, w-4), ""))
	l.width = w
	l.errorHeight = 1
	if err := l.validate(nil); err != nil {
		l.errorHeight = lenLines(ansi.Wrap(err.Error(), max(1, w), ""))
	}
	reserved := 1 + l.errorHeight + lenLines(ansi.Wrap(selectionControls, max(1, w), ""))
	if w < 16 || h < titleHeight+reserved+1 {
		return fmt.Errorf("terminal too small for selection controls")
	}
	l.form.WithHeight(h)
	resizeSelectionField(l.field, w, min(l.count+titleHeight, h-reserved), *l.ids, l.validate)
	return nil
}

func (l *selectionLayout) View(_ *huh.Form) string {
	text := ""
	if err := l.field.Error(); err != nil {
		text = ansi.Wrap(err.Error(), max(1, l.width), "")
	}
	errorRow := lipgloss.NewStyle().Height(l.errorHeight).Render(text)
	return lipgloss.JoinVertical(lipgloss.Left, l.field.View(), errorRow, ansi.Wrap(selectionControls, max(1, l.width), ""))
}
func (*selectionLayout) GroupWidth(_ *huh.Form, _ *huh.Group, w int) int { return w }
