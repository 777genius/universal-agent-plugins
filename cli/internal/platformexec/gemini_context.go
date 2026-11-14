package platformexec

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func geminiContextCandidates(graph pluginmodel.PackageGraph, state pluginmodel.TargetState) []geminiContextSelection {
	return collectSortedGeminiContextCandidates(state)
}

func validateGeminiContext(graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) []Diagnostic {
	selected := strings.TrimSpace(meta.ContextFileName)
	candidates := geminiContextMatches(graph, state, "")
	if selected != "" {
		return validateNamedGeminiContextSelection(state, selected, geminiContextMatches(graph, state, selected))
	}
	return validateDefaultGeminiContextSelection(candidates, geminiContextMatches(graph, state, "GEMINI.md"))
}
