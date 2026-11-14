package platformexec

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateGeminiPoliciesWarnsOnIgnoredAllow(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := filepath.Join("targets", "gemini", "policies", "policy.toml")
	writeGeminiValidateFile(t, filepath.Join(root, rel), "allow = true\n")

	diagnostics := validateGeminiPolicies(root, []string{rel})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `ignore "allow" at extension tier`) {
		t.Fatalf("diagnostics missing allow warning:\n%s", text)
	}
}

func TestValidateGeminiCommandsRejectsNonTOMLExtension(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := filepath.Join("targets", "gemini", "commands", "demo.yaml")
	writeGeminiValidateFile(t, filepath.Join(root, rel), "name: demo\n")

	diagnostics := validateGeminiCommands(root, []string{rel})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, "must use the .toml extension") {
		t.Fatalf("diagnostics missing extension failure:\n%s", text)
	}
}
