package platformexec

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateGeminiSettingsReportsDuplicateEnvVar(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeGeminiValidateFile(t, filepath.Join(root, "targets", "gemini", "settings", "one.yaml"), "name: One\ndescription: First\nenv_var: DEMO_API_KEY\nsensitive: true\n")
	writeGeminiValidateFile(t, filepath.Join(root, "targets", "gemini", "settings", "two.yaml"), "name: Two\ndescription: Second\nenv_var: DEMO_API_KEY\nsensitive: false\n")

	diagnostics := validateGeminiSettings(root, []string{
		filepath.Join("targets", "gemini", "settings", "one.yaml"),
		filepath.Join("targets", "gemini", "settings", "two.yaml"),
	})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `duplicates env_var "DEMO_API_KEY" already declared`) {
		t.Fatalf("diagnostics missing duplicate env_var:\n%s", text)
	}
}

func TestValidateGeminiThemesReportsDuplicateThemeName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeGeminiValidateFile(t, filepath.Join(root, "targets", "gemini", "themes", "one.yaml"), "name: Demo\nbackground:\n  primary: '#000000'\ntext:\n  primary: '#ffffff'\nstatus:\n  info: '#ffffff'\nui:\n  border: '#222222'\n")
	writeGeminiValidateFile(t, filepath.Join(root, "targets", "gemini", "themes", "two.yaml"), "name: Demo\nbackground:\n  primary: '#111111'\ntext:\n  primary: '#eeeeee'\nstatus:\n  info: '#dddddd'\nui:\n  border: '#333333'\n")

	diagnostics := validateGeminiThemes(root, []string{
		filepath.Join("targets", "gemini", "themes", "one.yaml"),
		filepath.Join("targets", "gemini", "themes", "two.yaml"),
	})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `duplicates theme name "Demo" already declared`) {
		t.Fatalf("diagnostics missing duplicate theme name:\n%s", text)
	}
}
