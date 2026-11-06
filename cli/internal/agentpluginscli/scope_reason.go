package agentpluginscli

import "github.com/spf13/cobra"

// Scope is never guessed from a parent directory. Only argumentless install
// may select the exact working directory's regular uap.toml automatically.
func scopeSelectionReason(cmd *cobra.Command, opts *options) string {
	if opts.scopeInferred {
		return "argumentless_project_manifest"
	}
	if cmd.Root().PersistentFlags().Changed("scope") || opts.scope == "project" || opts.projectRoot != "" {
		return "explicit"
	}
	return "default_user"
}
