package pluginkitairepo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginKitAIValidateWarnsButSucceedsOnExtraPluginYAMLFields(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := t.TempDir()

	initCmd := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "-o", plugRoot)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
	}
	bootstrapGeneratedGoPlugin(t, plugRoot)

	manifestPath := filepath.Join(plugRoot, authoredRel("plugin.yaml"))
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, []byte("extra_field: true\n")...)
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	validateCmd := exec.Command(pluginKitAIBin, "validate", plugRoot, "--platform", "codex-runtime")
	validateCmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := validateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai validate: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "Warning: unknown plugin.yaml field: extra_field") {
		t.Fatalf("validate output missing warning:\n%s", text)
	}
	if !strings.Contains(text, "Validated "+plugRoot) {
		t.Fatalf("validate output missing success summary:\n%s", text)
	}
}
