package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type groupProgressStep string

const (
	groupProgressQueued     groupProgressStep = "queued"
	groupProgressStaging    groupProgressStep = "copying"
	groupProgressCopied     groupProgressStep = "copied"
	groupProgressActivating groupProgressStep = "installing"
	groupProgressDone       groupProgressStep = "done"
	groupProgressFailed     groupProgressStep = "failed"
	groupProgressPending    groupProgressStep = "…"
	progressArrow                             = " → "
	lastProgressWidth                         = 10
)

type progressMark int

const (
	markPending progressMark = iota
	markCurrent
	markDone
	markFailed
)

type groupProgressRow struct {
	id     domain.ClientID
	label  string
	step   groupProgressStep
	failed bool
}

type groupProgressBoard struct {
	mu      sync.Mutex
	writer  io.Writer
	theme   terminaltheme.Theme
	rows    []groupProgressRow
	width   int
	painted int
	live    bool
}

type progressStager struct {
	ports.PackageStager
	board *groupProgressBoard
}

type progressActivator struct {
	ports.ClientActivator
	board *groupProgressBoard
}

func startGroupProgressBoard(app App, format string, selected []domain.DetectedClient) *groupProgressBoard {
	if format == "json" || !app.Terminal || len(selected) < 2 {
		return nil
	}
	writer := app.errorOutput()
	if !terminaltheme.IsTerminal(terminaltheme.Unwrap(writer)) {
		return nil
	}
	return newGroupProgressBoard(writer, selected, true)
}

func newGroupProgressBoard(writer io.Writer, selected []domain.DetectedClient, live bool) *groupProgressBoard {
	board := &groupProgressBoard{writer: writer, theme: terminaltheme.For(writer), live: live}
	for _, client := range selected {
		label := clientDisplayName(client)
		if utf8.RuneCountInString(label) > board.width {
			board.width = utf8.RuneCountInString(label)
		}
		board.rows = append(board.rows, groupProgressRow{id: client.ClientID, label: label, step: groupProgressQueued})
	}
	board.redraw()
	return board
}

func (board *groupProgressBoard) decorate(service usecase.Service) usecase.Service {
	if board == nil {
		return service
	}
	if service.Stager != nil {
		service.Stager = progressStager{PackageStager: service.Stager, board: board}
	}
	if service.Activator != nil {
		service.Activator = progressActivator{ClientActivator: service.Activator, board: board}
	}
	return service
}

func (board *groupProgressBoard) finish() {
	if board == nil || !board.live {
		return
	}
	board.mu.Lock()
	board.painted = 0
	board.mu.Unlock()
	_, _ = fmt.Fprintln(board.writer)
}

func (board *groupProgressBoard) set(id domain.ClientID, step groupProgressStep) {
	if board == nil {
		return
	}
	board.mu.Lock()
	defer board.mu.Unlock()
	changed := false
	for index := range board.rows {
		if !progressRowMatches(board.rows[index].id, id) {
			continue
		}
		if step == groupProgressFailed {
			if board.rows[index].failed {
				continue
			}
			board.rows[index].failed = true
			changed = true
			continue
		}
		if board.rows[index].step == step && !board.rows[index].failed {
			continue
		}
		board.rows[index].step = step
		board.rows[index].failed = false
		changed = true
	}
	if changed {
		board.redrawLocked()
	}
}

func (board *groupProgressBoard) redraw() {
	if board == nil {
		return
	}
	board.mu.Lock()
	defer board.mu.Unlock()
	board.redrawLocked()
}

func (board *groupProgressBoard) redrawLocked() {
	if !board.live {
		return
	}
	if board.painted > 0 {
		_, _ = fmt.Fprintf(board.writer, "\033[%dA", board.painted)
	}
	for _, row := range board.rows {
		_, _ = fmt.Fprintf(board.writer, "\033[2K  %-*s  %s\n", board.width, prompt.SafeText(row.label), progressPipeline(board.theme, row))
	}
	board.painted = len(board.rows)
}

func (stager progressStager) Stage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints) (domain.StagedDelivery, error) {
	return stager.trackStage(plan.ClientID, func() (domain.StagedDelivery, error) {
		return stager.PackageStager.Stage(ctx, envelope, plan, operationID, hints)
	})
}

func (stager progressStager) StageWithPluginData(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints, pluginDataPath string) (domain.StagedDelivery, error) {
	aware, ok := stager.PackageStager.(ports.PluginDataAwareStager)
	if !ok {
		return domain.StagedDelivery{}, fmt.Errorf("package stager cannot bind the owned PLUGIN_DATA locator")
	}
	return stager.trackStage(plan.ClientID, func() (domain.StagedDelivery, error) {
		return aware.StageWithPluginData(ctx, envelope, plan, operationID, hints, pluginDataPath)
	})
}

func (stager progressStager) trackStage(client domain.ClientID, stage func() (domain.StagedDelivery, error)) (domain.StagedDelivery, error) {
	stager.board.set(client, groupProgressStaging)
	delivery, err := stage()
	if err != nil {
		stager.board.set(client, groupProgressFailed)
		return delivery, err
	}
	stager.board.set(client, groupProgressCopied)
	return delivery, nil
}

func (activator progressActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	activator.board.set(request.Client.ClientID, groupProgressActivating)
	outcome, err := activator.ClientActivator.Activate(ctx, request)
	if err != nil || outcome.Activation == domain.ActivationFailed {
		activator.board.set(request.Client.ClientID, groupProgressFailed)
		return outcome, err
	}
	activator.board.set(request.Client.ClientID, groupProgressDone)
	return outcome, err
}

func progressPipeline(theme terminaltheme.Theme, row groupProgressRow) string {
	copyMark, copiedMark, lastMark := markPending, markPending, markPending
	last := string(groupProgressPending)
	switch row.step {
	case groupProgressStaging:
		copyMark = markCurrent
	case groupProgressCopied:
		copyMark, copiedMark = markDone, markCurrent
	case groupProgressActivating:
		copyMark, copiedMark, lastMark = markDone, markDone, markCurrent
		last = string(groupProgressActivating)
	case groupProgressDone:
		copyMark, copiedMark, lastMark = markDone, markDone, markDone
		last = string(groupProgressDone)
	}
	if row.failed {
		switch {
		case lastMark == markCurrent || lastMark == markDone:
			lastMark = markFailed
			last = string(groupProgressFailed)
		case copyMark == markCurrent:
			copyMark = markFailed
		case copiedMark == markCurrent:
			copiedMark = markFailed
		default:
			lastMark = markFailed
			last = string(groupProgressFailed)
		}
	}
	arrow := theme.Text(terminaltheme.Muted, progressArrow)
	return progressToken(theme, string(groupProgressStaging), copyMark) + arrow +
		progressToken(theme, string(groupProgressCopied), copiedMark) + arrow +
		progressToken(theme, padLastProgress(last), lastMark)
}

func padLastProgress(text string) string {
	width := utf8.RuneCountInString(text)
	if width >= lastProgressWidth {
		return text
	}
	return text + strings.Repeat(" ", lastProgressWidth-width)
}

func progressToken(theme terminaltheme.Theme, text string, mark progressMark) string {
	role := terminaltheme.Muted
	switch mark {
	case markCurrent:
		role = terminaltheme.Label
	case markDone:
		role = terminaltheme.Success
	case markFailed:
		role = terminaltheme.Error
	}
	return theme.Text(role, text)
}

func progressRowMatches(row, event domain.ClientID) bool {
	if row == event {
		return true
	}
	return (row == domain.ClientCopilot || row == domain.ClientVSCode) &&
		(event == domain.ClientCopilot || event == domain.ClientVSCode)
}
