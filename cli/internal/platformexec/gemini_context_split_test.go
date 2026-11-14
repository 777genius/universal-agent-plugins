package platformexec

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestGeminiContextCandidatesDedupesAndSorts(t *testing.T) {
	t.Parallel()

	state := pluginmodel.NewTargetState("gemini")
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "beta.md"))
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "alpha.md"))
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "alpha.md"))

	got := geminiContextCandidates(pluginmodel.PackageGraph{}, state)
	if len(got) != 2 {
		t.Fatalf("candidates = %+v", got)
	}
	if got[0].ArtifactName != "alpha.md" || got[1].ArtifactName != "beta.md" {
		t.Fatalf("candidates = %+v", got)
	}
}

func TestValidateNamedGeminiContextSelectionRejectsAmbiguousMatches(t *testing.T) {
	t.Parallel()

	state := pluginmodel.NewTargetState("gemini")
	diagnostics := validateNamedGeminiContextSelection(state, "GEMINI.md", []string{"a/GEMINI.md", "b/GEMINI.md"})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `context_file_name "GEMINI.md" is ambiguous`) {
		t.Fatalf("diagnostics = %s", text)
	}
}

func TestValidateDefaultGeminiContextSelectionAllowsSingleCandidate(t *testing.T) {
	t.Parallel()

	if diagnostics := validateDefaultGeminiContextSelection([]string{"targets/gemini/contexts/GEMINI.md"}, nil); len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestSelectNamedGeminiPrimaryContextReturnsUniqueMatch(t *testing.T) {
	t.Parallel()

	got, ok, err := selectNamedGeminiPrimaryContext([]geminiContextSelection{{
		ArtifactName: "GEMINI.md",
		SourcePath:   "targets/gemini/contexts/GEMINI.md",
	}}, "GEMINI.md")
	if err != nil || !ok {
		t.Fatalf("selection err = %v ok = %v", err, ok)
	}
	if got.ArtifactName != "GEMINI.md" {
		t.Fatalf("selection = %+v", got)
	}
}

func TestSelectOnlyGeminiPrimaryContextCandidateRejectsMultipleCandidates(t *testing.T) {
	t.Parallel()

	_, ok, err := selectOnlyGeminiPrimaryContextCandidate([]geminiContextSelection{
		{ArtifactName: "alpha.md", SourcePath: "alpha.md"},
		{ArtifactName: "beta.md", SourcePath: "beta.md"},
	})
	if err == nil || ok {
		t.Fatalf("selection err = %v ok = %v", err, ok)
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("error = %v", err)
	}
}

func TestGeminiContextMatchesFiltersAndSortsByArtifactName(t *testing.T) {
	t.Parallel()

	state := pluginmodel.NewTargetState("gemini")
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "beta.md"))
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "alpha.md"))
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "beta.md"))

	got := geminiContextMatches(pluginmodel.PackageGraph{}, state, "beta.md")
	want := []string{filepath.ToSlash(filepath.Join("targets", "gemini", "contexts", "beta.md"))}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matches = %#v want %#v", got, want)
	}
}
