package app

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func generateInitArtifacts(out string) error {
	if !shouldGenerateInitArtifacts(out) {
		return nil
	}
	generated, err := pluginmanifest.Generate(out, "all")
	if err != nil {
		return err
	}
	if err := pluginmanifest.WriteArtifacts(out, generated.Artifacts); err != nil {
		return err
	}
	if err := pluginmanifest.RemoveArtifacts(out, generated.StalePaths); err != nil {
		return err
	}
	return nil
}

func shouldGenerateInitArtifacts(out string) bool {
	return fileExists(filepath.Join(out, pluginmanifest.FileName)) ||
		fileExists(filepath.Join(out, pluginmodel.SourceDirName, pluginmanifest.FileName))
}
