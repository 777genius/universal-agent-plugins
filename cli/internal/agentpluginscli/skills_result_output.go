package agentpluginscli

import (
	"fmt"
	"io"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The returned transaction result, not the live animation, owns final truth.
func renderHumanSkillsResult(writer io.Writer, result SkillsBatchResult) error {
	readiness, _ := deriveSkillsReadiness(result.Status, result.Items...)
	status := result.Status
	if status == "succeeded" && readiness == "needs_action" {
		status = "needs attention"
	}
	if _, err := fmt.Fprintf(writer, "Skills installation %s\n", prompt.SafeText(status)); err != nil {
		return err
	}
	if result.Status == "succeeded" {
		if _, err := fmt.Fprintln(writer, "Files: installed or already up to date."); err != nil {
			return err
		}
	}
	if err := renderPartialSkillsReadiness(writer, result); err != nil {
		return err
	}
	manual, err := renderHumanSkillsClientResults(writer, result.Items)
	if err != nil {
		return err
	}
	manual = append(manual, result.RepairInstructions...)
	if readiness == "ready" {
		if _, err := fmt.Fprintln(writer, skillsReadyMessage); err != nil {
			return err
		}
	}
	if readiness == "needs_action" && len(manual) == 0 {
		manual = append(manual, "Run agentplugins doctor to verify the installed Skills and see any remaining setup steps.")
	}
	if len(manual) > 0 {
		if _, err := fmt.Fprintln(writer, "Next steps:"); err != nil {
			return err
		}
		for _, action := range manual {
			if _, err := fmt.Fprintln(writer, "  • "+prompt.SafeText(action)); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderHumanSkillsClientResults(writer io.Writer, items []SkillsBatchItem) ([]string, error) {
	theme := terminaltheme.For(writer)
	clients := []string{}
	byClient := map[string][]SkillsBatchItem{}
	for _, item := range items {
		if _, found := byClient[item.ClientID]; !found {
			clients = append(clients, item.ClientID)
		}
		byClient[item.ClientID] = append(byClient[item.ClientID], item)
	}
	manual, seen := []string{}, map[string]bool{}
	for _, client := range clients {
		if _, err := fmt.Fprintln(writer, theme.Text(terminaltheme.Label, prompt.SafeText(reviewClientName(domain.ClientID(client))))); err != nil {
			return nil, err
		}
		for _, item := range byClient[client] {
			if err := renderHumanSkillsItem(writer, theme, item); err != nil {
				return nil, err
			}
			for _, action := range item.ManualActions {
				text := reviewClientName(domain.ClientID(client)) + ": " + prompt.SafeText(action)
				if !seen[text] {
					manual = append(manual, text)
					seen[text] = true
				}
			}
		}
	}
	return manual, nil
}

func renderHumanSkillsItem(writer io.Writer, theme terminaltheme.Theme, item SkillsBatchItem) error {
	label := skillsItemPresentation(item)
	role, mark := terminaltheme.Warning, "!"
	switch label {
	case "installed", "removed":
		role, mark = terminaltheme.Success, "✓"
	case "failed", "rolled back":
		role, mark = terminaltheme.Error, "×"
	case "not attempted", "unchanged", "canceled":
		role, mark = terminaltheme.Muted, "·"
	}
	if _, err := fmt.Fprintf(writer, "  %s %s — %s\n", theme.Text(role, mark), prompt.SafeText(item.Alias), theme.Text(role, label)); err != nil {
		return err
	}
	if item.Reason != "" {
		_, err := fmt.Fprintln(writer, "    "+prompt.SafeText(item.Reason))
		return err
	}
	return nil
}
