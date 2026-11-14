package platformexec

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (codexPackageAdapter) Import(root string, seed ImportSeed) (ImportResult, error) {
	result := ImportResult{Manifest: seed.Manifest, Launcher: nil}
	pluginManifest, _, err := readImportedCodexPluginManifest(root)
	if err != nil {
		return ImportResult{}, err
	}
	if err := validateImportedCodexPackageBundle(root, pluginManifest); err != nil {
		return ImportResult{}, err
	}
	result.Artifacts = append(result.Artifacts, pluginmodel.Artifact{
		RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "package.yaml"),
		Content: mustYAML(pluginManifest.PackageMeta),
	})
	result.Manifest = mergeImportedCodexPackageManifest(result.Manifest, pluginManifest)
	extra := cloneStringMap(pluginManifest.Extra)
	if err := appendImportedCodexPackageBundleArtifacts(&result, root, pluginManifest, extra); err != nil {
		return ImportResult{}, err
	}
	appendImportedCodexPackageWarnings(&result, root, pluginManifest, extra)
	result.Artifacts = compactArtifacts(result.Artifacts)
	return result, nil
}
