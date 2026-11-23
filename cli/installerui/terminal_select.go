package installerui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

func (p terminalRenderer) selectMany(ctx context.Context, req MultiSelectRequest) (Selection, error) {
	theme := terminaltheme.Theme{Enabled: !p.NoColor}
	ids, field, layout, form := newSelectionForm(req, theme)
	validate, attempted := layout.validate, layout.attempted
	_ = layout.resize(80, 24) // Private injected qualification has no terminal geometry.
	resize := layout.resize
	if err := p.initialSize(resize); err != nil {
		return Selection{}, err
	}
	// Enter while filtering only accepts the query; release the input handoff
	// gate so the user can continue selecting instead of treating it as submit.
	err := p.run(ctx, form, formInput{canSubmit: func() bool { return !field.GetFiltering() && validate(*ids) == nil }, resize: resize, onKey: func(key tea.KeyPressMsg) {
		resetEmptyFilterCursor(field)
		if key.String() == "enter" && !field.GetFiltering() {
			*attempted = true
		}
	}})
	if _, canceled := err.(formCanceled); canceled { //nolint:errorlint // Only sole user cancellation is clean; wrapped cleanup errors must fail.
		return Selection{Cancelled: true}, nil //nolint:misspell // Preserve the existing public cancellation API.
	}
	if err != nil {
		return Selection{}, err
	}
	return terminalSelection(req, *ids)
}

// newSelectionForm builds the Huh field and its layout for a normalized request.
// Annotations are rendered into option labels; Huh keeps option IDs as values.
func newSelectionForm(req MultiSelectRequest, theme terminaltheme.Theme) (*[]string, *huh.MultiSelect[string], *selectionLayout, *huh.Form) {
	ids := append([]string(nil), req.Defaults...)
	choices := make([]huh.Option[string], len(req.Options))
	labels := make([]string, len(req.Options))
	for i, option := range req.Options {
		label := annotatedLabel(option.Label, req.Annotations[option.ID], theme)
		labels[i] = label
		if req.Filterable {
			// Huh toggles by display key, so clipped labels need unique keys.
			label = fmt.Sprintf("%d. %s", i+1, label)
		}
		choices[i] = huh.NewOption(label, option.ID)
	}
	validate := func(ids []string) error { _, err := terminalSelection(req, ids); return err }
	attempted := !req.Filterable
	validateField := func(ids []string) error {
		if !attempted {
			return nil
		}
		return validate(ids)
	}
	field := huh.NewMultiSelect[string]().Title(req.Title).Options(choices...).Value(&ids).Filterable(req.Filterable).Validate(validateField)
	layout := &selectionLayout{field: field, ids: &ids, validate: validate, title: req.Title, count: len(choices), filterable: req.Filterable,
		heading: req.UnavailableHeading, unavailable: req.Unavailable, theme: theme, attempted: &attempted, labels: labels}
	// Clip searchable options; wrap ordinary labels before Huh computes
	// viewport offsets, so continuation rows stay visible.
	// Clip the rendered option, not its key: Huh uses full option keys for
	// filtering and selection identity, so truncating keys can select twins.
	field.WithTheme(huh.ThemeFunc(func(dark bool) *huh.Styles {
		styles := semanticHuhTheme(dark)
		for _, style := range []*huh.FieldStyles{&styles.Focused, &styles.Blurred} {
			columns := max(1, layout.width-style.Base.GetHorizontalFrameSize()-lipgloss.Width(style.MultiSelectSelector.String())-lipgloss.Width(style.SelectedPrefix.String()))
			if req.Filterable {
				style.SelectedOption = style.SelectedOption.MaxWidth(columns).MaxHeight(1)
				style.UnselectedOption = style.UnselectedOption.MaxWidth(columns).MaxHeight(1)
			} else {
				style.SelectedOption = style.SelectedOption.Width(columns)
				style.UnselectedOption = style.UnselectedOption.Width(columns)
			}
		}
		return styles
	}))
	form := huh.NewForm(huh.NewGroup(field).WithShowHelp(false).WithShowErrors(false)).WithLayout(layout)
	layout.form = form
	return &ids, field, layout, form
}

// Huh v2.0.3 leaves a negative cursor after Down on an empty filter. Clamp it
// before the next key restores matches; Up preserves the query and selections.
func resetEmptyFilterCursor(field *huh.MultiSelect[string]) {
	if !field.GetFiltering() {
		return
	}
	if _, hovered := field.Hovered(); hovered {
		return
	}
	_, _ = field.Update(tea.KeyPressMsg{Code: tea.KeyUp})
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
	filterable                bool
	attempted                 *bool
	// Explanations of unavailable clients, shown below the list. notes holds
	// the rows chosen for the current geometry (blank separator included).
	heading     string
	unavailable []Unavailable
	theme       terminaltheme.Theme
	notes       []string
	labels      []string
}

// selectionOptionPrefix covers the border, cursor and checkbox before a label.
const selectionOptionPrefix = 8

const selectionControls = "↑/↓ move · space toggle · enter submit\nesc/ctrl+c/ctrl+d cancel"

func (l *selectionLayout) controls() string {
	if l.filterable {
		if l.field.GetFiltering() {
			return "Type to search · enter apply filter\nesc/ctrl+c/ctrl+d cancel"
		}
		return "↑/↓ move · space toggle · / search · enter submit\nesc/ctrl+c/ctrl+d cancel"
	}
	return selectionControls
}

func (l *selectionLayout) resize(w, h int) error {
	titleHeight := lenLines(ansi.Wrap(l.title, max(1, w-4), ""))
	l.width = w
	l.errorHeight = 1
	if err := l.validate(nil); err != nil {
		l.errorHeight = lenLines(ansi.Wrap(err.Error(), max(1, w), ""))
	}
	reserved := 1 + l.errorHeight + lenLines(ansi.Wrap(l.controls(), max(1, w), ""))
	if w < 16 || h < titleHeight+reserved+1 {
		return fmt.Errorf("terminal too small for selection controls")
	}
	l.form.WithHeight(h)
	rows := l.count
	if l.filterable {
		rows = min(rows, 8)
	} else {
		rows = l.optionRows(w)
	}
	l.notes = l.chooseNotes(w, h-reserved-titleHeight, rows)
	reserved += len(l.notes)
	resizeSelectionField(l.field, w, min(rows+titleHeight, h-reserved), *l.ids, l.validate)
	return nil
}

// optionRows counts display rows: an annotated option longer than the line
// wraps in Huh and would otherwise be clipped by the field height.
func (l *selectionLayout) optionRows(width int) int {
	columns := max(1, width-selectionOptionPrefix)
	rows := 0
	for _, label := range l.labels {
		rows += lipgloss.Height(lipgloss.NewStyle().Width(columns).Render(label))
	}
	return rows
}

// chooseNotes picks the explanation block for the room left below the title and
// controls. The full block is used only when it fits without hiding options;
// otherwise one summary row is used while at least one option row remains.
func (l *selectionLayout) chooseNotes(width, room, rows int) []string {
	if len(l.unavailable) == 0 {
		return nil
	}
	full := append([]string{""}, unavailableLines(l.heading, l.unavailable, width, l.theme)...)
	if room-rows >= len(full) {
		return full
	}
	if room-1 >= 1 {
		return []string{ansi.Truncate(collapsedUnavailable(l.unavailable, l.theme), width, "…")}
	}
	return nil
}

func (l *selectionLayout) View(_ *huh.Form) string {
	text := ""
	if err := l.field.Error(); err != nil {
		text = ansi.Wrap(err.Error(), max(1, l.width), "")
	}
	errorRow := lipgloss.NewStyle().Height(l.errorHeight).Render(text)
	parts := []string{l.field.View()}
	if len(l.notes) > 0 {
		parts = append(parts, strings.Join(l.notes, "\n"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, append(parts, errorRow, ansi.Wrap(l.controls(), max(1, l.width), ""))...)
}
func (*selectionLayout) GroupWidth(_ *huh.Form, _ *huh.Group, w int) int { return w }
