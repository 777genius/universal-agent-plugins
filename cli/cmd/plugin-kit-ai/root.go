package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "plugin-kit-ai",
	Short: "plugin-kit-ai CLI - scaffold and tooling for AI plugins",
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(bundleCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(compatCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(publicationCmd)
	rootCmd.AddCommand(normalizeCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(capabilitiesCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(integrationsCmd)
	rootCmd.AddCommand(versionCmd)
}
