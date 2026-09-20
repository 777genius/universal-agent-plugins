package main

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

func runInit(cmd *cobra.Command, runner initCommandRunner, flags initFlagState, args []string) error {
	name := strings.TrimSpace(args[0])
	runtime := flags.runtime
	if (strings.EqualFold(strings.TrimSpace(flags.platform), "gemini") ||
		strings.EqualFold(strings.TrimSpace(flags.platform), "codex-package") ||
		strings.EqualFold(strings.TrimSpace(flags.platform), "opencode") ||
		strings.EqualFold(strings.TrimSpace(flags.platform), "cursor") ||
		strings.EqualFold(strings.TrimSpace(flags.platform), "cursor-workspace")) &&
		!cmd.Flags().Changed("runtime") {
		runtime = ""
	}
	opts := app.InitOptions{
		ProjectName:           name,
		Template:              flags.template,
		Platform:              flags.platform,
		PlatformExplicit:      cmd.Flags().Changed("platform"),
		Runtime:               runtime,
		RuntimeExplicit:       cmd.Flags().Changed("runtime"),
		TypeScript:            flags.typescript,
		RuntimePackage:        flags.runtimePackage,
		RuntimePackageVersion: resolveRuntimePackageVersion(flags.runtimePackage, flags.runtimePackageVersion),
		OutputDir:             flags.output,
		Force:                 flags.force,
		Extras:                flags.extras,
		ClaudeExtendedHooks:   flags.claudeExtendedHooks,
	}
	out, err := runner.Run(opts)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), formatInitSuccess(out, opts))
	return nil
}
