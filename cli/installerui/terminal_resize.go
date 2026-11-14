package installerui

import (
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

// Bubble Tea v2.0.2 handles RequestWindowSize with an unjoined checkResize
// goroutine, which can call the inherited output's Fd after the form returns
// and its owner closes it. Resolve that command in the event loop instead.
// The library still owns initial sizing and its joined SIGWINCH listener.
// Do not cache Fd: that would leave late ioctls targeting a reused descriptor.
func formWindowSize(output io.Writer, msg tea.Msg) (tea.Msg, error) {
	if msg != tea.RequestWindowSize() {
		return msg, nil
	}
	f, ok := output.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return nil, nil // Like Bubble Tea, no size query for nonterminal output.
	}
	width, height, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return nil, err
	}
	return tea.WindowSizeMsg{Width: width, Height: height}, nil
}
