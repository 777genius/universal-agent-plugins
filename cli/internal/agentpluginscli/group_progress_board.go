package agentpluginscli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type groupProgressStep uint8

const (
	progressQueued groupProgressStep = iota
	progressPreparing
	progressConfiguring
	progressInstalling
	progressFinished
)

type groupProgressRow struct {
	id                   domain.ClientID
	label                string
	step                 groupProgressStep
	final                string
	prepared, configured bool
}

// groupProgressBoard owns only terminal presentation. The use case emits
// observed checkpoints and remains independent of terminal control sequences.
type groupProgressBoard struct {
	mu              sync.Mutex
	writer          io.Writer
	theme           terminaltheme.Theme
	rows            []groupProgressRow
	width           int
	painted         int
	columns, height int
	live            bool
	resized         func() bool
	stopResize      func()
}

func startGroupProgressBoard(app App, opts *options, selected []domain.DetectedClient) *groupProgressBoard {
	if opts.format != "human" || opts.plain || !app.Terminal || len(selected) < 2 || os.Getenv("TERM") == "dumb" {
		return nil
	}
	writer := app.errorOutput()
	if !terminaltheme.IsTerminal(terminaltheme.Unwrap(writer)) {
		return nil
	}
	return newGroupProgressBoard(writer, selected)
}

func newGroupProgressBoard(writer io.Writer, selected []domain.DetectedClient) *groupProgressBoard {
	board := &groupProgressBoard{writer: writer, theme: terminaltheme.For(writer)}
	for _, client := range selected {
		label := prompt.SafeText(clientDisplayName(client))
		if width := ansi.StringWidth(label); width > board.width {
			board.width = width
		}
		board.rows = append(board.rows, groupProgressRow{id: client.ClientID, label: label})
	}
	board.columns, board.height = board.terminalSize()
	board.live = board.fits(board.columns, board.height)
	if board.live {
		board.resized, board.stopResize = watchProgressResize()
	}
	board.redrawLocked()
	return board
}

func (board *groupProgressBoard) observe(event usecase.GroupProgressEvent) {
	board.mu.Lock()
	defer board.mu.Unlock()
	changed := false
	for index := range board.rows {
		row := &board.rows[index]
		if !progressRowMatches(row.id, event.ClientID) || row.step == progressFinished {
			continue
		}
		var next groupProgressStep
		switch event.Phase {
		case usecase.GroupProgressPreparing:
			next = progressPreparing
		case usecase.GroupProgressConfiguring:
			row.prepared = true
			next = progressConfiguring
		case usecase.GroupProgressConfigured:
			row.configured = true
			next = progressInstalling
		case usecase.GroupProgressActivated:
			next = progressFinished
			row.final = progressResultLabel(event.Result)
		default:
			continue
		}
		if next > row.step {
			row.step = next
			changed = true
		}
	}
	if changed {
		board.redrawLocked()
	}
}

func (board *groupProgressBoard) finish(results []usecase.AddResult) {
	if board == nil {
		return
	}
	board.mu.Lock()
	defer board.mu.Unlock()
	for index := range board.rows {
		row := &board.rows[index]
		if index < len(results) {
			row.final = progressResultLabel(results[index])
		} else if row.step != progressFinished {
			row.final = "not completed"
		}
		row.step = progressFinished
	}
	board.redrawLocked()
	board.painted = 0
	if board.stopResize != nil {
		board.stopResize()
	}
	_, _ = fmt.Fprintln(board.writer)
}

func progressResultLabel(result usecase.AddResult) string {
	return strings.ToLower(string(classifyBatchPresentation(addTargetResult{
		Status: string(result.GroupPhase), Error: result.Failure,
		Output: addResultData{Result: result},
	})))
}

func (board *groupProgressBoard) terminalSize() (int, int) {
	if f, ok := terminaltheme.Unwrap(board.writer).(*os.File); ok {
		if width, height, err := term.GetSize(int(f.Fd())); err == nil {
			return width, height
		}
	}
	return 0, 0
}

func (board *groupProgressBoard) fits(columns, height int) bool {
	// Reserve the final column to avoid autowrap, and a row for the cursor.
	// Account for the widest possible final status before the first paint.
	widest := 0
	for _, final := range []string{"installed", "setup required", "sign-in required", "user-attested", "not completed", "rolled back", "failed"} {
		row := groupProgressRow{step: progressFinished, final: final}
		widest = max(widest, ansi.StringWidth(progressPipeline(terminaltheme.Theme{}, row)))
	}
	return columns > 4+board.width+widest && height > len(board.rows)
}

func (board *groupProgressBoard) redrawLocked() {
	columns, height := board.terminalSize()
	resized := board.resized != nil && board.resized()
	if board.live && (resized || columns != board.columns || height != board.height || !board.fits(columns, height)) {
		// A terminal can reflow old rows on resize. Their physical positions are
		// no longer knowable; append plain progress for the rest of this board.
		board.live, board.painted = false, 0
		_, _ = fmt.Fprint(board.writer, "\r\n")
	}
	var frame strings.Builder
	if board.live && board.painted > 0 {
		fmt.Fprintf(&frame, "\r\033[%dA", board.painted)
	}
	for _, row := range board.rows {
		if board.live {
			frame.WriteString("\r\033[2K")
		}
		padding := strings.Repeat(" ", board.width-ansi.StringWidth(row.label))
		fmt.Fprintf(&frame, "  %s%s  %s\n", row.label, padding, progressPipeline(board.theme, row))
	}
	if board.columns > 0 {
		frame.WriteString("\r")
	}
	if n, err := io.WriteString(board.writer, frame.String()); err != nil || n != frame.Len() {
		board.live, board.painted = false, 0
		return
	}
	if board.live {
		board.painted = len(board.rows)
	}
}

func progressPipeline(theme terminaltheme.Theme, row groupProgressRow) string {
	labels := [3]string{"preparing", "configuring", "installing"}
	roles := [3]terminaltheme.Role{terminaltheme.Muted, terminaltheme.Muted, terminaltheme.Muted}
	switch row.step {
	case progressPreparing:
		roles[0] = terminaltheme.Label
	case progressConfiguring:
		roles[1] = terminaltheme.Label
	case progressInstalling:
		roles[2] = terminaltheme.Label
	case progressFinished:
		labels[2] = row.final
		switch labels[2] {
		case "installed":
			roles[2] = terminaltheme.Success
		case "failed":
			roles[2] = terminaltheme.Error
		default:
			roles[2] = terminaltheme.Warning
		}
	}
	if row.prepared {
		roles[0] = terminaltheme.Success
	}
	if row.configured {
		roles[1] = terminaltheme.Success
	}
	arrow := theme.Text(terminaltheme.Muted, " → ")
	parts := make([]string, len(labels))
	for index := range labels {
		mark := "· "
		switch roles[index] {
		case terminaltheme.Label:
			mark = "● "
		case terminaltheme.Success:
			mark = "✓ "
		case terminaltheme.Warning:
			mark = "! "
		case terminaltheme.Error:
			mark = "× "
		}
		parts[index] = theme.Text(roles[index], mark+labels[index])
	}
	return strings.Join(parts, arrow)
}

func progressRowMatches(row, event domain.ClientID) bool {
	if row == event {
		return true
	}
	return domain.SharesBackend(row, event)
}
