package platformexec

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func collectSortedGeminiContextCandidates(state pluginmodel.TargetState) []geminiContextSelection {
	out := collectGeminiContextCandidates(state)
	sortGeminiContextCandidates(out)
	return out
}

func collectGeminiContextCandidates(state pluginmodel.TargetState) []geminiContextSelection {
	var out []geminiContextSelection
	seen := map[string]struct{}{}
	for _, rel := range state.ComponentPaths("contexts") {
		out = appendGeminiContextCandidate(out, seen, rel)
	}
	return out
}

func appendGeminiContextCandidate(out []geminiContextSelection, seen map[string]struct{}, rel string) []geminiContextSelection {
	artifactName := filepath.Base(rel)
	if artifactName == "" {
		return out
	}
	if _, ok := seen[rel]; ok {
		return out
	}
	seen[rel] = struct{}{}
	return append(out, geminiContextSelection{ArtifactName: artifactName, SourcePath: rel})
}

func sortGeminiContextCandidates(out []geminiContextSelection) {
	slices.SortFunc(out, func(a, b geminiContextSelection) int {
		if cmp := strings.Compare(a.ArtifactName, b.ArtifactName); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.SourcePath, b.SourcePath)
	})
}

func candidatesByArtifactName(candidates []geminiContextSelection, name string) []geminiContextSelection {
	var out []geminiContextSelection
	for _, candidate := range candidates {
		if candidate.ArtifactName == name {
			out = append(out, candidate)
		}
	}
	return out
}

func geminiContextMatches(graph pluginmodel.PackageGraph, state pluginmodel.TargetState, name string) []string {
	var matches []string
	seen := map[string]struct{}{}
	for _, rel := range state.ComponentPaths("contexts") {
		rel = filepath.ToSlash(rel)
		if name == "" || filepath.Base(rel) == name {
			if _, ok := seen[rel]; ok {
				continue
			}
			seen[rel] = struct{}{}
			matches = append(matches, rel)
		}
	}
	slices.Sort(matches)
	return matches
}
