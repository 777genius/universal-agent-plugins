package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func materializedPackageArtifacts(root, packageRoot string, managedPaths []string, generated pluginmanifest.RenderResult) ([]pluginmanifest.Artifact, error) {
	renderedBodies := make(map[string][]byte, len(generated.Artifacts))
	for _, artifact := range generated.Artifacts {
		renderedBodies[filepath.ToSlash(artifact.RelPath)] = artifact.Content
	}
	out := make([]pluginmanifest.Artifact, 0, len(managedPaths))
	for _, rel := range managedPaths {
		var body []byte
		if generated, ok := renderedBodies[rel]; ok {
			body = append([]byte(nil), generated...)
		} else {
			sourceBody, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("read managed package artifact %s: %w", rel, err)
			}
			body = sourceBody
		}
		out = append(out, pluginmanifest.Artifact{
			RelPath: filepath.ToSlash(filepath.Join(packageRoot, rel)),
			Content: body,
		})
	}
	slices.SortFunc(out, func(a, b pluginmanifest.Artifact) int { return strings.Compare(a.RelPath, b.RelPath) })
	return out, nil
}
