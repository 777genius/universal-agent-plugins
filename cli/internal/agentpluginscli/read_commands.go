package agentpluginscli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func newListCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tracked Agent Plugins installations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			state, err := app.StateStore.Load()
			if err != nil {
				return err
			}
			installations := publicInstallations(state, false)
			if opts.format == "json" {
				return writeJSONOutput(cmd.OutOrStdout(), "list", map[string]any{"installations": installations})
			}
			return renderInstallationList(cmd.OutOrStdout(), installations)
		},
	}
}

func newInfoCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "info <name-or-installation-id>",
		Short: "Show lifecycle state without exposing local paths",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			state, err := app.StateStore.Load()
			if err != nil {
				return err
			}
			installation, err := selectInstallation(state, args[0])
			if err != nil {
				if strings.Contains(err.Error(), "ambiguous") || !isDirectorySelector(strings.TrimSpace(args[0])) {
					return err
				}
				return renderDirectorySelectorInfo(cmd, app, opts, state, args[0])
			}
			public, err := inspectInstalledProduct(cmd.Context(), app, state, installation)
			if err != nil {
				return fmt.Errorf("inspect signed Directory: %w", err)
			}
			if strings.TrimSpace(opts.target) != "" {
				if err := reconcileInstalledInfo(cmd.Context(), app, installation, opts.target, &public); err != nil {
					return err
				}
			}
			if opts.format == "json" {
				return writeJSONOutput(cmd.OutOrStdout(), "info", public)
			}
			return renderInstallation(cmd.OutOrStdout(), public)
		},
	}
}

// renderDirectorySelectorInfo handles a signed Directory lookup when no
// installed package matches the supplied direct selector.
func renderDirectorySelectorInfo(cmd *cobra.Command, app App, opts *options, state domain.StateFileV2, selector string) error {
	product, err := inspectDirectoryProduct(cmd.Context(), app, state, selector, opts.target)
	if err != nil {
		return err
	}
	if opts.format == "json" {
		return writeJSONOutput(cmd.OutOrStdout(), "info", product)
	}
	return renderProductInspection(cmd.OutOrStdout(), product)
}

func newDoctorCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [name-or-installation-id]",
		Short: "Read-only health check for clients, state, and open operations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			return runDoctor(cmd.Context(), cmd, app, opts, args)
		},
	}
}

func newVersionCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the agentplugins binary version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			version := strings.TrimSpace(app.Version)
			if version == "" {
				version = "development"
			}
			if opts.format == "json" {
				return writeJSONOutput(cmd.OutOrStdout(), "version", map[string]string{"version": version})
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "agentplugins %s\n", version)
			return err
		},
	}
}
