package main

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/capabilities"
	"github.com/spf13/cobra"
)

var (
	capabilitiesPlatform string
	capabilitiesFormat   string
	capabilitiesMode     string
)

var capabilitiesCmd = &cobra.Command{
	Use:   "capabilities",
	Short: "Show generated target/package or runtime support metadata",
	Long: `Shows generated contract metadata.

Default mode is target/package-oriented because plugin authors usually need to understand target class,
production boundary, import/generate/validate support, and supported component kinds.

Use --mode runtime to inspect runtime-event support for Claude, Codex, and Gemini.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch strings.ToLower(strings.TrimSpace(capabilitiesMode)) {
		case "", "targets":
			entries := capabilities.TargetByPlatform(capabilitiesPlatform)
			switch strings.ToLower(strings.TrimSpace(capabilitiesFormat)) {
			case "", "table":
				_, _ = cmd.OutOrStdout().Write(capabilities.TargetTable(entries))
				return nil
			case "json":
				out, err := capabilities.TargetJSON(entries)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				return fmt.Errorf("unsupported format %q (use table or json)", capabilitiesFormat)
			}
		case "runtime":
			entries := capabilities.ByPlatform(capabilitiesPlatform)
			switch strings.ToLower(strings.TrimSpace(capabilitiesFormat)) {
			case "", "table":
				_, _ = cmd.OutOrStdout().Write(capabilities.Table(entries))
				for _, line := range runtimeCapabilitiesNotes(capabilitiesPlatform) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
				}
				return nil
			case "json":
				out, err := capabilities.JSON(entries)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				return fmt.Errorf("unsupported format %q (use table or json)", capabilitiesFormat)
			}
		default:
			return fmt.Errorf("unsupported mode %q (use targets or runtime)", capabilitiesMode)
		}
	},
}

func runtimeCapabilitiesNotes(platform string) []string {
	if !strings.EqualFold(strings.TrimSpace(platform), "gemini") {
		return nil
	}
	return []string{
		"",
		"Note: Gemini runtime entries show the promoted production-ready 9-hook Go runtime surface.",
		"Runtime gate: make test-gemini-runtime",
		"Live runtime gate: make test-gemini-runtime-live",
	}
}

func init() {
	capabilitiesCmd.Flags().StringVar(&capabilitiesPlatform, "platform", "", "limit output to a single platform")
	capabilitiesCmd.Flags().StringVar(&capabilitiesFormat, "format", "table", "output format: table or json")
	capabilitiesCmd.Flags().StringVar(&capabilitiesMode, "mode", "targets", "capability view: targets or runtime")
}
