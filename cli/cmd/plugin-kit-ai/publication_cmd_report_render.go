package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/spf13/cobra"
)

type publicationInspection = pluginmanifest.Inspection
type publicationWarning = pluginmanifest.Warning

func renderPublicationReport(cmd *cobra.Command, report publicationInspection, warnings []publicationWarning, target, format string) error {
	switch normalizedPublicationReportFormat(format) {
	case "text":
		return writePublicationTextReport(cmd, report, warnings)
	case "json":
		return writePublicationJSONReport(cmd, buildPublicationJSONReport(report, warnings, target))
	default:
		return fmt.Errorf("unsupported format %q (use text or json)", format)
	}
}

func normalizedPublicationReportFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return "text"
	case "json":
		return "json"
	default:
		return "invalid"
	}
}

func writePublicationJSONReport(cmd *cobra.Command, report publicationJSONReport) error {
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}
