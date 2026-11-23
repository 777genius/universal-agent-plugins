package agentpluginscli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environmentapp"
)

func renderDependencyAppliedResults(cmd *cobra.Command, opts *options, actions []dependencyAction, applied environmentapp.Result, applyErr error) error {
	if err := renderDependencyResults(cmd, opts, actions, applied.Results(), applyErr); err != nil {
		return err
	}
	// Binding outcomes exist only after successful agreement publication. A
	// publication refusal alone must never be described as a saved installation.
	if applyErr != nil && len(applied.Outcomes) > 0 && opts.environmentOutput != nil && opts.format == "human" {
		scope := "Use --scope user for both commands."
		if opts.scope == "project" {
			scope = "Use --scope project and the same --project-root for both commands."
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "The dependency list records requested targets, not successful installations. Check the per-agent results and agentplugins deps status. Resolve failures, then retry agentplugins install --locked.", scope)
		return err
	}
	return nil
}
