package agentpluginscli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environmentapp"
)

func renderDependencyAppliedResults(cmd *cobra.Command, app App, opts *options, request dependencySyncRequest, applied environmentapp.Result, applyErr error) error {
	if err := renderDependencyResults(cmd, opts, request.prepared, applied.Results(), applyErr); err != nil {
		return err
	}
	if applyErr == nil || opts.format != "human" {
		return nil
	}
	message := ""
	switch {
	case len(applied.Outcomes) > 0 && opts.environmentOutput != nil:
		message = "The dependency list records requested targets, not successful installations. Check the per-agent results and resolve their next actions before further changes."
	case applied.Published && len(applied.Outcomes) == 0:
		message = "Environment intent may have changed; publication could not be confirmed. Inspect the manifest and lock below before retrying. If they disagree, deps lock reconciles the lock. A deps sync checks the complete saved environment; review its dry-run before applying. Use the scope below and, for project scope, the exact project root shown below."
	}
	if message == "" {
		return nil
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\nScope: %s\nProject root: %s\nManifest: %s\nLock: %s\n", message, app.scope(), app.ProjectRoot, request.intent.ManifestPath, request.intent.LockPath)
	return err
}
