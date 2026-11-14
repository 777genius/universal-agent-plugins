package pluginkitairepo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginKitAITestClaudeShellStableFlow(t *testing.T) {
	if !shellRuntimeAvailable() {
		t.Skip("bash runtime not available for shell test flow")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	initCmd := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "claude", "--runtime", "shell", "-o", plugRoot)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init claude shell: %v\n%s", err, out)
	}

	match := exec.Command(pluginKitAIBin, "test", plugRoot, "--platform", "claude", "--all")
	out, err := match.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai test claude match: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{
		"PASS claude/Stop",
		"PASS claude/PreToolUse",
		"PASS claude/UserPromptSubmit",
		"golden=matched",
		"Summary: total=3 passed=3 failed=0",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("test output missing %q:\n%s", want, text)
		}
	}
	for _, rel := range []string{
		filepath.Join("goldens", "claude", "Stop.stdout"),
		filepath.Join("goldens", "claude", "PreToolUse.stdout"),
		filepath.Join("goldens", "claude", "UserPromptSubmit.stdout"),
		filepath.Join("fixtures", "claude", "Stop.json"),
		filepath.Join("fixtures", "claude", "PreToolUse.json"),
		filepath.Join("fixtures", "claude", "UserPromptSubmit.json"),
	} {
		if _, err := os.Stat(filepath.Join(plugRoot, rel)); err != nil {
			t.Fatalf("expected scaffolded runtime test asset %s: %v", rel, err)
		}
	}
}

func TestPluginKitAITestCodexShellNotifyJSONFlow(t *testing.T) {
	if !shellRuntimeAvailable() {
		t.Skip("bash runtime not available for shell test flow")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	initCmd := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "shell", "-o", plugRoot)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init codex shell: %v\n%s", err, out)
	}

	match := exec.Command(pluginKitAIBin, "test", plugRoot, "--platform", "codex-runtime", "--event", "Notify", "--format", "json")
	out, err := match.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai test codex json: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{
		`"passed": true`,
		`"event": "Notify"`,
		`"golden_status": "matched"`,
		`"summary": {`,
		`"golden_matched": 1`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("json output missing %q:\n%s", want, text)
		}
	}
	for _, rel := range []string{
		filepath.Join("fixtures", "codex-runtime", "Notify.json"),
		filepath.Join("goldens", "codex-runtime", "Notify.exitcode"),
	} {
		if _, err := os.Stat(filepath.Join(plugRoot, rel)); err != nil {
			t.Fatalf("expected scaffolded runtime test asset %s: %v", rel, err)
		}
	}
}
