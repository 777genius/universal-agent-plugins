package main

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl"
	"github.com/spf13/cobra"
)

func newRootAddCommand() *cobra.Command {
	var (
		targets         []string
		scope           string
		autoUpdate      bool
		adoptNewTargets string
		allowPre        bool
		dryRun          bool
	)

	cmd := &cobra.Command{
		Use:           "add <source>",
		Short:         "Install an integration across supported agent targets",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeIntegrationsAdd(cmd, integrationctl.AddParams{
				Source:          args[0],
				Targets:         integrationctl.NormalizeTargets(targets),
				Scope:           strings.TrimSpace(scope),
				AutoUpdate:      boolPtr(autoUpdate),
				AdoptNewTargets: strings.TrimSpace(adoptNewTargets),
				AllowPrerelease: boolPtr(allowPre),
				DryRun:          dryRun,
			})
		},
	}
	cmd.Flags().StringSliceVar(&targets, "target", nil, "limit installation to one or more targets")
	cmd.Flags().StringVar(&scope, "scope", "user", "scope intent for the installation")
	cmd.Flags().BoolVar(&autoUpdate, "auto-update", true, "desired auto-update policy")
	cmd.Flags().StringVar(&adoptNewTargets, "adopt-new-targets", "manual", "policy for newly supported targets: manual or auto")
	cmd.Flags().BoolVar(&allowPre, "pre", false, "allow prerelease updates")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "plan only without mutating native targets")
	return cmd
}

func newRootUpdateCommand() *cobra.Command {
	var (
		dryRun    bool
		updateAll bool
	)

	cmd := &cobra.Command{
		Use:           "update [name]",
		Short:         "Update a managed integration",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args: func(cmd *cobra.Command, args []string) error {
			return validateUpdateArgs(updateAll, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return executeIntegrationsUpdate(cmd, integrationctl.UpdateParams{
				Name:   name,
				All:    updateAll,
				DryRun: dryRun,
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "plan only without mutating native targets")
	cmd.Flags().BoolVar(&updateAll, "all", false, "update all managed integrations")
	return cmd
}

func newRootRemoveCommand() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:           "remove <name>",
		Short:         "Remove a managed integration",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeIntegrationsRemove(cmd, integrationctl.RemoveParams{
				Name:   args[0],
				DryRun: dryRun,
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "plan only without mutating native targets")
	return cmd
}

func newRootRepairCommand() *cobra.Command {
	var (
		targets []string
		dryRun  bool
	)

	cmd := &cobra.Command{
		Use:           "repair <name>",
		Short:         "Repair managed integration drift",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeIntegrationsRepair(cmd, integrationctl.RepairParams{
				Name:   args[0],
				Target: firstNormalizedTarget(targets),
				DryRun: dryRun,
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "plan only without mutating native targets")
	cmd.Flags().StringSliceVar(&targets, "target", nil, "limit repair to one target")
	return cmd
}
