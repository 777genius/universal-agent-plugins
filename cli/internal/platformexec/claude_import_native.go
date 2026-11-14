package platformexec

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func appendImportedClaudeNativeDocs(root string, pluginManifest importedClaudePluginManifest, manifestPresent bool, result *ImportResult) error {
	if copied, warning, err := importClaudeStructuredDoc(root, "settings.json", filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "settings.json"), manifestPresent && pluginManifest.SettingsProvided, pluginManifest.Settings, "Claude manifest settings"); err != nil {
		return err
	} else {
		result.Artifacts = append(result.Artifacts, copied...)
		if strings.TrimSpace(warning) != "" {
			result.Warnings = append(result.Warnings, pluginmodel.Warning{Kind: pluginmodel.WarningFidelity, Path: "settings.json", Message: warning})
		}
	}
	if copied, warnings, err := importClaudeLSP(root, pluginManifest); err != nil {
		return err
	} else {
		result.Artifacts = append(result.Artifacts, copied...)
		result.Warnings = append(result.Warnings, warnings...)
	}
	if pluginManifest.UserConfigProvided {
		result.Artifacts = append(result.Artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "user-config.json"),
			Content: mustJSON(pluginManifest.UserConfig),
		})
	}
	if copied, warnings, err := importClaudeMCP(root, pluginManifest); err != nil {
		return err
	} else {
		result.Artifacts = append(result.Artifacts, copied...)
		result.Warnings = append(result.Warnings, warnings...)
	}
	if len(pluginManifest.Extra) > 0 {
		result.Artifacts = append(result.Artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "manifest.extra.json"),
			Content: mustJSON(pluginManifest.Extra),
		})
		result.Warnings = append(result.Warnings, pluginmodel.Warning{
			Kind:    pluginmodel.WarningFidelity,
			Path:    filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "manifest.extra.json")),
			Message: "preserved unsupported Claude manifest fields under targets/claude/manifest.extra.json",
		})
	}
	return nil
}
