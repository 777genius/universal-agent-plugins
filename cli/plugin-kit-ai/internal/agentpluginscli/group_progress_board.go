package agentpluginscli

import (
	"context"
	"fmt"
	"io"
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
	groupProgressStaging    groupProgressStep = "staging"
	groupProgressPrepared   groupProgressStep = "prepared"
	groupProgressActivating groupProgressStep = "activating"
	groupProgressDone       groupProgressStep = "done"
	groupProgressFailed     groupProgressStep = "failed"
)

type groupProgressRow struct {
	id    domain.ClientID
	label string
	step  groupProgressStep
}

type groupProgressBoard struct {
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
	if board == nil || !board.live || board.painted == 0 {
		return
	}
	_, _ = fmt.Fprintf(board.writer, "\033[%dA", board.painted)
	for range board.rows {
		_, _ = fmt.Fprint(board.writer, "\033[2K\n")
	}
	_, _ = fmt.Fprintf(board.writer, "\033[%dA", board.painted)
	board.painted = 0
}

func (board *groupProgressBoard) set(id domain.ClientID, step groupProgressStep) {
	if board == nil {
		return
	}
	changed := false
	for index := range board.rows {
		if !progressRowMatches(board.rows[index].id, id) {
			continue
		}
		if board.rows[index].step == step {
			continue
		}
		board.rows[index].step = step
		changed = true
	}
	if changed {
		board.redraw()
	}
}

func (board *groupProgressBoard) redraw() {
	if board == nil || !board.live {
		return
	}
	if board.painted > 0 {
		_, _ = fmt.Fprintf(board.writer, "\033[%dA", board.painted)
	}
	for _, row := range board.rows {
		_, _ = fmt.Fprintf(board.writer, "\033[2K  %-*s  %s\n", board.width, prompt.SafeText(row.label), board.theme.Text(progressTone(row.step), string(row.step)))
	}
	board.painted = len(board.rows)
}

func (stager progressStager) Stage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints) (domain.StagedDelivery, error) {
	stager.board.set(plan.ClientID, groupProgressStaging)
	delivery, err := stager.PackageStager.Stage(ctx, envelope, plan, operationID, hints)
	if err != nil {
		stager.board.set(plan.ClientID, groupProgressFailed)
		return delivery, err
	}
	stager.board.set(plan.ClientID, groupProgressPrepared)
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

func progressRowMatches(row, event domain.ClientID) bool {
	if row == event {
		return true
	}
	return (row == domain.ClientCopilot || row == domain.ClientVSCode) &&
		(event == domain.ClientCopilot || event == domain.ClientVSCode)
}

func progressTone(step groupProgressStep) terminaltheme.Role {
	switch step {
	case groupProgressDone:
		return terminaltheme.Success
	case groupProgressFailed:
		return terminaltheme.Error
	case groupProgressStaging, groupProgressActivating:
		return terminaltheme.Label
	default:
		return terminaltheme.Muted
	}
}
