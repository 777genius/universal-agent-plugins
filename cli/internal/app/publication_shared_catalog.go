package app

import (
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationexec"
)

func mergeCatalogAtDestination(dest, target string, generated pluginmanifest.Artifact) ([]byte, error) {
	full := filepath.Join(dest, filepath.FromSlash(generated.RelPath))
	existing, err := os.ReadFile(full)
	if err == nil {
		return publicationexec.MergeCatalogArtifact(target, existing, generated.Content)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	return generated.Content, nil
}
