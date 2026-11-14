package platformexec

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (qoderAdapter) Import(root string, seed ImportSeed) (ImportResult, error) {
	result := ImportResult{
		Manifest: seed.Manifest,
		Launcher: seed.Launcher,
	}
	manifest, ok, err := readImportedQoderPluginManifest(root)
	if err != nil {
		return ImportResult{}, err
	}
	if !ok {
		return ImportResult{}, fmt.Errorf("qoder plugin import requires %s", qoderPluginManifestPath)
	}
	if name := stringMapField(manifest, "name"); name != "" {
		result.Manifest.Name = name
	}
	if version := stringMapField(manifest, "version"); version != "" {
		result.Manifest.Version = version
	}
	if description := stringMapField(manifest, "description"); description != "" {
		result.Manifest.Description = description
	}
	if authorName := qoderPluginAuthorName(manifest); authorName != "" {
		result.Manifest.Author = &pluginmodel.Author{Name: authorName}
	}
	if servers, ok, err := importedQoderPluginMCP(root, manifest); err != nil {
		return ImportResult{}, err
	} else if ok {
		artifact, err := importedPortableMCPArtifact("qoder", servers)
		if err != nil {
			return ImportResult{}, err
		}
		result.Artifacts = append(result.Artifacts, artifact)
	}
	skillArtifacts, err := copyArtifactDirs(root,
		artifactDir{src: "skills", dst: "skills"},
		artifactDir{src: "hooks", dst: "targets/qoder/hooks"},
		artifactDir{src: "commands", dst: "targets/qoder/commands"},
		artifactDir{src: "bin", dst: "targets/qoder/bin"},
	)
	if err != nil {
		return ImportResult{}, err
	}
	result.Artifacts = append(result.Artifacts, skillArtifacts...)
	appendQoderUnsupportedAssetWarnings(&result, root)
	result.Artifacts = compactArtifacts(result.Artifacts)
	return result, nil
}

func appendQoderUnsupportedAssetWarnings(result *ImportResult, root string) {
	for _, asset := range []string{"agents", "output-styles"} {
		if !fileExists(filepath.Join(root, asset)) {
			continue
		}
		result.Warnings = append(result.Warnings, pluginmodel.Warning{
			Kind:    pluginmodel.WarningIgnoredImport,
			Path:    asset,
			Message: "ignored unsupported import asset: " + asset,
		})
	}
}
