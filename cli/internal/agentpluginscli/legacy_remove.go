package agentpluginscli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func runLegacyRemove(ctx context.Context, cmd *cobra.Command, app App, opts *options, installation domain.Installation) error {
	if app.LegacyLifecycle == nil {
		return fmt.Errorf("legacy lifecycle bridge is not configured")
	}
	if target := strings.TrimSpace(opts.target); target != "" && target != "legacy-all" {
		return fmt.Errorf("legacy removal is integration-wide; use --target legacy-all after reviewing every target")
	}
	if !opts.dryRun && automatedMutation(app, opts) && strings.TrimSpace(opts.target) != "legacy-all" {
		return fmt.Errorf("automated legacy removal requires --target legacy-all")
	}
	service := app.Lifecycle
	service.StateStore = app.StateStore
	service.Legacy = app.LegacyLifecycle
	service.LegacyLock = app.LegacyStateLock
	input := usecase.LegacyRemoveInput{Selector: installation.InstallationID, DryRun: opts.dryRun}
	planned, err := service.RemoveLegacy(ctx, input)
	if err != nil {
		return err
	}
	if opts.dryRun {
		return renderLegacyRemove(cmd.OutOrStdout(), opts.format, planned, true)
	}
	if opts.format == "human" {
		renderLegacyRemovePlan(cmd.OutOrStdout(), planned)
	}
	confirmed := mutationConfirmed(app, opts)
	if !confirmed && opts.format == "human" && app.Terminal {
		confirmed, err = promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Remove every legacy target listed above? [y/N]")
		if err != nil {
			return err
		}
	}
	if !confirmed {
		if opts.format == "json" {
			return renderLegacyRemove(cmd.OutOrStdout(), opts.format, planned, false)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return nil
	}
	input.Confirmed = true
	result, removeErr := service.RemoveLegacy(ctx, input)
	if renderErr := renderLegacyRemove(cmd.OutOrStdout(), opts.format, result, false); renderErr != nil && removeErr == nil {
		removeErr = renderErr
	}
	return removeErr
}
