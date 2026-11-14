package main

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl"
	"github.com/spf13/cobra"
)

func runIntegrationsAdd(cmd *cobra.Command, args []string) error {
	return executeIntegrationsAdd(cmd, integrationctl.AddParams{
		Source:          args[0],
		Targets:         integrationctl.NormalizeTargets(integrationTargets),
		Scope:           strings.TrimSpace(integrationScope),
		AutoUpdate:      boolPtr(integrationAutoUpdate),
		AdoptNewTargets: strings.TrimSpace(integrationAdoptNewTargets),
		AllowPrerelease: boolPtr(integrationAllowPre),
		DryRun:          integrationDryRun,
	})
}

func newIntegrationsAddCommand(use, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           use,
		Short:         short,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runIntegrationsAdd,
	}
	cmd.Flags().StringSliceVar(&integrationTargets, "target", nil, "limit planning to one or more targets")
	cmd.Flags().StringVar(&integrationScope, "scope", "user", "scope intent for the planned installation")
	cmd.Flags().BoolVar(&integrationAutoUpdate, "auto-update", true, "desired auto-update policy")
	cmd.Flags().StringVar(&integrationAdoptNewTargets, "adopt-new-targets", "manual", "policy for newly supported targets: manual or auto")
	cmd.Flags().BoolVar(&integrationAllowPre, "pre", false, "allow prerelease updates")
	cmd.Flags().BoolVar(&integrationDryRun, "dry-run", true, "plan only without mutating native targets")
	return cmd
}

var integrationsAddCmd = newIntegrationsAddCommand(
	"add <source>",
	"Plan installation of an integration across supported agent targets",
)

func validateIntegrationsUpdateArgs(cmd *cobra.Command, args []string) error {
	return validateUpdateArgs(integrationUpdateAll, args)
}

func runIntegrationsUpdate(cmd *cobra.Command, args []string) error {
	name := ""
	if len(args) == 1 {
		name = args[0]
	}
	return executeIntegrationsUpdate(cmd, integrationctl.UpdateParams{
		Name:   name,
		All:    integrationUpdateAll,
		DryRun: integrationDryRun,
	})
}

func newIntegrationsUpdateCommand(use, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           use,
		Short:         short,
		Args:          validateIntegrationsUpdateArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runIntegrationsUpdate,
	}
	cmd.Flags().BoolVar(&integrationDryRun, "dry-run", true, "plan only without mutating native targets")
	cmd.Flags().BoolVar(&integrationUpdateAll, "all", false, "update all managed integrations")
	return cmd
}

var integrationsUpdateCmd = newIntegrationsUpdateCommand(
	"update [name]",
	"Plan or apply an update for a managed integration",
)

func runIntegrationsRemove(cmd *cobra.Command, args []string) error {
	return executeIntegrationsRemove(cmd, integrationctl.RemoveParams{
		Name:   args[0],
		DryRun: integrationDryRun,
	})
}

func newIntegrationsRemoveCommand(use, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           use,
		Short:         short,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runIntegrationsRemove,
	}
	cmd.Flags().BoolVar(&integrationDryRun, "dry-run", true, "plan only without mutating native targets")
	return cmd
}

var integrationsRemoveCmd = newIntegrationsRemoveCommand(
	"remove <name>",
	"Plan or remove managed integration targets",
)

func runIntegrationsRepair(cmd *cobra.Command, args []string) error {
	return executeIntegrationsRepair(cmd, integrationctl.RepairParams{
		Name:   args[0],
		Target: firstNormalizedTarget(integrationTargets),
		DryRun: integrationDryRun,
	})
}

func newIntegrationsRepairCommand(use, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           use,
		Short:         short,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runIntegrationsRepair,
	}
	cmd.Flags().BoolVar(&integrationDryRun, "dry-run", true, "plan only without mutating native targets")
	cmd.Flags().StringSliceVar(&integrationTargets, "target", nil, "limit repair to one target")
	return cmd
}

var integrationsRepairCmd = newIntegrationsRepairCommand(
	"repair <name>",
	"Plan or repair managed integration drift",
)
