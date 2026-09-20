package platformexec

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateOpenCodeDefaultAgentRejectsBlankFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rel := filepath.ToSlash(filepath.Join("plugin", "targets", "opencode", "default-agent.txt"))
	writeOpenCodeValidateFile(t, filepath.Join(root, rel), "   \n")

	diagnostics := validateOpenCodeDefaultAgent(root, rel)
	joined := diagnosticsText(diagnostics)
	if !strings.Contains(joined, "must contain a non-empty agent name") {
		t.Fatalf("diagnostics missing blank default-agent failure:\n%s", joined)
	}
}
