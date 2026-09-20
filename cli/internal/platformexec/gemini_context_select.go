package platformexec

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func selectGeminiPrimaryContext(graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) (geminiContextSelection, bool, error) {
	candidates := geminiContextCandidates(graph, state)
	selected := strings.TrimSpace(meta.ContextFileName)
	if selected != "" {
		return selectNamedGeminiPrimaryContext(candidates, selected)
	}
	return selectFallbackGeminiPrimaryContext(candidates)
}

func selectNamedGeminiPrimaryContext(candidates []geminiContextSelection, selected string) (geminiContextSelection, bool, error) {
	matches := candidatesByArtifactName(candidates, selected)
	switch len(matches) {
	case 0:
		return geminiContextSelection{}, false, fmt.Errorf("gemini context_file_name %q does not resolve to a Gemini-native context source", selected)
	case 1:
		return matches[0], true, nil
	default:
		return geminiContextSelection{}, false, fmt.Errorf("gemini context_file_name %q is ambiguous across multiple context sources", selected)
	}
}

func selectFallbackGeminiPrimaryContext(candidates []geminiContextSelection) (geminiContextSelection, bool, error) {
	fallback := fallbackGeminiPrimaryContextCandidates(candidates)
	switch len(fallback) {
	case 1:
		return fallback[0], true, nil
	case 0:
		return selectOnlyGeminiPrimaryContextCandidate(candidates)
	default:
		return geminiContextSelection{}, false, fmt.Errorf("gemini primary context selection is ambiguous for GEMINI.md; keep one root context or set context_file_name explicitly")
	}
}

func fallbackGeminiPrimaryContextCandidates(candidates []geminiContextSelection) []geminiContextSelection {
	return candidatesByArtifactName(candidates, "GEMINI.md")
}

func selectOnlyGeminiPrimaryContextCandidate(candidates []geminiContextSelection) (geminiContextSelection, bool, error) {
	if len(candidates) == 0 {
		return geminiContextSelection{}, false, nil
	}
	if len(candidates) == 1 {
		return candidates[0], true, nil
	}
	return geminiContextSelection{}, false, fmt.Errorf("gemini primary context selection is ambiguous; set targets/gemini/package.yaml context_file_name explicitly")
}
