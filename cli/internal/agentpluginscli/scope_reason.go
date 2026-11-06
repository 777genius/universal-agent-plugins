package agentpluginscli

import "github.com/spf13/cobra"

// Scope is never guessed from a parent directory. Only argumentless install
// may select the exact working directory's regular uap.toml automatically.
func scopeSelectionReason(cmd *cobra.Command, opts *options) string {
	if cmd.Root().PersistentFlags().Changed("scope") || opts.projectRoot != "" {
		return "explicit"
	}
	if opts.scope == "project" {
		return "argumentless_project_manifest"
	}
	return "default_user"
}
