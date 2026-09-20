package main

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

var integrationsRunner = app.NewIntegrationsRunner(nil)

var (
	integrationTargets         []string
	integrationScope           string
	integrationAutoUpdate      bool
	integrationAdoptNewTargets string
	integrationAllowPre        bool
	integrationDryRun          bool
	integrationUpdateAll       bool
)

var integrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Foundation lifecycle commands for multi-agent integration management",
}

func init() {
	integrationsEnableCmd.Flags().BoolVar(&integrationDryRun, "dry-run", true, "plan only without mutating native targets")
	integrationsEnableCmd.Flags().StringSliceVar(&integrationTargets, "target", nil, "limit enable to one target")
	integrationsDisableCmd.Flags().BoolVar(&integrationDryRun, "dry-run", true, "plan only without mutating native targets")
	integrationsDisableCmd.Flags().StringSliceVar(&integrationTargets, "target", nil, "limit disable to one target")
	integrationsSyncCmd.Flags().BoolVar(&integrationDryRun, "dry-run", true, "plan only without mutating native targets")

	integrationsCmd.AddCommand(integrationsListCmd)
	integrationsCmd.AddCommand(integrationsDoctorCmd)
	integrationsCmd.AddCommand(integrationsAddCmd)
	integrationsCmd.AddCommand(integrationsUpdateCmd)
	integrationsCmd.AddCommand(integrationsRemoveCmd)
	integrationsCmd.AddCommand(integrationsRepairCmd)
	integrationsCmd.AddCommand(integrationsEnableCmd)
	integrationsCmd.AddCommand(integrationsDisableCmd)
	integrationsCmd.AddCommand(integrationsSyncCmd)

	rootCmd.AddCommand(newRootAddCommand())
	rootCmd.AddCommand(newRootUpdateCommand())
	rootCmd.AddCommand(newRootRemoveCommand())
	rootCmd.AddCommand(newRootRepairCommand())
}
