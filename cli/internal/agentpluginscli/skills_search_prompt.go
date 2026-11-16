package agentpluginscli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
)

func promptSkillsCatalog(cmd *cobra.Command, s *skillsCatalogSession, catalog skillspicker.Catalog, opts *options) ([]skillsource.Candidate, error) {
	available := []skillsource.Candidate{}
	for _, candidate := range catalog.Candidates {
		if candidate.Status == skillspicker.Available {
			available = append(available, s.candidates[candidate.Id])
		}
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("no available Skills in this catalog")
	}
	if !opts.plain {
		app, err := withPrompter(cmd, s.app, opts)
		if err != nil && !errors.Is(err, prompt.ErrPromptUnavailable) {
			return nil, err
		}
		if selector, ok := app.Prompter.(prompt.SkillsSelector); err == nil && ok {
			request := skillsCatalogRequest(available)
			result, err := selector.SelectSkills(cmd.Context(), request)
			if err == nil {
				return validateCatalogSelection(available, result.IDs)
			}
			if !errors.Is(err, prompt.ErrPromptUnavailable) {
				return nil, err
			}
		}
	}
	return promptFilteredSkills(cmd, available)
}

func skillsCatalogRequest(available []skillsource.Candidate) prompt.SkillsSelectionRequest {
	request := prompt.SkillsSelectionRequest{}
	for _, candidate := range available {
		label := prompt.SafeText(candidate.Name) + " · " + prompt.SafeText(candidate.Path)
		request.Choices = append(request.Choices, prompt.SkillChoice{ID: candidate.ID, Label: label})
	}
	return request
}

func validateCatalogSelection(available []skillsource.Candidate, ids []string) ([]skillsource.Candidate, error) {
	allowed := make(map[string]bool, len(available))
	for _, candidate := range available {
		allowed[candidate.ID] = true
	}
	selected := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !allowed[id] || selected[id] {
			return nil, errors.New("invalid or duplicate Skill selection")
		}
		selected[id] = true
	}
	if len(selected) == 0 {
		return nil, errors.New("select at least one Skill")
	}
	return selectedTerminalSkills(available, selected), nil
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
	writer := cmd.OutOrStdout()
	theme := terminaltheme.For(writer)
	plain, _ := cmd.Flags().GetBool("plain")
	if plain {
		theme = terminaltheme.Theme{}
	}
	counts := fmt.Sprintf("(%d of %d matches; %s)", len(shown), total, theme.Text(terminaltheme.Success, fmt.Sprintf("%d selected", len(selected))))
	if _, err := fmt.Fprintf(writer, "%s %s — filter: %s\n", theme.Text(terminaltheme.Label, "Available Skills"), counts, prompt.SafeText(query)); err != nil {
		return err
	}
	for i, candidate := range shown {
		mark := theme.Text(terminaltheme.Muted, "[ ]")
		name := prompt.SafeText(candidate.Name)
		if selected[candidate.ID] {
			mark = theme.Text(terminaltheme.Success, "[x]")
			name = theme.Text(terminaltheme.Label, name)
		}
		path := theme.Text(terminaltheme.Muted, "("+prompt.SafeText(candidate.Path)+")")
		if _, err := fmt.Fprintf(writer, "  %d. %s %s %s\n", i+1, mark, name, path); err != nil {
			return err
		}
	}
	controls := "Toggle numbers separated by commas; /query searches name, path and description; / clears filter;\nclear removes selections; done reviews; q or Enter cancels"
	_, err := fmt.Fprint(writer, theme.Text(terminaltheme.Muted, controls)+"\n"+theme.Text(terminaltheme.Label, "Select: "))
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
