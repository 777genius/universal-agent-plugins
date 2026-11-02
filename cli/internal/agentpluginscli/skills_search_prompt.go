package agentpluginscli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
)

func promptSkillsCatalog(cmd *cobra.Command, s *skillsCatalogSession, catalog skillspicker.Catalog) ([]skillsource.Candidate, error) {
	available := []skillsource.Candidate{}
	for _, candidate := range catalog.Candidates {
		if candidate.Status == skillspicker.Available {
			available = append(available, s.candidates[candidate.Id])
		}
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("no available Skills in this catalog")
	}
	return promptFilteredSkills(cmd, available)
}
func promptFilteredSkills(cmd *cobra.Command, available []skillsource.Candidate) ([]skillsource.Candidate, error) {
	query := ""
	selected := map[string]bool{}
	for {
		matches := filterTerminalSkills(available, query)
		shown := matches[:min(len(matches), 50)]
		if err := renderTerminalSkillMatches(cmd, shown, len(matches), query, selected); err != nil {
			return nil, err
		}
		line, err := readInputLine(cmd.Context(), cmd.InOrStdin())
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" || line == "q" || line == "quit" || line == "\x1b" {
			return nil, prompt.ErrPromptCanceled
		}
		if strings.HasPrefix(line, "/") {
			query = strings.TrimSpace(line[1:])
			continue
		}
		done, message := updateTerminalSkillsSelection(line, shown, selected)
		if done {
			return selectedTerminalSkills(available, selected), nil
		}
		if message == "" {
			continue
		}
		if _, err = fmt.Fprintln(cmd.OutOrStdout(), message); err != nil {
			return nil, err
		}
	}
}
func filterTerminalSkills(candidates []skillsource.Candidate, query string) []skillsource.Candidate {
	query = strings.ToLower(query)
	matches := []skillsource.Candidate{}
	for _, candidate := range candidates {
		haystack := strings.ToLower(candidate.Name + "\n" + candidate.Path + "\n" + candidate.Description)
		if strings.Contains(haystack, query) {
			matches = append(matches, candidate)
		}
	}
	return matches
}
func renderTerminalSkillMatches(cmd *cobra.Command, shown []skillsource.Candidate, total int, query string, selected map[string]bool) error {
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Available Skills (%d of %d matches; %d selected) — filter: %s\n", len(shown), total, len(selected), prompt.SafeText(query)); err != nil {
		return err
	}
	for i, candidate := range shown {
		mark := " "
		if selected[candidate.ID] {
			mark = "x"
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %d. [%s] %s (%s)\n", i+1, mark, prompt.SafeText(candidate.Name), prompt.SafeText(candidate.Path)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), "Toggle numbers separated by commas; /query searches name, path and description; / clears filter; clear removes selections; done reviews; q or Enter cancels: ")
	return err
}

func updateTerminalSkillsSelection(line string, shown []skillsource.Candidate, selected map[string]bool) (done bool, message string) {
	switch line {
	case "done", "d":
		if len(selected) == 0 {
			return false, "Select at least one Skill before typing done."
		}
		return true, ""
	case "clear":
		clear(selected)
		return false, ""
	}
	choices, err := numberedSkillSelection(line, shown)
	if err != nil {
		return false, "Choose unique numbers from the displayed matches, /query to search, or done to review your selections."
	}
	for _, candidate := range choices {
		if selected[candidate.ID] {
			delete(selected, candidate.ID)
		} else {
			selected[candidate.ID] = true
		}
	}
	return false, ""
}

func selectedTerminalSkills(available []skillsource.Candidate, selected map[string]bool) []skillsource.Candidate {
	result := []skillsource.Candidate{}
	for _, candidate := range available {
		if selected[candidate.ID] {
			result = append(result, candidate)
		}
	}
	return result
}
