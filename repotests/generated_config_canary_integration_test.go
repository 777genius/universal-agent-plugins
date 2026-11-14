package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestGeneratedConfigCanaries_ClaudeStableHookSubsetAndCommandShape(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := initGeneratedCanaryProject(t, pluginKitAIBin, "claude")

	runRenderCheckUnlessWindowsDrift(t, pluginKitAIBin, plugRoot)

	body, err := os.ReadFile(filepath.Join(plugRoot, "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var hooksFile struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(body, &hooksFile); err != nil {
		t.Fatalf("parse hooks/hooks.json: %v\n%s", err, body)
	}

	gotNames := sortedKeys(hooksFile.Hooks)
	wantNames := []string{"PreToolUse", "Stop", "UserPromptSubmit"}
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("hook names = %v, want %v", gotNames, wantNames)
	}
	for _, hookName := range wantNames {
		entries := hooksFile.Hooks[hookName]
		if len(entries) != 1 {
			t.Fatalf("%s entries = %d, want 1", hookName, len(entries))
		}
		if len(entries[0].Hooks) != 1 {
			t.Fatalf("%s hook commands = %d, want 1", hookName, len(entries[0].Hooks))
		}
		command := entries[0].Hooks[0]
		if command.Type != "command" {
			t.Fatalf("%s type = %q, want command", hookName, command.Type)
		}
		wantCommand := "./bin/genplug " + hookName
		if command.Command != wantCommand {
			t.Fatalf("%s command = %q, want %q", hookName, command.Command, wantCommand)
		}
	}

	report := inspectGeneratedProject(t, pluginKitAIBin, plugRoot, "claude")
	target := requireInspectTarget(t, report, "claude")
	mustHaveManagedArtifacts(t, target.ManagedArtifacts, ".claude-plugin/plugin.json", "hooks/hooks.json")
	mustExist(t, filepath.Join(plugRoot, ".claude-plugin", "plugin.json"))
	mustExist(t, filepath.Join(plugRoot, "hooks", "hooks.json"))
}

func TestGeneratedConfigCanaries_GeminiRuntimeContract(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := initGeneratedCanaryProject(t, pluginKitAIBin, "gemini")

	runRenderCheckUnlessWindowsDrift(t, pluginKitAIBin, plugRoot)

	body, err := os.ReadFile(filepath.Join(plugRoot, "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var hooksFile struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(body, &hooksFile); err != nil {
		t.Fatalf("parse hooks/hooks.json: %v\n%s", err, body)
	}

	gotNames := sortedKeys(hooksFile.Hooks)
	wantNames := []string{"AfterAgent", "AfterModel", "AfterTool", "BeforeAgent", "BeforeModel", "BeforeTool", "BeforeToolSelection", "SessionEnd", "SessionStart"}
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("hook names = %v, want %v", gotNames, wantNames)
	}
	wantCommands := map[string]string{
		"SessionStart":        "${extensionPath}${/}bin${/}genplug GeminiSessionStart",
		"SessionEnd":          "${extensionPath}${/}bin${/}genplug GeminiSessionEnd",
		"BeforeModel":         "${extensionPath}${/}bin${/}genplug GeminiBeforeModel",
		"AfterModel":          "${extensionPath}${/}bin${/}genplug GeminiAfterModel",
		"BeforeToolSelection": "${extensionPath}${/}bin${/}genplug GeminiBeforeToolSelection",
		"BeforeAgent":         "${extensionPath}${/}bin${/}genplug GeminiBeforeAgent",
		"AfterAgent":          "${extensionPath}${/}bin${/}genplug GeminiAfterAgent",
		"BeforeTool":          "${extensionPath}${/}bin${/}genplug GeminiBeforeTool",
		"AfterTool":           "${extensionPath}${/}bin${/}genplug GeminiAfterTool",
	}
	for _, hookName := range wantNames {
		entries := hooksFile.Hooks[hookName]
		if len(entries) != 1 {
			t.Fatalf("%s entries = %d, want 1", hookName, len(entries))
		}
		if entries[0].Matcher != "*" {
			t.Fatalf("%s matcher = %q, want *", hookName, entries[0].Matcher)
		}
		if len(entries[0].Hooks) != 1 {
			t.Fatalf("%s hook commands = %d, want 1", hookName, len(entries[0].Hooks))
		}
		command := entries[0].Hooks[0]
		if command.Type != "command" {
			t.Fatalf("%s type = %q, want command", hookName, command.Type)
		}
		if command.Command != wantCommands[hookName] {
			t.Fatalf("%s command = %q, want %q", hookName, command.Command, wantCommands[hookName])
		}
	}

	report := inspectGeneratedProject(t, pluginKitAIBin, plugRoot, "gemini")
	target := requireInspectTarget(t, report, "gemini")
	mustHaveManagedArtifacts(t, target.ManagedArtifacts, "gemini-extension.json", "hooks/hooks.json")
	mustExist(t, filepath.Join(plugRoot, "gemini-extension.json"))
	mustExist(t, filepath.Join(plugRoot, "hooks", "hooks.json"))

	textReport := inspectGeneratedProjectText(t, pluginKitAIBin, plugRoot, "gemini")
	for _, want := range []string{
		"launcher: runtime=go entrypoint=./bin/genplug",
		"next=go mod tidy; go test ./...; plugin-kit-ai generate --check .; plugin-kit-ai validate . --platform gemini --strict; gemini extensions link .",
		"runtime_gate=make test-gemini-runtime",
		"live_runtime_gate=make test-gemini-runtime-live",
	} {
		if !strings.Contains(textReport, want) {
			t.Fatalf("inspect text missing %q:\n%s", want, textReport)
		}
	}
	for _, want := range []string{"gemini-extension.json", "hooks/hooks.json"} {
		if !strings.Contains(textReport, want) {
			t.Fatalf("inspect text missing managed artifact %q:\n%s", want, textReport)
		}
	}

	validateCmd := exec.Command(pluginKitAIBin, "validate", plugRoot, "--platform", "gemini")
	validateOut, err := validateCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai validate: %v\n%s", err, validateOut)
	}
	validateText := string(validateOut)
	for _, want := range []string{
		`Warning: Gemini extension directory basename "project with spaces" does not match extension name "genplug"`,
		"Hint: rename the extension directory to match plugin.yaml name before running gemini extensions link .",
		"Validated " + plugRoot,
		"Hint: Gemini Go runtime is validate-clean; run make test-gemini-runtime before relinking the extension.",
		"Hint: relink the extension with gemini extensions link . before checking the runtime path in a real Gemini CLI session.",
		"Hint: use make test-gemini-runtime-live when you need real CLI evidence after the repo-local runtime gate is green.",
	} {
		if !strings.Contains(validateText, want) {
			t.Fatalf("validate output missing %q:\n%s", want, validateText)
		}
	}
}

func TestGeneratedConfigCanaries_CodexNotifyInvocationShape(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := initGeneratedCanaryProject(t, pluginKitAIBin, "codex-runtime")

	runRenderCheckUnlessWindowsDrift(t, pluginKitAIBin, plugRoot)

	body, err := os.ReadFile(filepath.Join(plugRoot, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := nonCommentLines(string(body))
	if len(lines) != 2 {
		t.Fatalf("config lines = %v, want exactly model + notify", lines)
	}
	if lines[0] != `model = "gpt-5.4-mini"` {
		t.Fatalf("first config line = %q, want gpt-5.4-mini", lines[0])
	}
	if lines[1] != `notify = ["./bin/genplug", "notify"]` {
		t.Fatalf("notify line = %q, want exact argv shape", lines[1])
	}
	packageBody, err := os.ReadFile(filepath.Join(plugRoot, authoredRel("targets", "codex-runtime", "package.yaml")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(packageBody), `model_hint: "gpt-5.4-mini"`) {
		t.Fatalf("targets/codex-runtime/package.yaml = %q, want gpt-5.4-mini model_hint", string(packageBody))
	}

	report := inspectGeneratedProject(t, pluginKitAIBin, plugRoot, "codex-runtime")
	target := requireInspectTarget(t, report, "codex-runtime")
	mustHaveManagedArtifacts(t, target.ManagedArtifacts, ".codex/config.toml")
	if got := target.NativeDocPaths["package_metadata"]; got != authoredSlash("targets", "codex-runtime", "package.yaml") {
		t.Fatalf("native_doc_paths[package_metadata] = %q", got)
	}
	if got := target.NativeSurfaceTiers["config_extra"]; got != "stable" {
		t.Fatalf("native_surface_tiers[config_extra] = %q", got)
	}
	mustExist(t, filepath.Join(plugRoot, ".codex", "config.toml"))
}

func TestGeneratedConfigCanaries_RenderCheckDetectsRuntimeArtifactDrift(t *testing.T) {
	cases := []struct {
		platform  string
		driftFile string
		driftBody string
	}{
		{
			platform:  "claude",
			driftFile: filepath.Join("hooks", "hooks.json"),
			driftBody: `{"hooks":{"Stop":[]}}`,
		},
		{
			platform:  "codex-runtime",
			driftFile: filepath.Join(".codex", "config.toml"),
			driftBody: "notify = [\"./bin/genplug\"]\n",
		},
		{
			platform:  "gemini",
			driftFile: filepath.Join("hooks", "hooks.json"),
			driftBody: `{"hooks":{"SessionStart":[]}}`,
		},
	}

	pluginKitAIBin := buildPluginKitAI(t)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.platform, func(t *testing.T) {
			plugRoot := initGeneratedCanaryProject(t, pluginKitAIBin, tc.platform)
			writeRuntimeFile(t, plugRoot, tc.driftFile, tc.driftBody)

			cmd := exec.Command(pluginKitAIBin, "generate", plugRoot, "--check")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("generate --check unexpectedly succeeded:\n%s", out)
			}
			if !strings.Contains(string(out), filepath.ToSlash(tc.driftFile)) {
				t.Fatalf("generate --check output = %q, want drift path %q", string(out), filepath.ToSlash(tc.driftFile))
			}
		})
	}
}

func TestGeneratedConfigCanaries_ClaudeAuthoredHookEntrypointDriftIsCaughtByValidate(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := initGeneratedCanaryProject(t, pluginKitAIBin, "claude")

	writeRuntimeFile(t, plugRoot, authoredRel("targets", "claude", "hooks", "hooks.json"), `{
  "hooks": {
    "Stop": [{"hooks": [{"type": "command", "command": "./bin/old-genplug Stop"}]}],
    "PreToolUse": [{"hooks": [{"type": "command", "command": "./bin/old-genplug PreToolUse"}]}],
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "./bin/old-genplug UserPromptSubmit"}]}]
  }
}
`)

	runPluginKitAICommand(t, pluginKitAIBin, "generate", plugRoot)
	runPluginKitAICommand(t, pluginKitAIBin, "generate", plugRoot, "--check")

	cmd := exec.Command(pluginKitAIBin, "validate", plugRoot, "--platform", "claude", "--strict")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("validate unexpectedly succeeded:\n%s", out)
	}
	if !strings.Contains(string(out), "entrypoint mismatch") {
		t.Fatalf("validate output = %q, want entrypoint mismatch", string(out))
	}
	if !strings.Contains(string(out), "./bin/old-genplug") || !strings.Contains(string(out), "./bin/genplug") {
		t.Fatalf("validate output = %q, want old and expected entrypoint details", string(out))
	}
}

func TestGeneratedConfigCanaries_CodexValidateJSONContract(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := initGeneratedCanaryProject(t, pluginKitAIBin, "codex-runtime")

	report := validateGeneratedProjectJSON(t, pluginKitAIBin, plugRoot, "codex-runtime", true)
	if report.Format != "plugin-kit-ai/validate-report" || report.SchemaVersion != 1 {
		t.Fatalf("contract = %#v", report)
	}
	if report.RequestedPlatform != "codex-runtime" || report.Outcome != "passed" {
		t.Fatalf("platform/outcome = %#v", report)
	}
	if !report.OK || !report.StrictMode || report.StrictFailed {
		t.Fatalf("summary = %#v", report)
	}
	if report.WarningCount != 0 || report.FailureCount != 0 {
		t.Fatalf("counts = %#v", report)
	}
}

type inspectReport struct {
	Targets []inspectTarget `json:"targets"`
}

type validateJSONCanaryReport struct {
	Format            string `json:"format"`
	SchemaVersion     int    `json:"schema_version"`
	RequestedPlatform string `json:"requested_platform"`
	Outcome           string `json:"outcome"`
	OK                bool   `json:"ok"`
	StrictMode        bool   `json:"strict_mode"`
	StrictFailed      bool   `json:"strict_failed"`
	WarningCount      int    `json:"warning_count"`
	FailureCount      int    `json:"failure_count"`
}

type inspectTarget struct {
	Target             string            `json:"target"`
	ManagedArtifacts   []string          `json:"managed_artifacts"`
	NativeDocPaths     map[string]string `json:"native_doc_paths"`
	NativeSurfaceTiers map[string]string `json:"native_surface_tiers"`
}

func initGeneratedCanaryProject(t *testing.T, pluginKitAIBin, platform string) string {
	t.Helper()
	plugRoot := runtimeProjectRoot(t)
	args := []string{"init", "genplug", "--platform", platform, "-o", plugRoot}
	if platform != "codex-package" {
		args = append(args, "--runtime", "go")
	}
	runPluginKitAICommand(t, pluginKitAIBin, args...)
	if platform != "codex-package" {
		bootstrapGeneratedGoPlugin(t, plugRoot)
	}
	return plugRoot
}

func inspectGeneratedProject(t *testing.T, pluginKitAIBin, root, target string) inspectReport {
	t.Helper()
	cmd := exec.Command(pluginKitAIBin, "inspect", root, "--target", target, "--format", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai inspect: %v\n%s", err, out)
	}
	var report inspectReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("parse inspect json: %v\n%s", err, out)
	}
	return report
}

func inspectGeneratedProjectText(t *testing.T, pluginKitAIBin, root, target string) string {
	t.Helper()
	cmd := exec.Command(pluginKitAIBin, "inspect", root, "--target", target, "--format", "text")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai inspect text: %v\n%s", err, out)
	}
	return string(out)
}

func validateGeneratedProjectJSON(t *testing.T, pluginKitAIBin, root, target string, strict bool) validateJSONCanaryReport {
	t.Helper()
	args := []string{"validate", root, "--platform", target, "--format", "json"}
	if strict {
		args = append(args, "--strict")
	}
	cmd := exec.Command(pluginKitAIBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai validate json: %v\n%s", err, out)
	}
	var report validateJSONCanaryReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("parse validate json: %v\n%s", err, out)
	}
	return report
}

func runRenderCheckUnlessWindowsDrift(t *testing.T, pluginKitAIBin, root string) {
	t.Helper()
	cmd := exec.Command(pluginKitAIBin, "generate", root, "--check")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return
	}
	if runtime.GOOS == "windows" && strings.Contains(string(out), "generated artifacts drifted:") {
		t.Logf("accepting known Windows generate --check drift instability:\n%s", out)
		return
	}
	t.Fatalf("plugin-kit-ai generate --check: %v\n%s", err, out)
}

func requireInspectTarget(t *testing.T, report inspectReport, wantTarget string) inspectTarget {
	t.Helper()
	for _, target := range report.Targets {
		if target.Target == wantTarget {
			return target
		}
	}
	t.Fatalf("missing inspect target %q in %+v", wantTarget, report.Targets)
	return inspectTarget{}
}

func mustHaveManagedArtifacts(t *testing.T, got []string, want ...string) {
	t.Helper()
	for _, item := range want {
		if !slices.Contains(got, item) {
			t.Fatalf("managed artifacts = %v, want %q", got, item)
		}
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func runPluginKitAICommand(t *testing.T, pluginKitAIBin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(pluginKitAIBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", cmd.String(), err, out)
	}
	return string(out)
}

func nonCommentLines(body string) []string {
	lines := strings.Split(body, "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func sortedKeys[M ~map[string]V, V any](m M) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}
