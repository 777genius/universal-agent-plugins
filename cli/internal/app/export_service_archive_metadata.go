package app

import "fmt"

func buildExportMetadata(ctx exportServiceContext) exportMetadata {
	return exportMetadata{
		PluginName:         ctx.graph.Manifest.Name,
		Platform:           ctx.platform,
		Runtime:            ctx.graph.Launcher.Runtime,
		Manager:            exportManager(ctx.project),
		BootstrapModel:     exportBootstrapModel(ctx.project),
		RuntimeRequirement: exportRuntimeRequirement(ctx.project.Runtime),
		RuntimeInstallHint: exportRuntimeInstallHint(ctx.project.Runtime),
		Next:               buildExportMetadataNext(ctx.platform),
		BundleFormat:       "tar.gz",
		GeneratedBy:        "plugin-kit-ai export",
	}
}

func buildExportMetadataNext(platform string) []string {
	return []string{
		"plugin-kit-ai doctor .",
		"plugin-kit-ai bootstrap .",
		fmt.Sprintf("plugin-kit-ai validate . --platform %s --strict", platform),
	}
}
