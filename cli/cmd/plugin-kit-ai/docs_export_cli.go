package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docsCLIExportCmd = &cobra.Command{
	Use:    "export-cli",
	Short:  "internal Cobra docs export",
	Hidden: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		return exportCLIDocs()
	},
}

func exportCLIDocs() error {
	if strings.TrimSpace(docsCLIExportDir) == "" {
		return fmt.Errorf("--out-dir is required")
	}
	if strings.TrimSpace(docsCLIManifestPath) == "" {
		return fmt.Errorf("--manifest-path is required")
	}
	root := docsRootForExport()
	disableAutoGenTag(root)
	if err := os.MkdirAll(docsCLIExportDir, 0o755); err != nil {
		return err
	}
	if err := doc.GenMarkdownTree(root, docsCLIExportDir); err != nil {
		return err
	}
	manifest := visibleCommandManifest(root)
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(docsCLIManifestPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(docsCLIManifestPath, append(body, '\n'), 0o644)
}

func docsRootForExport() *cobra.Command {
	return rootCmd
}

func disableAutoGenTag(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	for _, child := range cmd.Commands() {
		disableAutoGenTag(child)
	}
}

func visibleCommandManifest(root *cobra.Command) []docsManifestEntry {
	out := make([]docsManifestEntry, 0)
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if !cmd.Hidden {
			out = append(out, docsManifestEntry{
				CommandPath: cmd.CommandPath(),
				Slug:        commandSlug(cmd),
				FileName:    commandMarkdownFile(cmd),
				Use:         cmd.Use,
				Short:       cmd.Short,
				Long:        strings.TrimSpace(cmd.Long),
				Aliases:     append([]string(nil), cmd.Aliases...),
				Deprecated:  strings.TrimSpace(cmd.Deprecated) != "",
				Hidden:      cmd.Hidden,
			})
		}
		for _, child := range cmd.Commands() {
			if child.Hidden {
				continue
			}
			walk(child)
		}
	}
	walk(root)
	return out
}

func commandMarkdownFile(cmd *cobra.Command) string {
	return strings.ReplaceAll(cmd.CommandPath(), " ", "_") + ".md"
}

func commandSlug(cmd *cobra.Command) string {
	return strings.ReplaceAll(strings.ToLower(cmd.CommandPath()), " ", "-")
}
