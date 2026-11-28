package agentpluginscli

import (
	"errors"

	"github.com/spf13/cobra"
)

func (selection *skillsAddOptions) runLocal(cmd *cobra.Command, app App, opts *options, input string, interactive bool, targets []string) error {
	if selection.ui || selection.ref != "" {
		return errors.New("local Skills use terminal selection; omit --ui and --ref")
	}
	if opts.noSave {
		return errors.New("local Skills use saved snapshots; omit --no-save")
	}
	selections, err := selection.localSelections(cmd, app, opts, input, interactive, targets)
	if err != nil {
		return err
	}
	targets, err := skillsTerminalTargets(cmd, app, opts, interactive)
	if err != nil {
		return err
	}
	if !interactive && !selection.yes && !opts.dryRun {
		return errors.New("pass --yes to accept local Skill installation; --dry-run reviews without applying")
	}
	request := SkillsBatchRequest{Selections: selections}
	for _, target := range targets {
		request.TargetIDs = append(request.TargetIDs, string(target))
	}
	stop := startCommandActivity(cmd, app, opts, "Snapshotting local Skill and preparing installation review")
	prepared, err := app.PrepareSkills(cmd.Context(), request)
	stop()
	if err != nil {
		return reportSkillsPreparationRefusal(cmd, opts, &skillsCatalogSession{}, err)
	}
	defer prepared.Close()
	return runSkillsReviewedPlan(cmd, app, opts, prepared, interactive && !selection.yes)
}

func validateLocalSkillSelection(selection SkillsSelection) error {
	if selection.LocalPath == "" {
		return nil
	}
	if selection.Repository != "" || selection.URL != "" || selection.Website != "" || selection.Path != "" || selection.Commit != "" || selection.Selector.Kind != "" || selection.ExpectedIndexSHA256 != "" || selection.ExpectedRawTreeDigest != "" {
		return errors.New("local Skill selection cannot contain remote source fields")
	}
	return nil
}
