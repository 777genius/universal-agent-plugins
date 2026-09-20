package app

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeTestPreviewTruncatesEscapedNewlines(t *testing.T) {
	t.Parallel()

	got := runtimeTestPreview(strings.Repeat("line\n", 40))
	if !strings.Contains(got, `\n`) || !strings.HasSuffix(got, `..."`) {
		t.Fatalf("preview = %q", got)
	}
}

func TestFormatRuntimeTestCaseDetailsIncludesGoldenFileLabel(t *testing.T) {
	t.Parallel()

	lines := formatRuntimeTestCaseDetails(PluginTestCase{
		MismatchInfo: []PluginTestMismatch{{
			Field:           "stdout",
			GoldenFile:      filepath.Join("goldens", "claude", "Stop.stdout"),
			ExpectedPreview: `"a"`,
			ActualPreview:   `"b"`,
		}},
	})
	if len(lines) != 1 || !strings.Contains(lines[0], "stdout (goldens") {
		t.Fatalf("lines = %#v", lines)
	}
}
