package main

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var version = ""

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print plugin-kit-ai CLI module version (from build info)",
	Run: func(cmd *cobra.Command, args []string) {
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			fmt.Fprintln(cmd.OutOrStdout(), "plugin-kit-ai: build info unavailable")
			return
		}
		printVersion := strings.TrimSpace(version)
		if printVersion == "" {
			printVersion = bi.Main.Version
		}
		fmt.Fprintf(cmd.OutOrStdout(), "module: %s\n", bi.Main.Path)
		fmt.Fprintf(cmd.OutOrStdout(), "version: %s\n", printVersion)
		fmt.Fprintf(cmd.OutOrStdout(), "go: %s\n", bi.GoVersion)
	},
}
