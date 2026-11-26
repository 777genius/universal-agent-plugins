package agentpluginscli

import "github.com/spf13/cobra"

// Factory unavailability means there is no usable interaction host, unlike an
// optional rich selector declining its capability. Carry that refusal until
// source resolution establishes whether explicit flags avoid every prompt.
func resolveSkillsInteraction(cmd *cobra.Command, app App, opts *options, interactive bool) (App, error) {
	if !interactive || app.Prompter != nil || app.PromptFactory == nil {
		return app, nil
	}
	resolved, err := withPrompter(cmd, app, opts)
	if err != nil {
		return app, err
	}
	return resolved, nil
}

func (selection *skillsAddOptions) requiresInteraction(opts *options, explicitSelection bool) bool {
	return !explicitSelection || opts.target == "" || !selection.yes && !opts.dryRun
}
