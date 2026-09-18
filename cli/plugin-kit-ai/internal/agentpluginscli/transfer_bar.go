package agentpluginscli

import (
	"fmt"
	"io"
	"math"
	"sync"

	"charm.land/bubbles/v2/progress"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

const transferBarWidth = 28

type transferBar struct {
	mu     sync.Mutex
	writer io.Writer
	theme  terminaltheme.Theme
	model  progress.Model
	label  string
	phase  string
	last   float64
	live   bool
}

func startTransferBar(app App, label string) *transferBar {
	if app.outputFormat() == "json" || !app.Terminal {
		return nil
	}
	writer := app.errorOutput()
	if !terminaltheme.IsTerminal(terminaltheme.Unwrap(writer)) {
		return nil
	}
	return newTransferBar(writer, label, true)
}

func newTransferBar(writer io.Writer, label string, live bool) *transferBar {
	model := progress.New(
		progress.WithWidth(transferBarWidth),
		progress.WithFillCharacters(progress.DefaultFullCharFullBlock, progress.DefaultEmptyCharBlock),
		progress.WithColors(terminaltheme.Color(terminaltheme.Label)),
	)
	model.EmptyColor = terminaltheme.Color(terminaltheme.Muted)
	bar := &transferBar{writer: writer, theme: terminaltheme.For(writer), model: model, label: label, live: live}
	bar.redraw()
	return bar
}

func (app App) outputFormat() string {
	if app.progressFormat == nil || *app.progressFormat == "" {
		return "human"
	}
	return *app.progressFormat
}

func (app App) bindGitHubTransfer(repository string) func(error) {
	bar := startTransferBar(app, "Downloading "+repository)
	binder, ok := app.SourceAcquirer.(sourceacquisition.ReporterBinder)
	if ok && bar != nil {
		binder.BindReporter(bar)
	}
	return func(err error) {
		if ok {
			binder.BindReporter(nil)
		}
		bar.Finish(err)
	}
}

func (bar *transferBar) Report(transfer sourceacquisition.Transfer) {
	if bar == nil {
		return
	}
	bar.mu.Lock()
	defer bar.mu.Unlock()
	bar.last = clampFraction(transfer.Fraction)
	bar.phase = transfer.Phase
	bar.redrawLocked()
}

func (bar *transferBar) Finish(err error) {
	if bar == nil {
		return
	}
	bar.mu.Lock()
	defer bar.mu.Unlock()
	if err == nil {
		bar.last = 1
		bar.phase = "done"
	}
	bar.redrawLocked()
	if bar.live {
		_, _ = fmt.Fprintln(bar.writer)
	}
}

func (bar *transferBar) redraw() {
	bar.mu.Lock()
	defer bar.mu.Unlock()
	bar.redrawLocked()
}

func (bar *transferBar) redrawLocked() {
	if !bar.live {
		return
	}
	_, _ = fmt.Fprintf(bar.writer, "\r\033[2K%s", renderTransferLine(bar.theme, bar.model, bar.label, bar.phase, bar.last))
}

func renderTransferLine(theme terminaltheme.Theme, model progress.Model, label, phase string, fraction float64) string {
	line := theme.Text(terminaltheme.Muted, prompt.SafeText(label)) + "  " + model.ViewAs(clampFraction(fraction))
	phase = prompt.SafeText(phase)
	if phase != "" && phase != "done" {
		line += "  " + theme.Text(terminaltheme.Muted, phase)
	}
	return line
}

func clampFraction(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
