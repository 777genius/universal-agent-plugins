package agentpluginscli

import (
	"fmt"
	"os"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminalactivity"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

func startCommandActivity(cmd *cobra.Command, app App, opts *options, label string) func() {
	if !app.Terminal || opts.format == "json" {
		return func() {}
	}
	writer := cmd.ErrOrStderr()
	live := !opts.plain && os.Getenv("TERM") != "dumb" && terminaltheme.IsTerminal(terminaltheme.Unwrap(writer))
	width := func() int {
		if file, ok := terminaltheme.Unwrap(writer).(*os.File); ok {
			if columns, _, err := term.GetSize(int(file.Fd())); err == nil {
				return columns
			}
		}
		return 80
	}
	theme := terminaltheme.For(writer)
	model := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	render := func(_ int, label, elapsed string, columns int) string {
		mark := model.View()
		model, _ = model.Update(spinner.TickMsg{})
		title := theme.Text(terminaltheme.Label, label)
		if theme.Enabled {
			title = lipgloss.NewStyle().Bold(true).Render(title)
		}
		line := fmt.Sprintf("%s %s %s", theme.Text(terminaltheme.Label, mark), title, theme.Text(terminaltheme.Muted, "· "+elapsed))
		return ansi.Truncate(line, columns-1, "…")
	}
	return terminalactivity.Start(writer, label, live, width, render)
}
