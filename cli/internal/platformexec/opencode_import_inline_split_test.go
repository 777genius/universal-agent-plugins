package platformexec

import "testing"

func TestCanonicalOpenCodeNamedMarkdownPathRejectsTraversal(t *testing.T) {
	t.Parallel()
	if _, ok := canonicalOpenCodeNamedMarkdownPath("agents", "../planner"); ok {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestNormalizeInlineOpenCodeCommandRejectsUnsupportedFields(t *testing.T) {
	t.Parallel()
	_, _, ok := normalizeInlineOpenCodeCommand("demo", map[string]any{
		"template":    "echo demo",
		"temperature": 1.0,
	})
	if ok {
		t.Fatal("expected unsupported command field to be rejected")
	}
}
