package agentpluginscli

import (
	"errors"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
)

// Website discovery is explicit and deterministic: a publisher index and exact
// names, never HTML crawling or heuristics over arbitrary website content.
func (selection *skillsAddOptions) runWebsite(cmd *cobra.Command, app App, opts *options, endpoint string, interactive bool) error {
	if selection.ui || selection.ref != "" || cmd.Flags().Changed("path") {
		return errors.New("HTTPS Skills use --skill names; --ui, --ref and --path apply only to Git sources")
	}
	if len(selection.names) == 0 {
		return skillsMissingSelection(cmd, opts, "SELECTION_REQUIRED", "HTTPS Skill sources require an exact --skill name", []skillspicker.Candidate{})
	}
	targets, err := skillsTerminalTargets(cmd, app, opts, interactive)
	if err != nil {
		return err
	}
	if !interactive && !selection.yes && !opts.dryRun {
		return skillsMissingSelection(cmd, opts, "CONFIRMATION_REQUIRED", "Pass --yes to accept installation; --dry-run reviews without applying", []skillspicker.Candidate{})
	}
	request := SkillsBatchRequest{}
	for _, name := range selection.names {
		request.Selections = append(request.Selections, SkillsSelection{Website: endpoint, Name: name})
	}
	for _, target := range targets {
		request.TargetIDs = append(request.TargetIDs, string(target))
	}
	prepared, err := app.PrepareSkills(cmd.Context(), request)
	if err != nil {
		return err
	}
	defer prepared.Close()
	return runSkillsReviewedPlan(cmd, app, opts, prepared, interactive && !selection.yes)
}

func isWebsiteSkillInput(input string) bool {
	u, err := url.Parse(input)
	return err == nil && u.Scheme == "https" && !strings.EqualFold(u.Hostname(), "github.com") && !strings.EqualFold(u.Hostname(), "raw.githubusercontent.com")
}
