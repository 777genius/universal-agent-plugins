package agentpluginscli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

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
	id    domain.ClientID
	label string
	step  groupProgressStep
	final string
}

// groupProgressBoard owns only terminal presentation. The use case emits
// observed checkpoints and remains independent of terminal control sequences.
type groupProgressBoard struct {
	mu      sync.Mutex
	writer  io.Writer
	theme   terminaltheme.Theme
	rows    []groupProgressRow
	width   int
	painted int
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
		if width := utf8.RuneCountInString(label); width > board.width {
			board.width = width
		}
		board.rows = append(board.rows, groupProgressRow{id: client.ClientID, label: label})
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
			next = progressConfiguring
		case usecase.GroupProgressConfigured:
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
	_, _ = fmt.Fprintln(board.writer)
}

func progressResultLabel(result usecase.AddResult) string {
	switch result.GroupPhase {
	case usecase.GroupTargetExternalCompleted:
		if result.Activation.Activation == domain.ActivationActive && result.Activation.Verification == domain.VerificationInstalled {
			return "installed"
		}
		return "setup required"
	case usecase.GroupTargetExternalFailed, usecase.GroupTargetManagedUnknown:
		return "failed"
	default:
		return "not completed"
	}
}

func (board *groupProgressBoard) redrawLocked() {
	if board.painted > 0 {
		_, _ = fmt.Fprintf(board.writer, "\033[%dA", board.painted)
	}
	for _, row := range board.rows {
		_, _ = fmt.Fprintf(board.writer, "\033[2K  %-*s  %s\n", board.width, row.label, progressPipeline(board.theme, row))
	}
	board.painted = len(board.rows)
}

func progressPipeline(theme terminaltheme.Theme, row groupProgressRow) string {
	labels := [3]string{"preparing", "configuring", "installing"}
	roles := [3]terminaltheme.Role{terminaltheme.Muted, terminaltheme.Muted, terminaltheme.Muted}
	switch row.step {
	case progressPreparing:
		roles[0] = terminaltheme.Label
	case progressConfiguring:
		roles[0], roles[1] = terminaltheme.Success, terminaltheme.Label
	case progressInstalling:
		roles[0], roles[1], roles[2] = terminaltheme.Success, terminaltheme.Success, terminaltheme.Label
	case progressFinished:
		roles[0], roles[1] = terminaltheme.Success, terminaltheme.Success
		labels[2] = row.final
		if labels[2] == "installed" {
			roles[2] = terminaltheme.Success
		} else if labels[2] == "failed" {
			roles[2] = terminaltheme.Error
		} else {
			roles[2] = terminaltheme.Warning
		}
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
	return (row == domain.ClientCopilot || row == domain.ClientVSCode) &&
		(event == domain.ClientCopilot || event == domain.ClientVSCode)
}
