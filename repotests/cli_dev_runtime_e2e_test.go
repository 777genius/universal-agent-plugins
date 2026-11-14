package pluginkitairepo_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPluginKitAIDevOnceClaudeShellFlow(t *testing.T) {
	if !shellRuntimeAvailable() {
		t.Skip("bash runtime not available for shell dev flow")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	initCmd := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "claude", "--runtime", "shell", "-o", plugRoot)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init claude shell: %v\n%s", err, out)
	}

	dev := exec.Command(pluginKitAIBin, "dev", plugRoot, "--once", "--platform", "claude", "--event", "Stop")
	out, err := dev.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai dev --once: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{
		"Cycle 1 [initial]",
		"Generate: wrote",
		"Validate: ok",
		"PASS claude/Stop",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("dev output missing %q:\n%s", want, text)
		}
	}
}
