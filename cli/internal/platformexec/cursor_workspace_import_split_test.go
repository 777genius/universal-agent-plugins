package platformexec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCursorWorkspaceImportRejectsLegacyRepoRootRulesFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, removedCursorRulesFileName), []byte("# legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ensureCursorRulesMigrationGuard(root)
	if err == nil || err.Error() != "unsupported Cursor repo-root rules file: use .cursor/rules/*.mdc" {
		t.Fatalf("err = %v", err)
	}
}

func TestCursorManagedAgentsImportContentFallsBackToPlainRootBody(t *testing.T) {
	t.Parallel()

	body := "# Shared root\n\nUse full document.\n"
	got := cursorManagedAgentsImportContent(body)
	if got != "# Shared root\n\nUse full document." {
		t.Fatalf("content = %q", got)
	}
}
