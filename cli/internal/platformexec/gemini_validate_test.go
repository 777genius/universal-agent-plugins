package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestGeminiValidateRejectsExtensionSettingsWithoutAuthoredSettings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	name := filepath.Base(root)

	writeGeminiValidateFile(t, filepath.Join(root, "targets", "gemini", "package.yaml"), "{}\n")
	writeGeminiValidateFile(t, filepath.Join(root, "gemini-extension.json"), `{
  "name": "`+name+`",
  "version": "0.1.0",
  "description": "demo",
  "settings": [
    {"name":"API Key","description":"Demo key","envVar":"DEMO_API_KEY","sensitive":true}
  ]
}
`)

	state := pluginmodel.NewTargetState("gemini")
	state.SetDoc("package_metadata", filepath.Join("targets", "gemini", "package.yaml"))

	diagnostics, err := (geminiAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: name, Version: "0.1.0", Description: "demo"},
	}, state)
	if err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	joined := diagnosticsText(diagnostics)
	if !strings.Contains(joined, "may not define settings when targets/gemini/settings/** is absent") {
		t.Fatalf("diagnostics missing settings projection failure:\n%s", joined)
	}
}

func TestGeminiValidateReportsHookEntrypointMismatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	name := filepath.Base(root)
	authoredHooks := filepath.Join("targets", "gemini", "hooks", "hooks.json")

	writeGeminiValidateFile(t, filepath.Join(root, "targets", "gemini", "package.yaml"), "{}\n")
	writeGeminiValidateFile(t, filepath.Join(root, "gemini-extension.json"), `{
  "name": "`+name+`",
  "version": "0.1.0",
  "description": "demo"
}
`)
	writeGeminiValidateFile(t, filepath.Join(root, authoredHooks), string(defaultGeminiHooks("./bin/other")))
	writeGeminiValidateFile(t, filepath.Join(root, "hooks", "hooks.json"), string(defaultGeminiHooks("./bin/other")))

	state := pluginmodel.NewTargetState("gemini")
	state.SetDoc("package_metadata", filepath.Join("targets", "gemini", "package.yaml"))
	state.AddComponent("hooks", authoredHooks)

	diagnostics, err := (geminiAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: name, Version: "0.1.0", Description: "demo"},
		Launcher: &pluginmodel.Launcher{Runtime: "go", Entrypoint: "./bin/demo"},
	}, state)
	if err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	joined := diagnosticsText(diagnostics)
	if !strings.Contains(joined, "entrypoint mismatch") {
		t.Fatalf("diagnostics missing hook entrypoint mismatch:\n%s", joined)
	}
}

func writeGeminiValidateFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
