package platformexec

import (
	"fmt"
)

func (cursorAdapter) Import(root string, seed ImportSeed) (ImportResult, error) {
	result := ImportResult{
		Manifest: seed.Manifest,
		Launcher: seed.Launcher,
	}
	manifest, ok, err := readImportedCursorPluginManifest(root)
	if err != nil {
		return ImportResult{}, err
	}
	if !ok {
		return ImportResult{}, fmt.Errorf("Cursor plugin import requires %s", cursorPluginManifestPath)
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
	if servers, ok, err := importedCursorPluginMCP(root, manifest); err != nil {
		return ImportResult{}, err
	} else if ok {
		artifact, err := importedPortableMCPArtifact("cursor", servers)
		if err != nil {
			return ImportResult{}, err
		}
		result.Artifacts = append(result.Artifacts, artifact)
	}
	skillArtifacts, err := copyArtifactDirs(root, artifactDir{src: "skills", dst: "skills"})
	if err != nil {
		return ImportResult{}, err
	}
	result.Artifacts = append(result.Artifacts, skillArtifacts...)
	result.Artifacts = compactArtifacts(result.Artifacts)
	return result, nil
}
