package platformexec

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGeminiThemesRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	first := filepath.Join("targets", "gemini", "themes", "first.yaml")
	second := filepath.Join("targets", "gemini", "themes", "second.yaml")
	writeGeminiHelperFile(t, root, first, "name: Aurora\ncolors:\n  primary: '#fff'\n")
	writeGeminiHelperFile(t, root, second, "name: aurora\ncolors:\n  primary: '#000'\n")

	_, err := loadGeminiThemes(root, []string{first, second})
	if err == nil || !strings.Contains(err.Error(), `Gemini theme name "aurora" duplicates `+first) {
		t.Fatalf("error = %v", err)
	}
}

func TestNormalizeGeminiThemeMapSkipsBlankKeys(t *testing.T) {
	t.Parallel()

	theme := normalizeGeminiThemeMap(map[string]any{
		"":      "ignored",
		"name":  "Aurora",
		"tones": map[string]any{"primary": "#fff"},
	})
	if len(theme) != 2 || theme["name"] != "Aurora" {
		t.Fatalf("theme = %#v", theme)
	}
}
