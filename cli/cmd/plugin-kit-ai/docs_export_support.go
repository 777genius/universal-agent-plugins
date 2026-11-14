package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/capabilities"
	"github.com/spf13/cobra"
)

var docsSupportExportCmd = &cobra.Command{
	Use:    "export-support",
	Short:  "internal support and capability export",
	Hidden: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		return exportSupportDocs()
	},
}

func exportSupportDocs() error {
	entries := capabilities.All()
	if err := writeJSON(docsEventsPath, entries); err != nil {
		return err
	}
	if err := writeJSON(docsTargetsPath, capabilities.TargetAll()); err != nil {
		return err
	}
	return writeJSON(docsCapabilitiesPath, uniqueCapabilities(entries))
}

func writeJSON(path string, value any) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("output path is required")
	}
	body, err := marshalJSON(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

func uniqueCapabilities(entries []capabilities.Entry) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, entry := range entries {
		for _, capability := range entry.Capabilities {
			if _, ok := seen[capability]; ok {
				continue
			}
			seen[capability] = struct{}{}
			out = append(out, capability)
		}
	}
	slices.Sort(out)
	return out
}
