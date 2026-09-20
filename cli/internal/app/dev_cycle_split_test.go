package app

import (
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestSelectDevRuntimeTests_RejectsFixtureWithAll(t *testing.T) {
	t.Parallel()

	var lines []string
	selected, ok := selectDevRuntimeTests("claude", PluginDevOptions{
		All:     true,
		Fixture: "fixtures/claude/Stop.json",
	}, &lines)
	if ok {
		t.Fatalf("selected = %v, want failure", selected)
	}
	if !slices.Equal(lines, []string{"--fixture cannot be used with --all"}) {
		t.Fatalf("lines = %v", lines)
	}
}

func TestRuntimeStatusLine_FormatsDiagnosis(t *testing.T) {
	t.Parallel()

	got := runtimeStatusLine(runtimecheck.Diagnosis{
		Status: runtimecheck.StatusReady,
		Reason: "launcher and runtime validated",
	})
	if got != "Runtime: ready (launcher and runtime validated)" {
		t.Fatalf("runtimeStatusLine() = %q", got)
	}
}
