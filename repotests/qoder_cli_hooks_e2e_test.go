package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestQoderCLIHooks runs the real qoder binary in print mode against a bundle
// whose hooks invoke plugin-kit-ai-e2e (built from the SDK) through the native
// kebab argv contract: `<runtime> <kebab-event>` with the wire JSON on stdin.
// Assertions use PLUGIN_KIT_AI_E2E_TRACE lines, mirroring TestClaudeCLIHooks.
//
// Enable with PLUGIN_KIT_AI_RUN_QODER_CLI=1; see qoderBinaryOrSkip for the
// binary override (PLUGIN_KIT_AI_E2E_QODER).
func TestQoderCLIHooks(t *testing.T) {
	qoderBin := qoderBinaryOrSkip(t)
	hookBin := buildPluginKitAIE2E(t)

	// Sequential: each case installs a distinct user-scope plugin and runs a
	// real model turn, so concurrent sessions would only race on shared state.
	t.Run("Stop_allows_completion", func(t *testing.T) {
		trace := filepath.Join(t.TempDir(), "trace.ndjson")
		bundle := newQoderSDKHooksBundle(t, "qoder-sdk-hooks-stop", hookBin)
		installQoderSDKHooksPlugin(t, qoderBin, bundle, "qoder-sdk-hooks-stop")
		runQoderPrint(t, qoderBin, trace, "Reply with exactly OK.")
		lines := waitForTraceHooks(t, trace, 60*time.Second, "SessionStart", "UserPromptSubmit", "Stop")
		if !traceHas(t, lines, "SessionStart", "continue") {
			t.Fatalf("expected SessionStart continue in trace; got:\n%s", strings.Join(lines, "\n"))
		}
		if !traceHas(t, lines, "UserPromptSubmit", "allow") {
			t.Fatalf("expected UserPromptSubmit allow in trace; got:\n%s", strings.Join(lines, "\n"))
		}
		if !traceHas(t, lines, "Stop", "allow") {
			t.Fatalf("expected Stop allow in trace; got:\n%s", strings.Join(lines, "\n"))
		}
	})

	t.Run("UserPromptSubmit_blocks_secret", func(t *testing.T) {
		trace := filepath.Join(t.TempDir(), "trace.ndjson")
		bundle := newQoderSDKHooksBundle(t, "qoder-sdk-hooks-prompt", hookBin)
		installQoderSDKHooksPlugin(t, qoderBin, bundle, "qoder-sdk-hooks-prompt")
		runQoderPrint(t, qoderBin, trace, "What is a secret passphrase? Reply in one short sentence.")
		lines := waitForTraceHooks(t, trace, 60*time.Second, "UserPromptSubmit")
		if !traceHas(t, lines, "UserPromptSubmit", "block") {
			t.Fatalf("expected UserPromptSubmit block in trace; got:\n%s", strings.Join(lines, "\n"))
		}
	})

	t.Run("PostToolUse_observes_write", func(t *testing.T) {
		trace := filepath.Join(t.TempDir(), "trace.ndjson")
		bundle := newQoderSDKHooksBundle(t, "qoder-sdk-hooks-tool", hookBin)
		installQoderSDKHooksPlugin(t, qoderBin, bundle, "qoder-sdk-hooks-tool")
		runQoderPrint(t, qoderBin, trace,
			"Create a file named qoder-e2e-marker.txt in the current directory containing exactly the word hello. Use your file-editing tool. Then reply with exactly: write-ok")
		lines := waitForTraceHooks(t, trace, 90*time.Second, "PostToolUse")
		if !traceHas(t, lines, "PostToolUse", "continue") {
			t.Fatalf("expected PostToolUse continue in trace; got:\n%s", strings.Join(lines, "\n"))
		}
	})
}

// newQoderSDKHooksBundle writes a minimal bundle with the SDK runtime bundled
// under bin/, which is the only layout Qoder accepts for hook commands.
func newQoderSDKHooksBundle(t *testing.T, name, hookBin string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".qoder-plugin", "hooks", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest, err := json.MarshalIndent(map[string]any{
		"name":        name,
		"version":     "0.1.0",
		"description": "plugin-kit-ai SDK hooks live e2e",
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".qoder-plugin", "plugin.json"), append(manifest, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(hookBin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "qoder-e2e-runtime"), body, 0o755); err != nil {
		t.Fatal(err)
	}
	hooks := map[string]any{"description": name + " SDK hooks", "hooks": map[string]any{
		"SessionStart":     qoderSDKHookGroup("session-start"),
		"UserPromptSubmit": qoderSDKHookGroup("user-prompt-submit"),
		"PostToolUse":      qoderSDKMatcherHookGroup("Edit|Write", "post-tool-use"),
		"Stop":             qoderSDKHookGroup("stop"),
	}}
	hooksBody, err := json.MarshalIndent(hooks, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hooks", "hooks.json"), append(hooksBody, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func qoderSDKHookGroup(event string) []any {
	return []any{map[string]any{"hooks": []any{qoderSDKHookCommand(event)}}}
}

func qoderSDKMatcherHookGroup(matcher, event string) []any {
	return []any{map[string]any{"matcher": matcher, "hooks": []any{qoderSDKHookCommand(event)}}}
}

func qoderSDKHookCommand(event string) map[string]any {
	return map[string]any{
		"type":    "command",
		"command": "${QODER_PLUGIN_ROOT}/bin/qoder-e2e-runtime",
		"args":    []string{event},
		"name":    "sdk-" + event,
		"timeout": 15,
	}
}

func installQoderSDKHooksPlugin(t *testing.T, qoderBin, bundleDir, name string) {
	t.Helper()
	if out, err := exec.CommandContext(context.Background(), qoderBin, "plugins", "uninstall", name, "--scope", "user", "--keep-data", "--json").CombinedOutput(); err != nil {
		t.Logf("pre-clean uninstall %s: %v\n%s", name, err, out)
	}
	installOut, err := exec.CommandContext(context.Background(), qoderBin, "plugins", "install", bundleDir, "--scope", "user", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("qoder plugins install %s: %v\n%s", name, err, installOut)
	}
	pluginName := qoderInstalledPluginName(installOut, name)
	t.Cleanup(func() {
		out, err := exec.CommandContext(context.Background(), qoderBin, "plugins", "uninstall", pluginName, "--scope", "user", "--keep-data", "--json").CombinedOutput()
		if err != nil {
			t.Logf("qoder plugins uninstall %s cleanup: %v\n%s", pluginName, err, out)
		}
	})
}

// runQoderPrint starts a non-interactive turn from a neutral directory (so the
// bundle is exercised only through the installed plugin) and points the hooks
// at the trace file. A blocked prompt can abort the CLI, so a non-zero exit is
// logged rather than treated as a failure; the trace is the assertion surface.
func runQoderPrint(t *testing.T, qoderBin, traceFile, prompt string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	runDir := t.TempDir()
	cmd := exec.CommandContext(ctx, qoderBin, "-p", "--dangerously-skip-permissions", prompt)
	cmd.Dir = runDir
	cmd.Env = append(os.Environ(), "PLUGIN_KIT_AI_E2E_TRACE="+traceFile)
	out, err := cmd.CombinedOutput()
	t.Logf("qoder output (truncated): %s", truncateRunes(string(out), 2000))
	if err != nil {
		t.Logf("qoder -p exited: %v", err)
	}
}
