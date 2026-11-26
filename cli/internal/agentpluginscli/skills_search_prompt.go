package agentpluginscli

import (
	"errors"
	"fmt"
	"io"
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
	if err := renderSkillsCatalogOmissions(reviewWriter(cmd, s.app), catalog); err != nil {
		return nil, err
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("no available Skills in this catalog; inspect the candidate reasons or narrow the source with --path <folder>")
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
	return promptFilteredSkillsTo(cmd, available, reviewWriter(cmd, s.app))
}

func skillsCatalogRequest(available []skillsource.Candidate) prompt.SkillsSelectionRequest {
	request := prompt.SkillsSelectionRequest{}
	for _, candidate := range available {
		label := prompt.SkillNavigationLabel(candidate.Name + " · " + candidate.Path)
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
	return promptFilteredSkillsTo(cmd, available, cmd.OutOrStdout())
}
func promptFilteredSkillsTo(cmd *cobra.Command, available []skillsource.Candidate, output io.Writer) ([]skillsource.Candidate, error) {
	writer := &planWriter{writer: output}
	view := terminalSkillsView{}
	selected := map[string]bool{}
	for {
		matches := view.matches(available, selected)
		shown := view.pageMatches(matches)
		plain, _ := cmd.Flags().GetBool("plain")
		if err := renderTerminalSkillMatchesTo(writer, shown, len(matches), view.query, selected, plain); err != nil {
			return nil, err
		}
		if err := view.renderNavigation(writer, len(matches)); err != nil {
			return nil, err
		}
		line, err := readInputLine(cmd.Context(), cmd.InOrStdin())
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" || line == "q" || line == "quit" || strings.ContainsAny(line, "\x1b\x03\x04") {
			return nil, prompt.ErrPromptCanceled
		}
		if view.navigate(line, len(matches)) {
			continue
		}
		done, message := updateTerminalSkillsSelection(line, shown, selected)
		if done {
			return selectedTerminalSkills(available, selected), nil
		}
		if message == "" {
			continue
		}
		if _, err = fmt.Fprintln(writer, message); err != nil {
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
	plain, _ := cmd.Flags().GetBool("plain")
	return renderTerminalSkillMatchesTo(cmd.OutOrStdout(), shown, total, query, selected, plain)
}
func renderTerminalSkillMatchesTo(writer io.Writer, shown []skillsource.Candidate, total int, query string, selected map[string]bool, plain bool) error {
	theme := terminaltheme.For(writer)
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
	_, err := fmt.Fprintln(writer, theme.Text(terminaltheme.Muted, controls))
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
		return false, "Choose unique numbers from the displayed matches, next/prev to browse, /query to search, or done to review your selections."
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
