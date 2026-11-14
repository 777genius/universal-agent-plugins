package agentpluginscli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func validatePluginEnablementSelection(cmd *cobra.Command, app App, opts *options) error {
	if !opts.confirmPluginEnable {
		return nil
	}
	if !cmd.Root().PersistentFlags().Changed("scope") || !cmd.Root().PersistentFlags().Changed("target") || opts.scope != "project" || strings.Contains(opts.targetInput, ",") || strings.TrimSpace(opts.target) == "" {
		return fmt.Errorf("--confirm-plugin-enable requires one explicit --target and --scope project")
	}
	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return err
	}
	if len(targets) != 1 || !clients.SupportsPluginEnablement(app.ClientRegistry, targets[0], domain.ScopeProject) {
		return fmt.Errorf("selected target does not support explicit Project plugin enablement")
	}
	return nil
}
