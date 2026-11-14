package app

import "testing"

func TestNormalizeExportPathTrimsDotPrefix(t *testing.T) {
	t.Parallel()

	if got := normalizeExportPath("./bin/demo"); got != "bin/demo" {
		t.Fatalf("normalizeExportPath() = %q", got)
	}
}

func TestRelWithinRootRejectsParentPath(t *testing.T) {
	t.Parallel()

	if rel, ok := relWithinRoot("/tmp/root", "/tmp/other"); ok || rel != "" {
		t.Fatalf("relWithinRoot() = (%q, %v)", rel, ok)
	}
}
