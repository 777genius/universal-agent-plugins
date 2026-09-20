package platformexec

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (claudeAdapter) Import(root string, seed ImportSeed) (ImportResult, error) {
	result := ImportResult{
		Manifest: seed.Manifest,
		Launcher: seed.Launcher,
	}
	pluginManifest, _, manifestPresent, err := readImportedClaudePluginManifest(root)
	if err != nil {
		return ImportResult{}, err
	}
	appendImportedClaudeManifest(seed, pluginManifest, manifestPresent, &result)
	if copied, warnings, err := importClaudePortableSkills(root, pluginManifest); err != nil {
		return ImportResult{}, err
	} else {
		result.Artifacts = append(result.Artifacts, copied...)
		result.Warnings = append(result.Warnings, warnings...)
	}

	if hookArtifacts, hookBody, warnings, err := importClaudeHooks(root, pluginManifest); err != nil {
		return ImportResult{}, err
	} else {
		result.Warnings = append(result.Warnings, warnings...)
		if len(hookBody) > 0 {
			if entrypoint, ok := inferClaudeEntrypoint(hookBody); ok && result.Launcher == nil {
				result.Launcher = &pluginmodel.Launcher{
					Runtime:    "go",
					Entrypoint: entrypoint,
				}
			} else if ok {
				result.Launcher.Entrypoint = entrypoint
			} else {
				result.DroppedKinds = append(result.DroppedKinds, "hooks")
				result.Warnings = append(result.Warnings, pluginmodel.Warning{
					Kind:    pluginmodel.WarningFidelity,
					Path:    filepath.ToSlash(filepath.Join(".claude-plugin", "plugin.json")),
					Message: "Claude hooks were omitted from canonical package-standard import because their commands do not map to launcher.yaml entrypoint semantics",
				})
				hookArtifacts = nil
			}
		}
		result.Artifacts = append(result.Artifacts, hookArtifacts...)
	}

	if copied, warnings, err := importClaudeComponentRefs(root, "commands", filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "commands"), pluginManifest.CommandsOverride, pluginManifest.CommandsRefs); err != nil {
		return ImportResult{}, err
	} else {
		result.Artifacts = append(result.Artifacts, copied...)
		result.Warnings = append(result.Warnings, warnings...)
	}
	if copied, warnings, err := importClaudeComponentRefs(root, "agents", filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "agents"), pluginManifest.AgentsOverride, pluginManifest.AgentsRefs); err != nil {
		return ImportResult{}, err
	} else {
		result.Artifacts = append(result.Artifacts, copied...)
		result.Warnings = append(result.Warnings, warnings...)
	}
	if err := appendImportedClaudeNativeDocs(root, pluginManifest, manifestPresent, &result); err != nil {
		return ImportResult{}, err
	}
	result.Artifacts = compactArtifacts(result.Artifacts)
	return result, nil
}
