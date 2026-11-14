package app

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestBuildExportResultBaseLinesUsesIncludedFileCount(t *testing.T) {
	t.Parallel()

	lines := buildExportResultBaseLines(exportServiceContext{
		project: runtimecheck.Project{Root: "/tmp/out", Runtime: "node"},
	}, exportArchivePlan{
		outputPath: "/tmp/out/demo.tar.gz",
		files:      []string{"a", "b"},
	})
	if !strings.Contains(strings.Join(lines, "\n"), "Included files: 3") {
		t.Fatalf("lines = %#v", lines)
	}
}

func TestAppendExportResultNextLinesIncludesValidateStep(t *testing.T) {
	t.Parallel()

	lines := appendExportResultNextLines([]string{"base"}, "claude", "/tmp/out/demo.tar.gz")
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "plugin-kit-ai validate . --platform claude --strict") {
		t.Fatalf("lines = %#v", lines)
	}
}
