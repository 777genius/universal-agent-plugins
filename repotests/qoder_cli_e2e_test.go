package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestQoderCLIPluginValidateInstallSmoke proves a scaffolded qoder bundle
// passes the real Qoder CLI conformance gate and round-trips through the CLI
// plugin lifecycle: validate --strict, install, list, uninstall.
func TestQoderCLIPluginValidateInstallSmoke(t *testing.T) {
	qoderBin := qoderBinaryOrSkip(t)
	pluginKitAIBin := buildPluginKitAI(t)
	proj := filepath.Join(t.TempDir(), "qoder-smoke")

	initOut, err := exec.Command(pluginKitAIBin, "init", "qoder-smoke", "--platform", "qoder", "--extras", "-o", proj).CombinedOutput()
	if err != nil {
		t.Fatalf("init qoder starter: %v\n%s", err, initOut)
	}
	for _, rel := range []string{
		filepath.Join(".qoder-plugin", "plugin.json"),
		filepath.Join("hooks", "hooks.json"),
		filepath.Join("commands", "qoder-smoke.md"),
		filepath.Join("bin", "qoder-smoke-hook"),
	} {
		if _, err := os.Stat(filepath.Join(proj, rel)); err != nil {
			t.Fatalf("starter missing %s: %v", rel, err)
		}
	}

	validateOut, err := exec.Command(qoderBin, "plugins", "validate", proj, "--strict").CombinedOutput()
	if err != nil {
		t.Fatalf("qoder plugins validate --strict: %v\n%s", err, validateOut)
	}

	installOut, err := exec.Command(qoderBin, "plugins", "install", proj, "--scope", "user", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("qoder plugins install: %v\n%s", err, installOut)
	}
	pluginName := qoderInstalledPluginName(installOut, "qoder-smoke")
	t.Cleanup(func() {
		out, err := exec.Command(qoderBin, "plugins", "uninstall", pluginName, "--scope", "user", "--json").CombinedOutput()
		if err != nil {
			t.Logf("qoder plugins uninstall %s cleanup: %v\n%s", pluginName, err, out)
		}
	})

	listOut, err := exec.Command(qoderBin, "plugins", "list", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("qoder plugins list: %v\n%s", err, listOut)
	}
	if !strings.Contains(string(listOut), pluginName) {
		t.Fatalf("installed plugin %s missing from list:\n%s", pluginName, listOut)
	}
}

func qoderInstalledPluginName(installOut []byte, fallback string) string {
	var doc map[string]any
	if err := json.Unmarshal(installOut, &doc); err != nil {
		return fallback
	}
	for _, key := range []string{"name", "id", "plugin"} {
		if got := strings.TrimSpace(strings.Join(interfaceStrings(doc[key]), ",")); got != "" {
			return got
		}
	}
	if nested, ok := doc["plugin"].(map[string]any); ok {
		if got := strings.TrimSpace(strings.Join(interfaceStrings(nested["name"]), ",")); got != "" {
			return got
		}
	}
	return fallback
}

func interfaceStrings(v any) []string {
	switch value := v.(type) {
	case string:
		return []string{value}
	case []any:
		var out []string
		for _, item := range value {
			out = append(out, interfaceStrings(item)...)
		}
		return out
	default:
		return nil
	}
}

func qoderBinaryOrSkip(t *testing.T) string {
	t.Helper()
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_SKIP_QODER_CLI")) == "1" {
		t.Skip("PLUGIN_KIT_AI_SKIP_QODER_CLI=1")
	}
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_RUN_QODER_CLI")) != "1" {
		t.Skip("set PLUGIN_KIT_AI_RUN_QODER_CLI=1 to run real Qoder CLI e2e")
	}
	qoderBin := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_E2E_QODER"))
	if qoderBin == "" {
		var err error
		qoderBin, err = exec.LookPath("qoder")
		if err != nil {
			t.Skip("set PLUGIN_KIT_AI_E2E_QODER or install qoder in PATH to run Qoder CLI e2e")
		}
	}
	if out, err := exec.Command(qoderBin, "--version").CombinedOutput(); err != nil {
		t.Skipf("Qoder CLI is not runnable in this environment: %v\n%s", err, out)
	}
	return qoderBin
}
