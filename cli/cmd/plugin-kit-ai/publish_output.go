package main

import (
	"encoding/json"
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

func renderPublishResult(cmd *cobra.Command, result app.PluginPublishResult, format string) error {
	switch format {
	case "json":
		return writePublishJSON(cmd, result)
	case "text":
		return writePublishText(cmd, result)
	default:
		return fmt.Errorf("unsupported publish output format %q", format)
	}
}

func writePublishJSON(cmd *cobra.Command, result app.PluginPublishResult) error {
	body, err := json.MarshalIndent(buildPublishJSONPayload(result), "", "  ")
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", body)
	return nil
}

func writePublishText(cmd *cobra.Command, result app.PluginPublishResult) error {
	for _, line := range result.Lines {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
	}
	return nil
}

func buildPublishJSONPayloadCollection(results []app.PluginPublishResult) []map[string]any {
	if len(results) == 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(results))
	for _, result := range results {
		out = append(out, buildPublishJSONPayload(result))
	}
	return out
}
