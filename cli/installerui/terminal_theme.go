package installerui

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

func semanticHuhTheme(dark bool) *huh.Styles {
	t := huh.ThemeBase(dark)
	for _, f := range []*huh.FieldStyles{&t.Focused, &t.Blurred} {
		f.Title = f.Title.Foreground(terminaltheme.Color(terminaltheme.Label))
		f.Description = f.Description.Foreground(terminaltheme.Color(terminaltheme.Muted))
		f.ErrorIndicator = f.ErrorIndicator.Foreground(terminaltheme.Color(terminaltheme.Error))
		f.ErrorMessage = f.ErrorMessage.Foreground(terminaltheme.Color(terminaltheme.Error))
		f.SelectedPrefix = f.SelectedPrefix.Foreground(terminaltheme.Color(terminaltheme.Success))
		f.MultiSelectSelector = f.MultiSelectSelector.Foreground(terminaltheme.Color(terminaltheme.Label))
		// Huh uses FocusedButton for the selected confirmation choice.
		// Keep that choice visible when color is disabled or hard to distinguish.
		f.FocusedButton = f.FocusedButton.Foreground(lipgloss.Color("0")).SetString("✓")
		// Only the selected choice gets a background; the other stays plain text.
		f.BlurredButton = f.BlurredButton.UnsetBackground().UnsetForeground()
	}
	return t
}
