package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/spf13/cobra"
)

type compatRunner interface {
	Compat(app.PluginCompatOptions) (pluginmanifest.SourceInspection, []pluginmanifest.Warning, error)
}

var compatCmd = newCompatCmd(pluginService)

func newCompatCmd(runner compatRunner) *cobra.Command {
	compatTarget := "all"
	compatFormat := "text"
	compatFrom := ""
	compatIncludeUserScope := false
	cmd := &cobra.Command{
		Use:   "compat <source>",
		Short: "Inspect a native source and report target compatibility",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			report, warnings, err := runner.Compat(app.PluginCompatOptions{
				Source:           args[0],
				From:             compatFrom,
				Target:           compatTarget,
				IncludeUserScope: compatIncludeUserScope,
			})
			if err != nil {
				return err
			}
			for _, warning := range warnings {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning.Message)
			}
			switch strings.ToLower(strings.TrimSpace(compatFormat)) {
			case "", "text":
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "source: %s\n", report.RequestedSource)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "resolved: %s\n", report.ResolvedSource)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "kind: %s digest=%s\n", report.SourceKind, report.SourceDigest)
				if report.CanonicalPackage {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "mode: canonical-package targets=%s\n", strings.Join(report.OriginTargets, ", "))
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "mode: imported-native from=%s detected=%s\n", report.ImportSource, joinOrDash(report.DetectedImportKinds))
					if len(report.DroppedKinds) > 0 {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "normalized-dropped: %s\n", joinOrDash(report.DroppedKinds))
					}
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "package: %s %s\n", report.Inspection.Manifest.Name, report.Inspection.Manifest.Version)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "portable: skills=%d mcp=%t\n", len(report.Inspection.Portable.Paths("skills")), report.Inspection.Portable.MCP != nil)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "compatibility:")
				for _, item := range report.Compatibility {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "- %s: status=%s supported=%s unsupported=%s\n",
						item.Target,
						item.Status,
						joinOrDash(item.SupportedKinds),
						joinOrDash(item.UnsupportedKinds),
					)
					for _, note := range item.Notes {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  note=%s\n", note)
					}
				}
				return nil
			case "json":
				body, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(body))
				return nil
			default:
				return fmt.Errorf("unsupported format %q (use text or json)", compatFormat)
			}
		},
	}
	cmd.Flags().StringVar(&compatTarget, "target", "all", `compatibility target ("all", "claude", "codex-package", "codex-runtime", "gemini", "opencode", "cursor", or "cursor-workspace")`)
	cmd.Flags().StringVar(&compatFormat, "format", "text", "output format: text or json")
	cmd.Flags().StringVar(&compatFrom, "from", "", `source platform ("claude", "codex-package", "codex-runtime", "gemini", "opencode", "cursor", or "cursor-workspace"; omit to auto-detect current native layouts)`)
	cmd.Flags().BoolVar(&compatIncludeUserScope, "include-user-scope", false, "include explicit user-scope native sources when supported by the detected import target")
	return cmd
}

func joinOrDash(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ",")
}
