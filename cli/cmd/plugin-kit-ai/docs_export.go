package main

import (
	"github.com/spf13/cobra"
)

type docsManifestEntry struct {
	CommandPath string   `json:"command_path"`
	Slug        string   `json:"slug"`
	FileName    string   `json:"file_name"`
	Use         string   `json:"use"`
	Short       string   `json:"short,omitempty"`
	Long        string   `json:"long,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Deprecated  bool     `json:"deprecated"`
	Hidden      bool     `json:"hidden"`
}

var docsCmd = &cobra.Command{
	Use:    "__docs",
	Short:  "internal docs export helpers",
	Hidden: true,
}

var (
	docsCLIExportDir     string
	docsCLIManifestPath  string
	docsEventsPath       string
	docsTargetsPath      string
	docsCapabilitiesPath string
)

func init() {
	docsCLIExportCmd.Flags().StringVar(&docsCLIExportDir, "out-dir", "", "directory for generated markdown output")
	docsCLIExportCmd.Flags().StringVar(&docsCLIManifestPath, "manifest-path", "", "path for the generated command manifest json")
	docsSupportExportCmd.Flags().StringVar(&docsEventsPath, "events-path", "", "path for support event json")
	docsSupportExportCmd.Flags().StringVar(&docsTargetsPath, "targets-path", "", "path for target support json")
	docsSupportExportCmd.Flags().StringVar(&docsCapabilitiesPath, "capabilities-path", "", "path for capability summary json")
	docsCmd.AddCommand(docsCLIExportCmd)
	docsCmd.AddCommand(docsSupportExportCmd)
	rootCmd.AddCommand(docsCmd)
}
