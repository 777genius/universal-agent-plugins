package platformexec

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/pelletier/go-toml/v2"
)

func (codexRuntimeAdapter) Import(root string, seed ImportSeed) (ImportResult, error) {
	result := ImportResult{
		Manifest: seed.Manifest,
		Launcher: seed.Launcher,
	}
	config, _, err := readImportedCodexConfig(root)
	if err != nil {
		return ImportResult{}, err
	}
	result.Artifacts = append(result.Artifacts, importedCodexRuntimePackageArtifact(config))
	if len(config.Extra) > 0 {
		body, err := toml.Marshal(config.Extra)
		if err != nil {
			return ImportResult{}, err
		}
		result.Artifacts = append(result.Artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "config.extra.toml"),
			Content: body,
		})
		result.Warnings = append(result.Warnings, pluginmodel.Warning{
			Kind:    pluginmodel.WarningFidelity,
			Path:    filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "config.extra.toml")),
			Message: "preserved unsupported Codex config fields under targets/codex-runtime/config.extra.toml",
		})
	}
	if len(config.Notify) > 0 && result.Launcher != nil && strings.TrimSpace(config.Notify[0]) != "" {
		result.Launcher.Entrypoint = strings.TrimSpace(config.Notify[0])
	}
	if len(config.Notify) > 0 && !pluginmodel.IsCanonicalCodexNotify(config.Notify) {
		result.Warnings = append(result.Warnings, pluginmodel.Warning{
			Kind:    pluginmodel.WarningFidelity,
			Path:    filepath.ToSlash(filepath.Join(".codex", "config.toml")),
			Message: "normalized Codex notify argv to the managed [entrypoint, \"notify\"] shape",
		})
	}
	copied, err := copyArtifactDirs(root,
		artifactDir{src: "commands", dst: filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "commands")},
		artifactDir{src: "contexts", dst: filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "contexts")},
	)
	if err != nil {
		return ImportResult{}, err
	}
	result.Artifacts = append(result.Artifacts, copied...)
	result.Artifacts = compactArtifacts(result.Artifacts)
	return result, nil
}

func importedCodexRuntimePackageArtifact(config importedCodexNativeConfig) pluginmodel.Artifact {
	meta := codexRuntimeMeta{}
	if strings.TrimSpace(config.Model) != "" {
		meta.ModelHint = config.Model
	}
	return pluginmodel.Artifact{
		RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "package.yaml"),
		Content: mustYAML(meta),
	}
}
