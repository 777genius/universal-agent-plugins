package platformexec

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func geminiContextArtifacts(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) (string, pluginmodel.Artifact, []pluginmodel.Artifact, bool, error) {
	selected, ok, err := resolveGeminiContextArtifactSelection(graph, state, meta)
	if err != nil {
		return "", pluginmodel.Artifact{}, nil, false, err
	}
	if !ok {
		return "", pluginmodel.Artifact{}, nil, false, nil
	}
	primary, err := readGeminiPrimaryContextArtifact(root, selected)
	if err != nil {
		return "", pluginmodel.Artifact{}, nil, false, err
	}
	extra, err := readGeminiExtraContextArtifacts(root, state, selected)
	if err != nil {
		return "", pluginmodel.Artifact{}, nil, false, err
	}
	return selected.ArtifactName, primary, extra, true, nil
}

func resolveGeminiContextArtifactSelection(graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) (geminiContextSelection, bool, error) {
	return selectGeminiPrimaryContext(graph, state, meta)
}

func readGeminiPrimaryContextArtifact(root string, selected geminiContextSelection) (pluginmodel.Artifact, error) {
	body, err := os.ReadFile(filepath.Join(root, selected.SourcePath))
	if err != nil {
		return pluginmodel.Artifact{}, err
	}
	return pluginmodel.Artifact{RelPath: selected.ArtifactName, Content: body}, nil
}

func readGeminiExtraContextArtifacts(root string, state pluginmodel.TargetState, selected geminiContextSelection) ([]pluginmodel.Artifact, error) {
	var extra []pluginmodel.Artifact
	for _, rel := range state.ComponentPaths("contexts") {
		if rel == selected.SourcePath {
			continue
		}
		artifact, err := readGeminiExtraContextArtifact(root, rel)
		if err != nil {
			return nil, err
		}
		extra = append(extra, artifact)
	}
	return sortGeminiContextArtifacts(extra), nil
}

func readGeminiExtraContextArtifact(root, rel string) (pluginmodel.Artifact, error) {
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return pluginmodel.Artifact{}, err
	}
	return pluginmodel.Artifact{
		RelPath: geminiExtraContextArtifactPath(rel),
		Content: body,
	}, nil
}

func sortGeminiContextArtifacts(extra []pluginmodel.Artifact) []pluginmodel.Artifact {
	slices.SortFunc(extra, func(a, b pluginmodel.Artifact) int { return strings.Compare(a.RelPath, b.RelPath) })
	return extra
}
