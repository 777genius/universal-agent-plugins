package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestLoadGeminiSettingsRejectsDuplicateEnvVar(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join("targets", "gemini", "settings", "first.yaml")
	second := filepath.Join("targets", "gemini", "settings", "second.yaml")
	writeGeminiHelperFile(t, root, first, "name: Alpha\ndescription: First\nenv_var: DEMO_TOKEN\nsensitive: true\n")
	writeGeminiHelperFile(t, root, second, "name: Beta\ndescription: Second\nenv_var: DEMO_TOKEN\nsensitive: false\n")

	_, err := loadGeminiSettings(root, []string{first, second})
	if err == nil {
		t.Fatal("expected duplicate env_var error")
	}
	if !strings.Contains(err.Error(), `Gemini setting env_var "DEMO_TOKEN" duplicates `+first) {
		t.Fatalf("duplicate env_var error = %v", err)
	}
}

func TestImportedGeminiThemeArtifactsUsesCollisionSafeSlugs(t *testing.T) {
	artifacts := importedGeminiThemeArtifacts([]any{
		map[string]any{"name": "Aurora"},
		map[string]any{"name": "Aurora"},
		map[string]any{"name": ""},
	})
	if len(artifacts) != 3 {
		t.Fatalf("artifact count = %d", len(artifacts))
	}
	if got := filepath.ToSlash(artifacts[0].RelPath); got != filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "themes", "aurora.yaml")) {
		t.Fatalf("artifact[0].RelPath = %q", got)
	}
	if got := filepath.ToSlash(artifacts[1].RelPath); got != filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "themes", "aurora-2.yaml")) {
		t.Fatalf("artifact[1].RelPath = %q", got)
	}
	if got := filepath.ToSlash(artifacts[2].RelPath); got != filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "themes", "item.yaml")) {
		t.Fatalf("artifact[2].RelPath = %q", got)
	}
}

func writeGeminiHelperFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
