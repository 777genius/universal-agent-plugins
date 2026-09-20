package platformexec

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func appendImportedClaudeManifest(seed ImportSeed, pluginManifest importedClaudePluginManifest, manifestPresent bool, result *ImportResult) {
	manifestPath := filepath.ToSlash(filepath.Join(".claude-plugin", "plugin.json"))
	if manifestPresent {
		if strings.TrimSpace(pluginManifest.Name) != "" {
			result.Manifest.Name = pluginManifest.Name
		}
		if strings.TrimSpace(pluginManifest.Version) != "" {
			result.Manifest.Version = pluginManifest.Version
		}
		if strings.TrimSpace(pluginManifest.Description) != "" {
			result.Manifest.Description = pluginManifest.Description
		}
	} else {
		result.Warnings = append(result.Warnings, pluginmodel.Warning{
			Kind:    pluginmodel.WarningFidelity,
			Path:    ".claude-plugin/plugin.json",
			Message: "native Claude plugin imported without manifest; package-standard defaults were derived from the directory name",
		})
	}
	for _, warning := range pluginManifest.Warnings {
		result.Warnings = append(result.Warnings, pluginmodel.Warning{
			Kind:    pluginmodel.WarningFidelity,
			Path:    manifestPath,
			Message: warning,
		})
	}
	if strings.TrimSpace(pluginManifest.Name) != "" && pluginManifest.Name != seed.Manifest.Name {
		result.Warnings = append(result.Warnings, pluginmodel.Warning{
			Kind:    pluginmodel.WarningFidelity,
			Path:    manifestPath,
			Message: "normalized Claude plugin identity into canonical package-standard plugin.yaml",
		})
	}
}
