package main

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func shouldDiagnoseGeneratedPublicationArtifacts(root string) bool {
	return fileExists(filepath.Join(root, pluginmodel.SourceDirName, pluginmanifest.FileName))
}
