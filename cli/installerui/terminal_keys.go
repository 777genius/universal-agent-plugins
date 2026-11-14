package installerui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

func promptKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc", "ctrl+d"), key.WithHelp("esc/ctrl+c/ctrl+d", "cancel"))
	// Consent requires Enter, even when a printable key is held or pasted.
	km.Confirm.Toggle = key.NewBinding(key.WithKeys("left", "right", "space"), key.WithHelp("←/→/space", "choose"))
	km.Confirm.Next = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit"))
	km.MultiSelect.Next = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit"))
	km.Confirm.Accept = key.NewBinding(key.WithDisabled())
	km.Confirm.Reject = key.NewBinding(key.WithDisabled())
	return km
}

type formInput struct {
	canSubmit func() bool
	queued    []byte
	resize    func(int, int) error
	onKey     func(tea.KeyPressMsg)
}
