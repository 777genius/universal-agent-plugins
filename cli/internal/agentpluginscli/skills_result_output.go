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
	if _, err := fmt.Fprintf(writer, "Skills installation %s\n", prompt.SafeText(result.Status)); err != nil {
		return err
	}
	if result.Status == "succeeded" {
		if _, err := fmt.Fprintln(writer, "Files: installed or already up to date."); err != nil {
			return err
		}
	}
	theme := terminaltheme.For(writer)
	clients := []string{}
	byClient := map[string][]SkillsBatchItem{}
	for _, item := range result.Items {
		if _, found := byClient[item.ClientID]; !found {
			clients = append(clients, item.ClientID)
		}
		byClient[item.ClientID] = append(byClient[item.ClientID], item)
	}
	manual, seen := []string{}, map[string]bool{}
	for _, client := range clients {
		if _, err := fmt.Fprintln(writer, theme.Text(terminaltheme.Label, prompt.SafeText(reviewClientName(domain.ClientID(client))))); err != nil {
			return err
		}
		for _, item := range byClient[client] {
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
				if _, err := fmt.Fprintln(writer, "    "+prompt.SafeText(item.Reason)); err != nil {
					return err
				}
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
	manual = append(manual, result.RepairInstructions...)
	// Keep the published Skills flow's evidence boundary: native/file readiness
	// is not an agent discovery check or a model-side Skill activation proof.
	if len(result.Items) > 0 {
		if _, err := fmt.Fprint(writer, "Agent discovery: not verified by this Skills flow.\nActivation: not verified.\n"); err != nil {
			return err
		}
	}
	if result.Status == "succeeded" && len(manual) == 0 {
		manual = append(manual, "Open each agent, complete any trust/sign-in steps, reload if prompted, and confirm the Skill is visible.")
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
