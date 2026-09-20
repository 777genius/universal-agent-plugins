package platformexec

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func appendImportedGeminiExtensionArtifacts(root string, data importedGeminiExtension, result *ImportResult) error {
	applyImportedGeminiManifest(data, result)
	if err := appendImportedGeminiMCP(data, result); err != nil {
		return err
	}
	appendImportedGeminiPackageDocs(data, result)
	appendImportedGeminiManifestExtra(data, result)
	return appendImportedGeminiPrimaryContext(root, data.Meta, result)
}

func applyImportedGeminiManifest(data importedGeminiExtension, result *ImportResult) {
	if strings.TrimSpace(data.Name) != "" {
		result.Manifest.Name = data.Name
	}
	if strings.TrimSpace(data.Version) != "" {
		result.Manifest.Version = data.Version
	}
	if strings.TrimSpace(data.Description) != "" {
		result.Manifest.Description = data.Description
	}
}

func appendImportedGeminiMCP(data importedGeminiExtension, result *ImportResult) error {
	if len(data.MCPServers) == 0 {
		return nil
	}
	artifact, err := importedPortableMCPArtifact("gemini", data.MCPServers)
	if err != nil {
		return err
	}
	result.Artifacts = append(result.Artifacts, artifact)
	return nil
}

func appendImportedGeminiPackageDocs(data importedGeminiExtension, result *ImportResult) {
	if body := importedGeminiPackageYAML(data.Meta); len(body) > 0 {
		result.Artifacts = append(result.Artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "package.yaml"),
			Content: body,
		})
	}
	result.Artifacts = append(result.Artifacts, importedGeminiSettingsArtifacts(data.Settings)...)
	result.Artifacts = append(result.Artifacts, importedGeminiThemeArtifacts(data.Themes)...)
}

func appendImportedGeminiManifestExtra(data importedGeminiExtension, result *ImportResult) {
	if len(data.Extra) == 0 {
		return
	}
	result.Artifacts = append(result.Artifacts, pluginmodel.Artifact{
		RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "manifest.extra.json"),
		Content: mustJSON(data.Extra),
	})
	result.Warnings = append(result.Warnings, pluginmodel.Warning{
		Kind:    pluginmodel.WarningFidelity,
		Path:    filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "manifest.extra.json")),
		Message: "preserved additional Gemini manifest fields under targets/gemini/manifest.extra.json",
	})
}

func appendImportedGeminiPrimaryContext(root string, meta geminiPackageMeta, result *ImportResult) error {
	contextName := importedGeminiPrimaryContextName(root, meta)
	if contextName == "" {
		return nil
	}
	contextArtifacts, err := copySingleArtifactIfExists(root, contextName, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", filepath.Base(contextName)))
	if err != nil {
		return err
	}
	result.Artifacts = append(result.Artifacts, contextArtifacts...)
	return nil
}

func importedGeminiPackageYAML(meta geminiPackageMeta) []byte {
	if len(meta.ExcludeTools) == 0 &&
		strings.TrimSpace(meta.ContextFileName) == "" &&
		strings.TrimSpace(meta.PlanDirectory) == "" {
		return nil
	}
	return mustYAML(meta)
}

func importedGeminiPrimaryContextName(root string, meta geminiPackageMeta) string {
	if strings.TrimSpace(meta.ContextFileName) != "" {
		return filepath.Base(strings.TrimSpace(meta.ContextFileName))
	}
	if fileExists(filepath.Join(root, "GEMINI.md")) {
		return "GEMINI.md"
	}
	return ""
}
