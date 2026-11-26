package agentpluginscli

import (
	"errors"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func (selection *skillsAddOptions) runLocal(cmd *cobra.Command, app App, opts *options, input string, interactive bool) error {
	if selection.ui || selection.ref != "" || cmd.Flags().Changed("path") {
		return errors.New("local Skills select one directory; omit --ui, --ref and --path")
	}
	if opts.noSave {
		return errors.New("local Skills use saved snapshots; omit --no-save")
	}
	absolute, err := filepath.Abs(input)
	if err != nil {
		return err
	}
	absolute = filepath.ToSlash(absolute)
	if err := sourcespec.ValidateLocalSkillPath(absolute); err != nil {
		return err
	}
	name := path.Base(absolute)
	if len(selection.names) > 1 || len(selection.names) == 1 && selection.names[0] != name {
		return errors.New("--skill must match the selected local Skill directory name")
	}
	targets, err := skillsTerminalTargets(cmd, app, opts, interactive)
	if err != nil {
		return err
	}
	if !interactive && !selection.yes && !opts.dryRun {
		return errors.New("pass --yes to accept local Skill installation; --dry-run reviews without applying")
	}
	request := SkillsBatchRequest{Selections: []SkillsSelection{{LocalPath: absolute, Name: name, Alias: opts.alias}}}
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
