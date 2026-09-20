package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/spf13/cobra"
)

type inspectRunner interface {
	Inspect(app.PluginInspectOptions) (pluginmanifest.Inspection, []pluginmanifest.Warning, error)
}

var inspectCmd = newInspectCmd(pluginService)

func newInspectCmd(runner inspectRunner) *cobra.Command {
	inspectTarget := "all"
	inspectFormat := "text"
	inspectAuthoring := false
	cmd := &cobra.Command{
		Use:   "inspect [path]",
		Short: "Inspect the discovered package graph and target coverage",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			report, warnings, err := runner.Inspect(app.PluginInspectOptions{
				Root:   root,
				Target: inspectTarget,
			})
			if err != nil {
				return err
			}
			for _, warning := range warnings {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning.Message)
			}
			if inspectAuthoring {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), renderInspectAuthoring(report))
				return nil
			}
			switch strings.ToLower(strings.TrimSpace(inspectFormat)) {
			case "", "text":
				_, _ = fmt.Fprint(cmd.OutOrStdout(), renderInspectText(report))
				return nil
			case "json":
				out, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				return fmt.Errorf("unsupported format %q (use text or json)", inspectFormat)
			}
		},
	}
	cmd.Flags().StringVar(&inspectTarget, "target", "all", `inspect target ("all", "claude", "codex-package", "codex-runtime", "gemini", "opencode", "cursor", or "cursor-workspace")`)
	cmd.Flags().StringVar(&inspectFormat, "format", "text", "output format: text or json")
	cmd.Flags().BoolVar(&inspectAuthoring, "authoring", false, "show a plain-language authoring view instead of the raw contract view")
	return cmd
}
