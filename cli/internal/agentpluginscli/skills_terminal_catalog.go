package agentpluginscli

import (
	"fmt"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
)

const terminalSkillsPageSize = 50

type terminalSkillsView struct {
	query        string
	page         int
	selectedOnly bool
}

func (view terminalSkillsView) matches(available []skillsource.Candidate, selected map[string]bool) []skillsource.Candidate {
	if view.selectedOnly {
		available = selectedTerminalSkills(available, selected)
	}
	return filterTerminalSkills(available, view.query)
}
func (view *terminalSkillsView) pageMatches(matches []skillsource.Candidate) []skillsource.Candidate {
	view.page = min(view.page, max(0, (len(matches)-1)/terminalSkillsPageSize))
	start := view.page * terminalSkillsPageSize
	return matches[start:min(start+terminalSkillsPageSize, len(matches))]
}
func (view *terminalSkillsView) navigate(line string, total int) bool {
	if strings.HasPrefix(line, "/") {
		view.query, view.page = strings.TrimSpace(line[1:]), 0
		return true
	}
	switch line {
	case "next", "n":
		if (view.page+1)*terminalSkillsPageSize < total {
			view.page++
		}
	case "prev", "previous", "p":
		view.page = max(0, view.page-1)
	case "selected":
		view.selectedOnly, view.page, view.query = true, 0, ""
	case "all":
		view.selectedOnly, view.page = false, 0
	default:
		return false
	}
	return true
}
func (view terminalSkillsView) renderNavigation(writer io.Writer, total int) error {
	mode := "catalog"
	if view.selectedOnly {
		mode = "selected Skills"
	}
	_, err := fmt.Fprintf(writer, "Page %d of %d · %s; next/prev browse; selected shows basket; all returns to catalog.\nSelect: ", view.page+1, max(1, (total+terminalSkillsPageSize-1)/terminalSkillsPageSize), mode)
	return err
}

// Discovery omissions are guidance rather than selectable items. Use only
// bounded metadata and the shared diagnostic vocabulary, never provider errors.
func renderSkillsCatalogOmissions(output io.Writer, catalog skillspicker.Catalog) error {
	writer := &planWriter{writer: output}
	omitted, shown := 0, 0
	for _, candidate := range catalog.Candidates {
		if candidate.Status == skillspicker.Available {
			continue
		}
		omitted++
		if shown == 10 {
			continue
		}
		shown++
		path := candidate.Path
		if path == "" {
			path = "."
		}
		guidance := "this Skill is not available for installation"
		if len(candidate.DiagnosticCodes) > 0 {
			guidance = skillDiagnosticGuidance(candidate.DiagnosticCodes)
		}
		if _, err := fmt.Fprintf(writer, "Unavailable Skill %s (%s): %s\n", prompt.SafeText(path), prompt.SafeText(string(candidate.Status)), prompt.SafeText(guidance)); err != nil {
			return err
		}
	}
	if omitted > shown {
		if _, err := fmt.Fprintf(writer, "Showing reasons for %d of %d unavailable Skills; narrow the source with --path <folder>.\n", shown, omitted); err != nil {
			return err
		}
	}
	if !catalog.Complete {
		if _, err := fmt.Fprintln(writer, "Catalog scan is incomplete; select from validated discoveries or narrow the source with --path <folder> to discover nested Skills."); err != nil {
			return err
		}
	}
	return nil
}
