package agentpluginscli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminalprompts"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// One renderer serves native plugin progress and multi-Skill binding progress.
// The latter retains binding identity so a first success cannot finish an agent.
type skillsProgressBoard struct {
	board      *groupProgressBoard
	items      map[string]SkillsBatchItem
	identities map[string][]string
}

func startSkillsProgressBoard(app App, opts *options, actions []SkillsAction) *skillsProgressBoard {
	if opts.format != "human" || !app.Terminal || len(actions) == 0 {
		return nil
	}
	var clients []domain.DetectedClient
	seen := map[string]bool{}
	progress := &skillsProgressBoard{items: map[string]SkillsBatchItem{}, identities: map[string][]string{}}
	for _, action := range actions {
		key := skillsBindingKey(action.Alias, action.ClientID, action.Action)
		progress.items[key] = SkillsBatchItem{Alias: action.Alias, ClientID: action.ClientID, Action: action.Action, Status: "not_attempted"}
		identity := skillsBindingKey(action.Alias, action.ClientID, "")
		progress.identities[identity] = append(progress.identities[identity], key)
		if !seen[action.ClientID] {
			seen[action.ClientID] = true
			clients = append(clients, domain.DetectedClient{ClientID: domain.ClientID(action.ClientID), DisplayName: reviewClientName(domain.ClientID(action.ClientID))})
		}
	}
	progress.board = configuredGroupProgressBoard(app.errorOutput(), clients, app.scope(), func(board *groupProgressBoard) {
		board.forcePlain = terminalprompts.DetectMode(app.input(), app.output(), app.errorOutput(), opts.plain) != terminalprompts.Rich
		for i := range board.rows {
			for _, item := range progress.items {
				if item.ClientID == string(board.rows[i].id) {
					board.rows[i].bindingsTotal++
				}
			}
			board.rows[i].bindingState = "queued"
		}
	})
	return progress
}

func (progress *skillsProgressBoard) observe(event SkillsProgress) {
	board := progress.board
	board.mu.Lock()
	defer board.mu.Unlock()
	key := skillsBindingKey(event.Alias, event.ClientID, event.Action)
	item, known := progress.items[key]
	if !known {
		return
	}
	for i := range board.rows {
		row := &board.rows[i]
		if string(row.id) != event.ClientID {
			continue
		}
		row.bindingAlias = ansi.Truncate(prompt.SafeText(event.Alias), 24, "…")
		if event.Phase == "finished" {
			item.Status, item.Client, item.ManualActions = string(event.Result.Outcome), event.Result.Client, event.Result.ManualActions
			progress.items[key] = item
			progress.reconcileRow(row, false)
		} else {
			row.bindingState = event.Phase
		}
	}
	board.redrawLocked()
}

func (progress *skillsProgressBoard) finish(result SkillsBatchResult) {
	if progress == nil {
		return
	}
	defer func() { _ = recover() }() // Terminal failure never masks the apply error.
	board := progress.board
	board.mu.Lock()
	defer board.mu.Unlock()
	// The transaction result, not the animation's last event, is authoritative.
	for key, item := range progress.items {
		item.Status, item.Client, item.ManualActions = "not_attempted", installer.ClientResult{}, nil
		progress.items[key] = item
	}
	for _, item := range result.Items {
		key := skillsBindingKey(item.Alias, item.ClientID, item.Action)
		if item.Action == "" {
			keys := progress.identities[key]
			if len(keys) != 1 {
				continue
			}
			key = keys[0]
		}
		if _, known := progress.items[key]; known {
			progress.items[key] = item
		}
	}
	for i := range board.rows {
		progress.reconcileRow(&board.rows[i], true)
		board.rows[i].bindingAlias = ""
	}
	board.finishLocked()
}

func (progress *skillsProgressBoard) reconcileRow(row *groupProgressRow, final bool) {
	row.bindingsCompleted = 0
	state, priority := "installed", 0
	allUnchanged, allRemoved := true, true
	for _, item := range progress.items {
		if item.ClientID != string(row.id) {
			continue
		}
		if item.Status == string(installer.OutcomeCompleted) || item.Status == string(installer.OutcomeUnchanged) {
			row.bindingsCompleted++
		}
		label := skillsItemPresentation(item)
		allUnchanged = allUnchanged && label == "unchanged"
		allRemoved = allRemoved && label == "removed"
		p := skillsProgressPriority(label)
		if p > priority {
			state, priority = label, p
		}
	}
	if allUnchanged {
		state = "unchanged"
	} else if allRemoved {
		state = "removed"
	}
	if !final && state == "not attempted" {
		state = "waiting"
	}
	if final && row.bindingsCompleted < row.bindingsTotal && priority < 6 && (row.bindingsCompleted > 0 || priority > 1) {
		state = "not completed"
	}
	row.bindingState = state
}

func skillsProgressPriority(label string) int {
	switch label {
	case "not attempted":
		return 1
	case "not completed":
		return 2
	case "setup required":
		return 3
	case "sign-in required":
		return 4
	case "user-attested":
		return 5
	case "rolled back":
		return 6
	case "canceled":
		return 7
	case "failed":
		return 8
	default:
		return 0
	}
}

func bindingProgressPipeline(theme terminaltheme.Theme, row groupProgressRow) string {
	role, mark := terminaltheme.Muted, "·"
	switch row.bindingState {
	case "preparing", "installing":
		role, mark = terminaltheme.Label, "●"
	case "installed", "removed":
		role, mark = terminaltheme.Success, "✓"
	case "failed":
		role, mark = terminaltheme.Error, "×"
	case "setup required", "sign-in required", "rolled back", "canceled", "not completed", "user-attested":
		role, mark = terminaltheme.Warning, "!"
	}
	state := fmt.Sprintf("%s %d/%d %s", mark, row.bindingsCompleted, row.bindingsTotal, row.bindingState)
	if row.bindingAlias != "" {
		state += " — " + row.bindingAlias
	}
	return theme.Text(role, state)
}

func skillsItemPresentation(item SkillsBatchItem) string {
	if item.Client.GroupPhase == string(usecase.GroupTargetManagedRolledBack) {
		return "rolled back"
	}
	if item.Client.Failure != nil {
		return "failed"
	}
	switch item.Status {
	case "not_attempted", "":
		return "not attempted"
	case string(installer.OutcomeUnchanged):
		return "unchanged"
	case string(installer.OutcomeCancelled):
		return "canceled"
	case string(installer.OutcomeCompleted):
		if item.Action == "remove" {
			return "removed"
		}
	default:
		return "failed"
	}
	client := item.Client
	return strings.ToLower(string(classifyBatchPresentation(addTargetResult{Output: addResultData{Result: usecase.AddResult{
		GroupPhase: usecase.GroupTargetPhase(client.GroupPhase),
		Activation: domain.ActivationOutcome{Activation: domain.ActivationState(client.Activation), Authentication: domain.AuthenticationState(client.Authentication), Verification: domain.VerificationState(client.Verification)},
	}}})))
}
