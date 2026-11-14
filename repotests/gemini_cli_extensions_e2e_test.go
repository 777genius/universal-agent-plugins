package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const geminiRuntimeLiveEnvVar = "PLUGIN_KIT_AI_RUN_GEMINI_RUNTIME_LIVE"
const geminiExtensionLiveEnvVar = "PLUGIN_KIT_AI_RUN_GEMINI_CLI"
const geminiRuntimeLiveToolPrompt = "Use the read_file tool to read README.md from the current workspace, then reply with exactly OK."
const geminiRuntimeLiveRewriteToolPrompt = "Use the read_file tool to read THIS_FILE_SHOULD_NOT_EXIST_FOR_GEMINI_E2E.md from the current workspace, then reply with exactly OK."

type geminiCLIEnvelope struct {
	SessionID string `json:"session_id"`
	Response  string `json:"response"`
	Stats     struct {
		Tools struct {
			TotalCalls   int `json:"totalCalls"`
			TotalSuccess int `json:"totalSuccess"`
			TotalFail    int `json:"totalFail"`
			ByName       map[string]struct {
				Count      int `json:"count"`
				TotalCalls int `json:"totalCalls"`
				Success    int `json:"success"`
				Fail       int `json:"fail"`
			} `json:"byName"`
		} `json:"tools"`
	} `json:"stats"`
}

type geminiRuntimeLiveFixture struct {
	GeminiBin    string
	ExtensionDir string
	HomeDir      string
	TracePath    string
}

func TestGeminiCLIExtensionLink(t *testing.T) {
	if strings.TrimSpace(os.Getenv(geminiExtensionLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real Gemini extension lifecycle smoke", geminiExtensionLiveEnvVar)
	}
	geminiBin := geminiBinaryOrSkip(t)
	root := RepoRoot(t)
	pluginKitAIBin := buildPluginKitAI(t)

	workRoot := t.TempDir()
	extensionDir := filepath.Join(workRoot, "gemini-extension-package")
	copyTree(t, filepath.Join(root, "examples", "plugins", "gemini-extension-package"), extensionDir)

	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", extensionDir, "--check"))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", extensionDir, "--platform", "gemini", "--strict"))

	homeDir := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	seedGeminiHome(t, homeDir, extensionDir)
	validateOutput := runGeminiCommand(t, geminiBin, homeDir, extensionDir, "extensions", "validate", extensionDir)
	if !strings.Contains(validateOutput, "successfully validated") {
		t.Fatalf("gemini validate output missing success marker:\n%s", validateOutput)
	}
	output := runGeminiLink(t, geminiBin, homeDir, extensionDir)
	if !strings.Contains(output, `Extension "gemini-extension-package" linked successfully and enabled.`) {
		t.Fatalf("gemini link output missing success marker:\n%s", output)
	}
	installMetadataPath := filepath.Join(homeDir, ".gemini", "extensions", "gemini-extension-package", ".gemini-extension-install.json")
	installMetadataBody, err := os.ReadFile(installMetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installMetadataBody), extensionDir) || !strings.Contains(string(installMetadataBody), `"type": "link"`) {
		t.Fatalf("unexpected install metadata:\n%s", installMetadataBody)
	}
	envPath := filepath.Join(homeDir, ".gemini", "extensions", "gemini-extension-package", ".env")
	assertFileContains(t, envPath, "RELEASE_PROFILE=stable")
	listOutput := runGeminiCommand(t, geminiBin, homeDir, extensionDir, "extensions", "list")
	if !strings.Contains(listOutput, "gemini-extension-package") {
		t.Fatalf("gemini list output missing linked extension:\n%s", listOutput)
	}
	for _, want := range []string{
		"MCP servers:",
		"release-checks",
		"Context files:",
		"GEMINI.md",
		"Settings:",
		"release-profile: stable",
		"Excluded tools:",
		"run_shell_command(rm -rf)",
	} {
		if !strings.Contains(listOutput, want) {
			t.Fatalf("gemini list output missing %q:\n%s", want, listOutput)
		}
	}

	configOutput := runGeminiConfig(t, geminiBin, homeDir, extensionDir, "gemini-extension-package", "release-profile", "canary\n")
	if !strings.Contains(configOutput, `Setting "release-profile" updated.`) {
		t.Fatalf("gemini config output missing success marker:\n%s", configOutput)
	}
	assertFileContains(t, envPath, "RELEASE_PROFILE=canary")

	disableOutput := runGeminiCommand(t, geminiBin, homeDir, extensionDir, "extensions", "disable", "gemini-extension-package", "--scope", "user")
	if !strings.Contains(disableOutput, `Extension "gemini-extension-package" successfully disabled for scope "user".`) {
		t.Fatalf("gemini disable output missing success marker:\n%s", disableOutput)
	}
	assertEnablementRule(t, filepath.Join(homeDir, ".gemini", "extensions", "extension-enablement.json"), "gemini-extension-package", "!"+homeDir+"/*")

	enableOutput := runGeminiCommand(t, geminiBin, homeDir, extensionDir, "extensions", "enable", "gemini-extension-package", "--scope", "user")
	if !strings.Contains(enableOutput, `Extension "gemini-extension-package" successfully enabled for scope "user".`) {
		t.Fatalf("gemini enable output missing success marker:\n%s", enableOutput)
	}
	assertEnablementRule(t, filepath.Join(homeDir, ".gemini", "extensions", "extension-enablement.json"), "gemini-extension-package", homeDir+"/*")

	registryBody, err := os.ReadFile(filepath.Join(homeDir, ".gemini", "projects.json"))
	if err != nil {
		t.Fatal(err)
	}
	registry := string(registryBody)
	if !strings.Contains(registry, extensionDir) && !strings.Contains(registry, filepath.Base(extensionDir)) {
		t.Fatalf("gemini project registry was not updated:\n%s", registryBody)
	}
}

func setupGeminiRuntimeLiveFixture(t *testing.T) geminiRuntimeLiveFixture {
	t.Helper()
	fixture := geminiRuntimeLiveFixture{
		GeminiBin: geminiBinaryOrSkip(t),
		TracePath: filepath.Join(t.TempDir(), "trace.ndjson"),
	}
	root := RepoRoot(t)
	pluginKitAIBin := buildPluginKitAI(t)
	hookBin := buildPluginKitAIE2E(t)
	env := newGoModuleEnv(t)

	workRoot := t.TempDir()
	fixture.ExtensionDir = filepath.Join(workRoot, "gemini-runtime-live")
	run := exec.Command(pluginKitAIBin, "init", "gemini-runtime-live", "--platform", "gemini", "--runtime", "go", "-o", fixture.ExtensionDir)
	run.Dir = root
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
	}

	wireGeneratedGoModuleToLocalSDK(t, fixture.ExtensionDir, env)
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = fixture.ExtensionDir
	tidy.Env = env
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}

	binName := "gemini-runtime-live"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	build := exec.Command("go", "build", "-o", filepath.Join("bin", binName), "./cmd/gemini-runtime-live")
	build.Dir = fixture.ExtensionDir
	build.Env = env
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build generated entrypoint: %v\n%s", err, out)
	}

	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", fixture.ExtensionDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", fixture.ExtensionDir, "--check"))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", fixture.ExtensionDir, "--platform", "gemini", "--strict"))

	absHook, err := filepath.Abs(hookBin)
	if err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(fixture.ExtensionDir, "hooks", "hooks.json")
	hooksBody, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	updatedHooks := strings.ReplaceAll(string(hooksBody), "${extensionPath}${/}bin${/}gemini-runtime-live", absHook)
	if err := os.WriteFile(hooksPath, []byte(updatedHooks), 0o644); err != nil {
		t.Fatal(err)
	}

	fixture.HomeDir = filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(fixture.HomeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	seedGeminiHome(t, fixture.HomeDir, fixture.ExtensionDir)
	return fixture
}

func runGeminiRuntimeLivePrompt(t *testing.T, fixture geminiRuntimeLiveFixture, prompt string, extraEnv ...string) ([]byte, geminiCLIEnvelope) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, fixture.GeminiBin, "-p", prompt, "--output-format", "json")
	cmd.Dir = fixture.ExtensionDir
	cmd.Env = append(geminiCLIEnv(fixture.HomeDir), append([]string{"PLUGIN_KIT_AI_E2E_TRACE=" + fixture.TracePath}, extraEnv...)...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("gemini runtime smoke timed out; %s rerun make test-gemini-runtime first, then make test-gemini-runtime-live.\ntrace=%s\noutput:\n%s", geminiAuthRecoveryHint(string(out)), fixture.TracePath, truncateRunes(string(out), 4000))
	}
	if err != nil {
		if geminiEnvironmentIssue(string(out)) {
			t.Skipf("gemini environment is not ready for isolated runtime live e2e; %s\n%s", geminiAuthRecoveryHint(string(out)), truncateRunes(string(out), 4000))
		}
		t.Fatalf("gemini runtime smoke: %v\ntrace=%s\nhint=confirm make test-gemini-runtime passes, then confirm gemini extensions link . succeeded, inspect hooks/hooks.json command wiring, and rerun the live smoke.\noutput:\n%s", err, fixture.TracePath, truncateRunes(string(out), 4000))
	}
	return out, parseGeminiJSONEnvelope(t, out)
}

func TestGeminiCLIRuntimeHooks(t *testing.T) {
	if strings.TrimSpace(os.Getenv(geminiRuntimeLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real Gemini runtime hook smoke", geminiRuntimeLiveEnvVar)
	}
	fixture := setupGeminiRuntimeLiveFixture(t)
	linkOutput := runGeminiLink(t, fixture.GeminiBin, fixture.HomeDir, fixture.ExtensionDir)
	if !strings.Contains(linkOutput, `linked successfully and enabled`) {
		t.Fatalf("gemini runtime live link did not report success:\n%s", linkOutput)
	}
	out, envelope := runGeminiRuntimeLivePrompt(t, fixture, geminiRuntimeLiveToolPrompt)
	if strings.TrimSpace(envelope.SessionID) == "" {
		t.Fatalf("expected Gemini live output to include session_id\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if strings.TrimSpace(envelope.Response) != "OK" {
		t.Fatalf("expected Gemini live output response=OK, got %q\noutput:\n%s", envelope.Response, truncateRunes(string(out), 4000))
	}
	if envelope.Stats.Tools.TotalCalls <= 0 {
		t.Fatalf("expected Gemini live output to report at least one tool call\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if envelope.Stats.Tools.TotalSuccess <= 0 || envelope.Stats.Tools.TotalFail != 0 {
		t.Fatalf("expected Gemini happy-path output to report successful tool execution without failures\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if stats, ok := envelope.Stats.Tools.ByName["read_file"]; !ok || maxInt(stats.Count, stats.TotalCalls) <= 0 || stats.Success <= 0 || stats.Fail != 0 {
		t.Fatalf("expected Gemini live output to report successful read_file tool usage without failures\noutput:\n%s", truncateRunes(string(out), 4000))
	}

	lines := waitForTraceHooks(t, fixture.TracePath, 5*time.Second, "SessionStart", "BeforeModel", "AfterModel", "BeforeToolSelection", "BeforeAgent", "AfterAgent", "BeforeTool", "AfterTool", "SessionEnd")
	sessionStartIndex, sessionStart, ok := traceIndex(t, lines, "SessionStart")
	if !ok {
		t.Fatalf("expected SessionStart trace; hint=confirm make test-gemini-runtime passes, then confirm the linked extension still points at the generated runtime repo, inspect hooks/hooks.json, and rerun gemini -p.\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(sessionStart.Outcome) != "continue" || strings.TrimSpace(sessionStart.Source) != "startup" {
		t.Fatalf("expected SessionStart continue trace with startup source; got outcome=%q source=%q\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", sessionStart.Outcome, sessionStart.Source, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	beforeModelIndex, beforeModel, ok := traceIndex(t, lines, "BeforeModel")
	if !ok {
		t.Fatalf("expected BeforeModel trace; hint=confirm make test-gemini-runtime passes, then confirm the prompt still reaches Gemini model planning and rerun gemini -p.\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	afterModelIndex, afterModel, ok := traceIndex(t, lines, "AfterModel")
	if !ok {
		t.Fatalf("expected AfterModel trace; hint=confirm make test-gemini-runtime passes, then confirm the prompt still reaches Gemini response generation and rerun gemini -p.\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	beforeToolSelectionIndex, beforeToolSelection, ok := traceIndex(t, lines, "BeforeToolSelection")
	if !ok {
		t.Fatalf("expected BeforeToolSelection trace; hint=confirm make test-gemini-runtime passes, then confirm the prompt still triggers Gemini tool routing and rerun gemini -p with an explicit tool-use prompt.\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(beforeModel.Outcome) != "continue" || !beforeModel.HasRequest || beforeModel.RequestSize == 0 {
		t.Fatalf("expected BeforeModel continue with request payload; got outcome=%q has_request=%v request_size=%d\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeModel.Outcome, beforeModel.HasRequest, beforeModel.RequestSize, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(afterModel.Outcome) != "continue" || !afterModel.HasRequest || !afterModel.HasResponse || afterModel.ResponseSize == 0 {
		t.Fatalf("expected AfterModel continue with request+response payloads; got outcome=%q has_request=%v has_response=%v response_size=%d\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", afterModel.Outcome, afterModel.HasRequest, afterModel.HasResponse, afterModel.ResponseSize, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(beforeToolSelection.Outcome) != "continue" {
		t.Fatalf("expected BeforeToolSelection continue outcome; got=%q\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeToolSelection.Outcome, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if beforeModelIndex >= afterModelIndex {
		t.Fatalf("expected BeforeModel to occur before AfterModel; before=%d after=%d\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeModelIndex, afterModelIndex, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	beforeAgent, ok := traceFind(t, lines, "BeforeAgent")
	if !ok {
		t.Fatalf("expected BeforeAgent trace; hint=confirm make test-gemini-runtime passes, then confirm the prompt still reaches Gemini planning and rerun gemini -p.\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	afterAgent, ok := traceFind(t, lines, "AfterAgent")
	if !ok {
		t.Fatalf("expected AfterAgent trace; hint=confirm make test-gemini-runtime passes, then confirm the turn reaches Gemini final response and rerun gemini -p.\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(beforeAgent.Outcome) != "continue" {
		t.Fatalf("expected BeforeAgent continue outcome; got=%q\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeAgent.Outcome, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(afterAgent.Outcome) != "continue" {
		t.Fatalf("expected AfterAgent continue outcome; got=%q\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", afterAgent.Outcome, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	beforeTool, ok := traceFind(t, lines, "BeforeTool")
	if !ok || strings.TrimSpace(beforeTool.Tool) == "" {
		t.Fatalf("expected BeforeTool trace with tool_name; hint=confirm make test-gemini-runtime passes, then confirm the prompt still triggers a Gemini tool path and rerun gemini -p with an explicit tool-use prompt.\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if !beforeTool.HasInput || beforeTool.InputSize == 0 {
		t.Fatalf("expected BeforeTool trace with tool_input payload; has_input=%v input_size=%d\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeTool.HasInput, beforeTool.InputSize, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	afterTool, ok := traceFind(t, lines, "AfterTool")
	if !ok || strings.TrimSpace(afterTool.Tool) == "" {
		t.Fatalf("expected AfterTool trace with tool_name; hint=confirm make test-gemini-runtime passes, then confirm the prompt still triggers a Gemini tool path and rerun gemini -p with an explicit tool-use prompt.\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if !afterTool.HasInput || afterTool.InputSize == 0 || !afterTool.HasResponse || afterTool.ResponseSize == 0 {
		t.Fatalf("expected AfterTool trace with tool_input+tool_response payloads; has_input=%v input_size=%d has_response=%v response_size=%d\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", afterTool.HasInput, afterTool.InputSize, afterTool.HasResponse, afterTool.ResponseSize, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	beforeToolIndex, _, ok := traceIndex(t, lines, "BeforeTool")
	if !ok {
		t.Fatalf("expected BeforeTool trace index; trace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if beforeToolSelectionIndex >= beforeToolIndex {
		t.Fatalf("expected BeforeToolSelection to occur before BeforeTool; selection=%d before_tool=%d\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeToolSelectionIndex, beforeToolIndex, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if beforeTool.Tool != afterTool.Tool {
		t.Fatalf("expected BeforeTool and AfterTool to reference the same Gemini tool; before=%q after=%q\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeTool.Tool, afterTool.Tool, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	sessionEndIndex, sessionEnd, ok := traceIndex(t, lines, "SessionEnd")
	if !ok {
		t.Fatalf("expected SessionEnd trace; hint=confirm make test-gemini-runtime passes, then confirm the Gemini CLI session exits cleanly and rerun gemini -p.\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(sessionEnd.Outcome) != "continue" || !isGeminiSessionEndReason(sessionEnd.Reason) {
		t.Fatalf("expected SessionEnd continue trace with documented reason; got outcome=%q reason=%q\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", sessionEnd.Outcome, sessionEnd.Reason, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if sessionStartIndex >= sessionEndIndex {
		t.Fatalf("expected SessionStart to occur before SessionEnd; start=%d end=%d\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", sessionStartIndex, sessionEndIndex, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
}

func TestGeminiCLIRuntimeBeforeToolDeny(t *testing.T) {
	if strings.TrimSpace(os.Getenv(geminiRuntimeLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real Gemini runtime hook smoke", geminiRuntimeLiveEnvVar)
	}
	fixture := setupGeminiRuntimeLiveFixture(t)
	linkOutput := runGeminiLink(t, fixture.GeminiBin, fixture.HomeDir, fixture.ExtensionDir)
	if !strings.Contains(linkOutput, `linked successfully and enabled`) {
		t.Fatalf("gemini runtime live link did not report success:\n%s", linkOutput)
	}

	out, envelope := runGeminiRuntimeLivePrompt(t, fixture, geminiRuntimeLiveToolPrompt, "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_TOOL=deny:blocked by e2e")
	if strings.TrimSpace(envelope.SessionID) == "" {
		t.Fatalf("expected Gemini deny-path output to include session_id\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if !strings.Contains(envelope.Response, "OK") {
		t.Fatalf("expected Gemini deny-path response to preserve the final OK reply, got %q\noutput:\n%s", envelope.Response, truncateRunes(string(out), 4000))
	}
	if !strings.Contains(string(out), "blocked by e2e") {
		t.Fatalf("expected Gemini deny-path output to surface the denied-tool reason\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if envelope.Stats.Tools.TotalCalls <= 0 || envelope.Stats.Tools.TotalFail <= 0 {
		t.Fatalf("expected Gemini deny-path output to report a failed tool call\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	stats, ok := envelope.Stats.Tools.ByName["read_file"]
	if !ok {
		t.Fatalf("expected Gemini deny-path output to report read_file tool usage\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if maxInt(stats.Count, stats.TotalCalls) <= 0 || stats.Fail <= 0 || stats.Success != 0 {
		t.Fatalf("expected Gemini deny-path stats to show read_file failed without success; got count=%d totalCalls=%d fail=%d success=%d\noutput:\n%s", stats.Count, stats.TotalCalls, stats.Fail, stats.Success, truncateRunes(string(out), 4000))
	}

	lines := waitForTraceHooks(t, fixture.TracePath, 5*time.Second, "SessionStart", "BeforeModel", "BeforeToolSelection", "BeforeAgent", "BeforeTool", "AfterAgent", "SessionEnd")
	beforeTool, ok := traceFind(t, lines, "BeforeTool")
	if !ok {
		t.Fatalf("expected BeforeTool deny trace; trace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(beforeTool.Outcome) != "deny" || strings.TrimSpace(beforeTool.Tool) != "read_file" || !beforeTool.HasInput || beforeTool.InputSize == 0 {
		t.Fatalf("expected denied BeforeTool trace with read_file input; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeTool, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if _, ok := traceFind(t, lines, "AfterTool"); ok {
		t.Fatalf("expected denied Gemini tool path to skip AfterTool trace\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	afterAgent, ok := traceFind(t, lines, "AfterAgent")
	if !ok || strings.TrimSpace(afterAgent.Outcome) != "continue" {
		t.Fatalf("expected AfterAgent continue trace after denied tool path; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", afterAgent, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	sessionEnd, ok := traceFind(t, lines, "SessionEnd")
	if !ok || !isGeminiSessionEndReason(sessionEnd.Reason) {
		t.Fatalf("expected documented SessionEnd reason after deny-path live run; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", sessionEnd, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
}

func TestGeminiCLIRuntimeBeforeModelDeny(t *testing.T) {
	if strings.TrimSpace(os.Getenv(geminiRuntimeLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real Gemini runtime hook smoke", geminiRuntimeLiveEnvVar)
	}
	fixture := setupGeminiRuntimeLiveFixture(t)
	linkOutput := runGeminiLink(t, fixture.GeminiBin, fixture.HomeDir, fixture.ExtensionDir)
	if !strings.Contains(linkOutput, `linked successfully and enabled`) {
		t.Fatalf("gemini runtime live link did not report success:\n%s", linkOutput)
	}

	out, envelope := runGeminiRuntimeLivePrompt(t, fixture, "Reply with exactly OK.", "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_MODEL=deny:blocked model request")
	if strings.TrimSpace(envelope.SessionID) == "" {
		t.Fatalf("expected Gemini model-deny output to include session_id\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if strings.TrimSpace(envelope.Response) != "" {
		t.Fatalf("expected Gemini model-deny response to be empty, got %q\noutput:\n%s", envelope.Response, truncateRunes(string(out), 4000))
	}
	if envelope.Stats.Tools.TotalCalls != 0 || envelope.Stats.Tools.TotalSuccess != 0 || envelope.Stats.Tools.TotalFail != 0 {
		t.Fatalf("expected Gemini model-deny output to skip all tool activity\noutput:\n%s", truncateRunes(string(out), 4000))
	}

	lines := waitForTraceHooks(t, fixture.TracePath, 5*time.Second, "SessionStart", "BeforeAgent", "BeforeModel", "AfterAgent", "SessionEnd")
	beforeModel, ok := traceFind(t, lines, "BeforeModel")
	if !ok {
		t.Fatalf("expected BeforeModel deny trace; trace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(beforeModel.Outcome) != "deny" || !beforeModel.HasRequest || beforeModel.RequestSize == 0 {
		t.Fatalf("expected denied BeforeModel trace with request payload; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeModel, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if _, ok := traceFind(t, lines, "AfterModel"); ok {
		t.Fatalf("expected denied Gemini model path to skip AfterModel trace\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if _, ok := traceFind(t, lines, "BeforeToolSelection"); ok {
		t.Fatalf("expected denied Gemini model path to skip BeforeToolSelection trace\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if _, ok := traceFind(t, lines, "BeforeTool"); ok {
		t.Fatalf("expected denied Gemini model path to skip BeforeTool trace\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	afterAgent, ok := traceFind(t, lines, "AfterAgent")
	if !ok || strings.TrimSpace(afterAgent.Outcome) != "continue" {
		t.Fatalf("expected AfterAgent continue trace after denied model path; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", afterAgent, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	sessionEnd, ok := traceFind(t, lines, "SessionEnd")
	if !ok || !isGeminiSessionEndReason(sessionEnd.Reason) {
		t.Fatalf("expected documented SessionEnd reason after model-deny live run; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", sessionEnd, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
}

func TestGeminiCLIRuntimeDisableAllTools(t *testing.T) {
	if strings.TrimSpace(os.Getenv(geminiRuntimeLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real Gemini runtime hook smoke", geminiRuntimeLiveEnvVar)
	}
	fixture := setupGeminiRuntimeLiveFixture(t)
	linkOutput := runGeminiLink(t, fixture.GeminiBin, fixture.HomeDir, fixture.ExtensionDir)
	if !strings.Contains(linkOutput, `linked successfully and enabled`) {
		t.Fatalf("gemini runtime live link did not report success:\n%s", linkOutput)
	}

	out, envelope := runGeminiRuntimeLivePrompt(t, fixture, geminiRuntimeLiveToolPrompt, "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_TOOL_SELECTION=disable_all")
	if strings.TrimSpace(envelope.SessionID) == "" {
		t.Fatalf("expected Gemini disable-all output to include session_id\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if envelope.Stats.Tools.TotalCalls != 0 || envelope.Stats.Tools.TotalSuccess != 0 || envelope.Stats.Tools.TotalFail != 0 {
		t.Fatalf("expected Gemini disable-all output to report zero tool activity\noutput:\n%s", truncateRunes(string(out), 4000))
	}

	lines := waitForTraceHooks(t, fixture.TracePath, 5*time.Second, "SessionStart", "BeforeAgent", "BeforeModel", "BeforeToolSelection", "AfterModel", "AfterAgent", "SessionEnd")
	beforeToolSelection, ok := traceFind(t, lines, "BeforeToolSelection")
	if !ok {
		t.Fatalf("expected BeforeToolSelection disable_all trace; trace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(beforeToolSelection.Outcome) != "disable_all" || !beforeToolSelection.HasRequest || beforeToolSelection.RequestSize == 0 {
		t.Fatalf("expected disable_all BeforeToolSelection trace with request payload; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeToolSelection, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if _, ok := traceFind(t, lines, "BeforeTool"); ok {
		t.Fatalf("expected disable_all Gemini path to skip BeforeTool trace\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if _, ok := traceFind(t, lines, "AfterTool"); ok {
		t.Fatalf("expected disable_all Gemini path to skip AfterTool trace\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	afterModel, ok := traceFind(t, lines, "AfterModel")
	if !ok || strings.TrimSpace(afterModel.Outcome) != "continue" || !afterModel.HasResponse || afterModel.ResponseSize == 0 {
		t.Fatalf("expected AfterModel continue trace after disable_all path; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", afterModel, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	sessionEnd, ok := traceFind(t, lines, "SessionEnd")
	if !ok || !isGeminiSessionEndReason(sessionEnd.Reason) {
		t.Fatalf("expected documented SessionEnd reason after disable_all live run; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", sessionEnd, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
}

func TestGeminiCLIRuntimeAfterModelReplaceResponse(t *testing.T) {
	if strings.TrimSpace(os.Getenv(geminiRuntimeLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real Gemini runtime hook smoke", geminiRuntimeLiveEnvVar)
	}
	fixture := setupGeminiRuntimeLiveFixture(t)
	linkOutput := runGeminiLink(t, fixture.GeminiBin, fixture.HomeDir, fixture.ExtensionDir)
	if !strings.Contains(linkOutput, `linked successfully and enabled`) {
		t.Fatalf("gemini runtime live link did not report success:\n%s", linkOutput)
	}

	out, envelope := runGeminiRuntimeLivePrompt(t, fixture, "Reply with exactly OK.", "PLUGIN_KIT_AI_E2E_GEMINI_AFTER_MODEL=replace_response")
	if strings.TrimSpace(envelope.SessionID) == "" {
		t.Fatalf("expected Gemini replace-response output to include session_id\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if !strings.Contains(strings.ToLower(envelope.Response), "rewritten") {
		t.Fatalf("expected Gemini replace-response output to include rewritten model text, got %q\noutput:\n%s", envelope.Response, truncateRunes(string(out), 4000))
	}
	if envelope.Stats.Tools.TotalCalls != 0 || envelope.Stats.Tools.TotalSuccess != 0 || envelope.Stats.Tools.TotalFail != 0 {
		t.Fatalf("expected Gemini replace-response output to skip all tool activity\noutput:\n%s", truncateRunes(string(out), 4000))
	}

	lines := waitForTraceHooks(t, fixture.TracePath, 5*time.Second, "SessionStart", "BeforeModel", "AfterModel", "BeforeAgent", "AfterAgent", "SessionEnd")
	beforeModel, ok := traceFind(t, lines, "BeforeModel")
	if !ok || strings.TrimSpace(beforeModel.Outcome) != "continue" || !beforeModel.HasRequest || beforeModel.RequestSize == 0 {
		t.Fatalf("expected BeforeModel continue trace with request payload before replace-response path; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeModel, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	afterModel, ok := traceFind(t, lines, "AfterModel")
	if !ok {
		t.Fatalf("expected AfterModel replace_response trace; trace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(afterModel.Outcome) != "replace_response" || !afterModel.HasRequest || afterModel.RequestSize == 0 || !afterModel.HasResponse || afterModel.ResponseSize == 0 {
		t.Fatalf("expected AfterModel replace_response trace with request+response payloads; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", afterModel, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	beforeToolSelection, ok := traceFind(t, lines, "BeforeToolSelection")
	if !ok || strings.TrimSpace(beforeToolSelection.Outcome) != "continue" || !beforeToolSelection.HasRequest || beforeToolSelection.RequestSize == 0 {
		t.Fatalf("expected replace-response Gemini path to preserve BeforeToolSelection planning trace with request payload; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeToolSelection, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if _, ok := traceFind(t, lines, "BeforeTool"); ok {
		t.Fatalf("expected replace-response Gemini path to skip BeforeTool trace\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	afterAgent, ok := traceFind(t, lines, "AfterAgent")
	if !ok || strings.TrimSpace(afterAgent.Outcome) != "continue" {
		t.Fatalf("expected AfterAgent continue trace after replace-response path; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", afterAgent, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	sessionEnd, ok := traceFind(t, lines, "SessionEnd")
	if !ok || !isGeminiSessionEndReason(sessionEnd.Reason) {
		t.Fatalf("expected documented SessionEnd reason after replace-response live run; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", sessionEnd, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
}

func TestGeminiCLIRuntimeAfterAgentRetry(t *testing.T) {
	if strings.TrimSpace(os.Getenv(geminiRuntimeLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real Gemini runtime hook smoke", geminiRuntimeLiveEnvVar)
	}
	fixture := setupGeminiRuntimeLiveFixture(t)
	linkOutput := runGeminiLink(t, fixture.GeminiBin, fixture.HomeDir, fixture.ExtensionDir)
	if !strings.Contains(linkOutput, `linked successfully and enabled`) {
		t.Fatalf("gemini runtime live link did not report success:\n%s", linkOutput)
	}

	out, envelope := runGeminiRuntimeLivePrompt(t, fixture, "Reply with exactly OK.", "PLUGIN_KIT_AI_E2E_GEMINI_AFTER_AGENT=deny_once:Your previous answer was rejected by a hook. Retry and reply with exactly RETRY_OK.")
	if strings.TrimSpace(envelope.SessionID) == "" {
		t.Fatalf("expected Gemini after-agent-retry output to include session_id\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if !strings.Contains(strings.ToUpper(envelope.Response), "RETRY_OK") {
		t.Fatalf("expected Gemini after-agent-retry output to include RETRY_OK, got %q\noutput:\n%s", envelope.Response, truncateRunes(string(out), 4000))
	}
	if envelope.Stats.Tools.TotalCalls != 0 || envelope.Stats.Tools.TotalSuccess != 0 || envelope.Stats.Tools.TotalFail != 0 {
		t.Fatalf("expected Gemini after-agent-retry output to skip all tool activity\noutput:\n%s", truncateRunes(string(out), 4000))
	}

	lines := waitForTraceHooks(t, fixture.TracePath, 5*time.Second, "SessionStart", "BeforeAgent", "BeforeModel", "AfterModel", "AfterAgent", "SessionEnd")
	afterAgentRecs := traceFindAll(t, lines, "AfterAgent")
	if len(afterAgentRecs) < 2 {
		t.Fatalf("expected at least two AfterAgent traces for retry path; got %d\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", len(afterAgentRecs), fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	first := afterAgentRecs[0]
	last := afterAgentRecs[len(afterAgentRecs)-1]
	if strings.TrimSpace(first.Outcome) != "deny" || first.StopHookActive {
		t.Fatalf("expected first AfterAgent trace to deny without stop_hook_active; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", first, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(last.Outcome) != "continue" || !last.StopHookActive {
		t.Fatalf("expected last AfterAgent trace to continue with stop_hook_active after retry; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", last, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if !first.HasResponse || !last.HasResponse {
		t.Fatalf("expected AfterAgent retry path to keep prompt_response payloads on both attempts; first=%+v last=%+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", first, last, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	sessionEnd, ok := traceFind(t, lines, "SessionEnd")
	if !ok || !isGeminiSessionEndReason(sessionEnd.Reason) {
		t.Fatalf("expected documented SessionEnd reason after after-agent-retry live run; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", sessionEnd, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
}

func TestGeminiCLIRuntimeRewriteToolInput(t *testing.T) {
	if strings.TrimSpace(os.Getenv(geminiRuntimeLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real Gemini runtime hook smoke", geminiRuntimeLiveEnvVar)
	}
	fixture := setupGeminiRuntimeLiveFixture(t)
	linkOutput := runGeminiLink(t, fixture.GeminiBin, fixture.HomeDir, fixture.ExtensionDir)
	if !strings.Contains(linkOutput, `linked successfully and enabled`) {
		t.Fatalf("gemini runtime live link did not report success:\n%s", linkOutput)
	}

	out, envelope := runGeminiRuntimeLivePrompt(t, fixture, geminiRuntimeLiveRewriteToolPrompt, "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_TOOL=rewrite_input")
	if strings.TrimSpace(envelope.SessionID) == "" {
		t.Fatalf("expected Gemini rewrite-path output to include session_id\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if strings.TrimSpace(envelope.Response) != "OK" {
		t.Fatalf("expected Gemini rewrite-path response to preserve the final OK reply, got %q\noutput:\n%s", envelope.Response, truncateRunes(string(out), 4000))
	}
	if envelope.Stats.Tools.TotalCalls <= 0 || envelope.Stats.Tools.TotalSuccess <= 0 || envelope.Stats.Tools.TotalFail != 0 {
		t.Fatalf("expected Gemini rewrite-path output to report successful tool execution without failures\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	stats, ok := envelope.Stats.Tools.ByName["read_file"]
	if !ok {
		t.Fatalf("expected Gemini rewrite-path output to report read_file tool usage\noutput:\n%s", truncateRunes(string(out), 4000))
	}
	if maxInt(stats.Count, stats.TotalCalls) <= 0 || stats.Success <= 0 || stats.Fail != 0 {
		t.Fatalf("expected Gemini rewrite-path stats to show successful read_file usage without failures; got count=%d totalCalls=%d success=%d fail=%d\noutput:\n%s", stats.Count, stats.TotalCalls, stats.Success, stats.Fail, truncateRunes(string(out), 4000))
	}

	lines := waitForTraceHooks(t, fixture.TracePath, 5*time.Second, "SessionStart", "BeforeModel", "AfterModel", "BeforeToolSelection", "BeforeAgent", "AfterAgent", "BeforeTool", "AfterTool", "SessionEnd")
	beforeTool, ok := traceFind(t, lines, "BeforeTool")
	if !ok {
		t.Fatalf("expected BeforeTool rewrite trace; trace=%s\noutput:\n%s\ntrace_lines:\n%s", fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(beforeTool.Outcome) != "rewrite_input" || strings.TrimSpace(beforeTool.Tool) != "read_file" || !beforeTool.HasInput || beforeTool.InputSize == 0 || beforeTool.RewritePath != "README.md" {
		t.Fatalf("expected rewritten BeforeTool trace with read_file input rewritten to README.md; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", beforeTool, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	afterTool, ok := traceFind(t, lines, "AfterTool")
	if !ok || strings.TrimSpace(afterTool.Outcome) != "continue" || strings.TrimSpace(afterTool.Tool) != "read_file" || !afterTool.HasResponse || afterTool.ResponseSize == 0 {
		t.Fatalf("expected AfterTool continue trace with response payload after rewritten tool input; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", afterTool, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
	sessionEnd, ok := traceFind(t, lines, "SessionEnd")
	if !ok || !isGeminiSessionEndReason(sessionEnd.Reason) {
		t.Fatalf("expected documented SessionEnd reason after rewrite-path live run; got %+v\ntrace=%s\noutput:\n%s\ntrace_lines:\n%s", sessionEnd, fixture.TracePath, truncateRunes(string(out), 4000), strings.Join(lines, "\n"))
	}
}

func TestGeminiE2ETracePreservesOriginalRequestName(t *testing.T) {
	e2eBin := buildPluginKitAIE2E(t)
	tracePath := filepath.Join(t.TempDir(), "trace.jsonl")

	cases := []struct {
		name    string
		payload string
		hook    string
	}{
		{
			name:    "GeminiBeforeTool",
			hook:    "BeforeTool",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"BeforeTool","tool_name":"read_file","tool_input":{"file_path":"README.md"},"mcp_context":{"server":"filesystem"},"original_request_name":"tail.read_file"}`,
		},
		{
			name:    "GeminiAfterTool",
			hook:    "AfterTool",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"AfterTool","tool_name":"read_file","tool_input":{"file_path":"README.md"},"tool_response":{"llmContent":"ok","returnDisplay":"ok"},"mcp_context":{"server":"filesystem"},"original_request_name":"tail.read_file"}`,
		},
	}

	for _, tc := range cases {
		cmd := launcherCommand(e2eBin, tc.name)
		cmd.Env = append(os.Environ(), "PLUGIN_KIT_AI_E2E_TRACE="+tracePath)
		cmd.Stdin = strings.NewReader(tc.payload)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", tc.name, err, out)
		}
		if got := strings.TrimSpace(string(out)); got != `{}` {
			t.Fatalf("%s stdout = %q, want {}", tc.name, got)
		}

		lines := waitForTraceHooks(t, tracePath, 2*time.Second, tc.hook)
		rec, ok := traceFind(t, lines, tc.hook)
		if !ok {
			t.Fatalf("%s trace missing; lines=\n%s", tc.name, strings.Join(lines, "\n"))
		}
		if rec.OriginalRequestName != "tail.read_file" {
			t.Fatalf("%s original_request_name = %q, want %q\ntrace_lines:\n%s", tc.name, rec.OriginalRequestName, "tail.read_file", strings.Join(lines, "\n"))
		}
		if !rec.HasMCPContext || rec.MCPContextSize == 0 {
			t.Fatalf("%s expected non-empty mcp_context trace; has_mcp_context=%v mcp_context_size=%d\ntrace_lines:\n%s", tc.name, rec.HasMCPContext, rec.MCPContextSize, strings.Join(lines, "\n"))
		}
		if !rec.HasInput || rec.InputSize == 0 {
			t.Fatalf("%s expected non-empty tool_input trace; has_input=%v input_size=%d\ntrace_lines:\n%s", tc.name, rec.HasInput, rec.InputSize, strings.Join(lines, "\n"))
		}
		if tc.hook == "AfterTool" && (!rec.HasResponse || rec.ResponseSize == 0) {
			t.Fatalf("%s expected non-empty tool_response trace; has_response=%v response_size=%d\ntrace_lines:\n%s", tc.name, rec.HasResponse, rec.ResponseSize, strings.Join(lines, "\n"))
		}
	}
}

func TestGeminiE2ETraceCapturesModelAndToolSelectionPayloads(t *testing.T) {
	e2eBin := buildPluginKitAIE2E(t)

	cases := []struct {
		name         string
		payload      string
		hook         string
		wantRequest  bool
		wantResponse bool
	}{
		{
			name:        "GeminiBeforeModel",
			hook:        "BeforeModel",
			wantRequest: true,
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeModel","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"hi"}]}}`,
		},
		{
			name:         "GeminiAfterModel",
			hook:         "AfterModel",
			wantRequest:  true,
			wantResponse: true,
			payload:      `{"session_id":"s","cwd":".","hook_event_name":"AfterModel","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"hi"}]},"llm_response":{"candidates":[{"content":{"role":"model","parts":["ok"]}}]}}`,
		},
		{
			name:        "GeminiBeforeToolSelection",
			hook:        "BeforeToolSelection",
			wantRequest: true,
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeToolSelection","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"read the repo"}]}}`,
		},
	}

	for _, tc := range cases {
		tracePath := filepath.Join(t.TempDir(), tc.hook+".jsonl")
		cmd := launcherCommand(e2eBin, tc.name)
		cmd.Env = append(os.Environ(), "PLUGIN_KIT_AI_E2E_TRACE="+tracePath)
		cmd.Stdin = strings.NewReader(tc.payload)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", tc.name, err, out)
		}
		if got := strings.TrimSpace(string(out)); got != `{}` {
			t.Fatalf("%s stdout = %q, want {}", tc.name, got)
		}

		lines := waitForTraceHooks(t, tracePath, 2*time.Second, tc.hook)
		rec, ok := traceFind(t, lines, tc.hook)
		if !ok {
			t.Fatalf("%s trace missing; lines=\n%s", tc.name, strings.Join(lines, "\n"))
		}
		if strings.TrimSpace(rec.Outcome) != "continue" {
			t.Fatalf("%s outcome = %q, want continue\ntrace_lines:\n%s", tc.name, rec.Outcome, strings.Join(lines, "\n"))
		}
		if tc.wantRequest && (!rec.HasRequest || rec.RequestSize == 0) {
			t.Fatalf("%s expected non-empty llm_request trace; has_request=%v request_size=%d\ntrace_lines:\n%s", tc.name, rec.HasRequest, rec.RequestSize, strings.Join(lines, "\n"))
		}
		if tc.wantResponse && (!rec.HasResponse || rec.ResponseSize == 0) {
			t.Fatalf("%s expected non-empty llm_response trace; has_response=%v response_size=%d\ntrace_lines:\n%s", tc.name, rec.HasResponse, rec.ResponseSize, strings.Join(lines, "\n"))
		}
	}
}

func TestGeminiE2ETraceCapturesLifecycleAndAdvisoryHooks(t *testing.T) {
	e2eBin := buildPluginKitAIE2E(t)

	cases := []struct {
		name    string
		payload string
		hook    string
		check   func(t *testing.T, rec traceRec, lines []string)
	}{
		{
			name:    "GeminiSessionStart",
			hook:    "SessionStart",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"SessionStart","source":"startup"}`,
			check: func(t *testing.T, rec traceRec, lines []string) {
				t.Helper()
				if strings.TrimSpace(rec.Outcome) != "continue" || rec.Source != "startup" {
					t.Fatalf("SessionStart trace = %+v\ntrace_lines:\n%s", rec, strings.Join(lines, "\n"))
				}
			},
		},
		{
			name:    "GeminiSessionEnd",
			hook:    "SessionEnd",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"SessionEnd","reason":"prompt_input_exit"}`,
			check: func(t *testing.T, rec traceRec, lines []string) {
				t.Helper()
				if strings.TrimSpace(rec.Outcome) != "continue" || rec.Reason != "prompt_input_exit" {
					t.Fatalf("SessionEnd trace = %+v\ntrace_lines:\n%s", rec, strings.Join(lines, "\n"))
				}
			},
		},
	}

	for _, tc := range cases {
		tracePath := filepath.Join(t.TempDir(), tc.hook+".jsonl")
		cmd := launcherCommand(e2eBin, tc.name)
		cmd.Env = append(os.Environ(), "PLUGIN_KIT_AI_E2E_TRACE="+tracePath)
		cmd.Stdin = strings.NewReader(tc.payload)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", tc.name, err, out)
		}
		if got := strings.TrimSpace(string(out)); got != `{}` {
			t.Fatalf("%s stdout = %q, want {}", tc.name, got)
		}

		lines := waitForTraceHooks(t, tracePath, 2*time.Second, tc.hook)
		rec, ok := traceFind(t, lines, tc.hook)
		if !ok {
			t.Fatalf("%s trace missing; lines=\n%s", tc.name, strings.Join(lines, "\n"))
		}
		tc.check(t, rec, lines)
	}
}

func TestGeminiE2ETraceCapturesRuntimeLifecycleHooks(t *testing.T) {
	e2eBin := buildPluginKitAIE2E(t)

	cases := []struct {
		name    string
		payload string
		hook    string
		check   func(t *testing.T, rec traceRec, lines []string)
	}{
		{
			name:    "GeminiSessionStart",
			hook:    "SessionStart",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"SessionStart","source":"startup"}`,
			check: func(t *testing.T, rec traceRec, lines []string) {
				t.Helper()
				if strings.TrimSpace(rec.Outcome) != "continue" || rec.Source != "startup" {
					t.Fatalf("SessionStart trace = %+v\ntrace_lines:\n%s", rec, strings.Join(lines, "\n"))
				}
			},
		},
		{
			name:    "GeminiSessionEnd",
			hook:    "SessionEnd",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"SessionEnd","reason":"prompt_input_exit"}`,
			check: func(t *testing.T, rec traceRec, lines []string) {
				t.Helper()
				if strings.TrimSpace(rec.Outcome) != "continue" || rec.Reason != "prompt_input_exit" {
					t.Fatalf("SessionEnd trace = %+v\ntrace_lines:\n%s", rec, strings.Join(lines, "\n"))
				}
			},
		},
	}

	for _, tc := range cases {
		tracePath := filepath.Join(t.TempDir(), tc.hook+"-stable.jsonl")
		cmd := launcherCommand(e2eBin, tc.name)
		cmd.Env = append(os.Environ(), "PLUGIN_KIT_AI_E2E_TRACE="+tracePath)
		cmd.Stdin = strings.NewReader(tc.payload)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", tc.name, err, out)
		}
		if got := strings.TrimSpace(string(out)); got != `{}` {
			t.Fatalf("%s stdout = %q, want {}", tc.name, got)
		}
		lines := waitForTraceHooks(t, tracePath, 2*time.Second, tc.hook)
		rec, ok := traceFind(t, lines, tc.hook)
		if !ok {
			t.Fatalf("%s trace missing; lines=\n%s", tc.name, strings.Join(lines, "\n"))
		}
		tc.check(t, rec, lines)
	}
}

func TestGeminiE2ETraceCapturesRuntimeControlSemantics(t *testing.T) {
	e2eBin := buildPluginKitAIE2E(t)

	cases := []struct {
		name        string
		hook        string
		payload     string
		envKey      string
		envValue    string
		wantOutcome string
		wantSubstrs []string
	}{
		{
			name:        "GeminiSessionEnd",
			hook:        "SessionEnd",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"SessionEnd","reason":"prompt_input_exit"}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_SESSION_END",
			envValue:    "message:bye from e2e",
			wantOutcome: "message",
			wantSubstrs: []string{`"systemMessage":"bye from e2e"`},
		},
		{
			name:        "GeminiSessionStart",
			hook:        "SessionStart",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"SessionStart","source":"startup"}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_SESSION_START",
			envValue:    "message:hello from e2e",
			wantOutcome: "message",
			wantSubstrs: []string{`"systemMessage":"hello from e2e"`},
		},
		{
			name:        "GeminiBeforeTool",
			hook:        "BeforeTool",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeTool","tool_name":"read_file","tool_input":{"file_path":"README.md"}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_TOOL",
			envValue:    "deny:blocked by e2e",
			wantOutcome: "deny",
			wantSubstrs: []string{`"decision":"deny"`, `"reason":"blocked by e2e"`},
		},
		{
			name:        "GeminiBeforeModel",
			hook:        "BeforeModel",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeModel","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"hi"}]}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_MODEL",
			envValue:    "deny:blocked model request",
			wantOutcome: "deny",
			wantSubstrs: []string{`"decision":"deny"`, `"reason":"blocked model request"`},
		},
		{
			name:        "GeminiAfterAgent",
			hook:        "AfterAgent",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"AfterAgent","prompt":"hello","prompt_response":"ok","stop_hook_active":false}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_AFTER_AGENT",
			envValue:    "deny:retry please",
			wantOutcome: "deny",
			wantSubstrs: []string{`"decision":"deny"`, `"reason":"retry please"`},
		},
		{
			name:        "GeminiAfterAgent",
			hook:        "AfterAgent",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"AfterAgent","prompt":"hello","prompt_response":"ok","stop_hook_active":false}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_AFTER_AGENT",
			envValue:    "clearcontext",
			wantOutcome: "clear_context",
			wantSubstrs: []string{`"hookEventName":"AfterAgent"`, `"clearContext":true`},
		},
		{
			name:        "GeminiAfterModel",
			hook:        "AfterModel",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"AfterModel","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"hi"}]},"llm_response":{"candidates":[{"content":{"role":"model","parts":["ok"]}}]}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_AFTER_MODEL",
			envValue:    "stop:halt now",
			wantOutcome: "stop",
			wantSubstrs: []string{`"continue":false`, `"stopReason":"halt now"`},
		},
		{
			name:        "GeminiAfterTool",
			hook:        "AfterTool",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"AfterTool","tool_name":"read_file","tool_input":{"file_path":"README.md"},"tool_response":{"llmContent":"ok","returnDisplay":"ok"}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_AFTER_TOOL",
			envValue:    "stop:stop after tool",
			wantOutcome: "stop",
			wantSubstrs: []string{`"continue":false`, `"stopReason":"stop after tool"`},
		},
		{
			name:        "GeminiAfterTool",
			hook:        "AfterTool",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"AfterTool","tool_name":"read_file","tool_input":{"file_path":"README.md"},"tool_response":{"llmContent":"ok","returnDisplay":"ok"}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_AFTER_TOOL",
			envValue:    "tailcall",
			wantOutcome: "tail_call",
			wantSubstrs: []string{`"hookEventName":"AfterTool"`, `"tailToolCallRequest":{"name":"read_file","args":{"file_path":"README.md"}}`},
		},
		{
			name:        "GeminiBeforeToolSelection",
			hook:        "BeforeToolSelection",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeToolSelection","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"read the repo"}]}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_TOOL_SELECTION",
			envValue:    "quiet",
			wantOutcome: "quiet",
			wantSubstrs: []string{`"suppressOutput":true`},
		},
		{
			name:        "GeminiBeforeToolSelection",
			hook:        "BeforeToolSelection",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeToolSelection","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"read the repo"}]}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_TOOL_SELECTION",
			envValue:    "disable_all",
			wantOutcome: "disable_all",
			wantSubstrs: []string{`"hookEventName":"BeforeToolSelection"`, `"toolConfig":{"mode":"NONE"}`},
		},
	}

	for _, tc := range cases {
		tracePath := filepath.Join(t.TempDir(), tc.hook+"-control.jsonl")
		cmd := launcherCommand(e2eBin, tc.name)
		cmd.Env = append(os.Environ(),
			"PLUGIN_KIT_AI_E2E_TRACE="+tracePath,
			tc.envKey+"="+tc.envValue,
		)
		cmd.Stdin = strings.NewReader(tc.payload)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", tc.name, err, out)
		}
		got := strings.TrimSpace(string(out))
		for _, want := range tc.wantSubstrs {
			if !strings.Contains(got, want) {
				t.Fatalf("%s stdout = %q, want substring %q", tc.name, got, want)
			}
		}

		lines := waitForTraceHooks(t, tracePath, 2*time.Second, tc.hook)
		rec, ok := traceFind(t, lines, tc.hook)
		if !ok {
			t.Fatalf("%s trace missing; lines=\n%s", tc.name, strings.Join(lines, "\n"))
		}
		if strings.TrimSpace(rec.Outcome) != tc.wantOutcome {
			t.Fatalf("%s outcome = %q, want %q\ntrace_lines:\n%s", tc.name, rec.Outcome, tc.wantOutcome, strings.Join(lines, "\n"))
		}
		if tc.name == "GeminiBeforeTool" && tc.wantOutcome == "rewrite_input" && rec.RewritePath != "README.md" {
			t.Fatalf("%s rewrite_path = %q, want %q\ntrace_lines:\n%s", tc.name, rec.RewritePath, "README.md", strings.Join(lines, "\n"))
		}
	}
}

func TestGeminiE2ETraceCapturesRuntimeTransformSemantics(t *testing.T) {
	e2eBin := buildPluginKitAIE2E(t)

	cases := []struct {
		name        string
		hook        string
		payload     string
		envKey      string
		envValue    string
		wantOutcome string
		wantSubstrs []string
	}{
		{
			name:        "GeminiBeforeModel",
			hook:        "BeforeModel",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeModel","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"hi"}]}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_MODEL",
			envValue:    "rewrite_request",
			wantOutcome: "rewrite_request",
			wantSubstrs: []string{`"hookEventName":"BeforeModel"`, `"llm_request":{`, `"model":"gemini-2.5-pro"`, `"temperature":0.1`},
		},
		{
			name:        "GeminiBeforeModel",
			hook:        "BeforeModel",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeModel","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"hi"}]}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_MODEL",
			envValue:    "synthetic_response",
			wantOutcome: "synthetic_response",
			wantSubstrs: []string{`"hookEventName":"BeforeModel"`, `"llm_response":{`, `"candidates":[`, `"role":"model"`, `"synthetic"`},
		},
		{
			name:        "GeminiAfterModel",
			hook:        "AfterModel",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"AfterModel","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"hi"}]},"llm_response":{"candidates":[{"content":{"role":"model","parts":["ok"]}}]}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_AFTER_MODEL",
			envValue:    "replace_response",
			wantOutcome: "replace_response",
			wantSubstrs: []string{`"hookEventName":"AfterModel"`, `"llm_response":{`, `"candidates":[`, `"role":"model"`, `"rewritten"`},
		},
		{
			name:        "GeminiBeforeToolSelection",
			hook:        "BeforeToolSelection",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeToolSelection","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"read the repo"}]}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_TOOL_SELECTION",
			envValue:    "allow_only",
			wantOutcome: "allow_only",
			wantSubstrs: []string{`"hookEventName":"BeforeToolSelection"`, `"allowedFunctionNames":["read_file","list_directory"]`},
		},
		{
			name:        "GeminiBeforeToolSelection",
			hook:        "BeforeToolSelection",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeToolSelection","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"read the repo"}]}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_TOOL_SELECTION",
			envValue:    "force_any",
			wantOutcome: "force_any",
			wantSubstrs: []string{`"hookEventName":"BeforeToolSelection"`, `"toolConfig":{"mode":"ANY","allowedFunctionNames":["read_file"]}`},
		},
		{
			name:        "GeminiBeforeAgent",
			hook:        "BeforeAgent",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeAgent","prompt":"hello"}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_AGENT",
			envValue:    "context:repo memory",
			wantOutcome: "add_context",
			wantSubstrs: []string{`"hookEventName":"BeforeAgent"`, `"additionalContext":"repo memory"`},
		},
		{
			name:        "GeminiBeforeTool",
			hook:        "BeforeTool",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"BeforeTool","tool_name":"read_file","tool_input":{"file_path":"README.md"}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_BEFORE_TOOL",
			envValue:    "rewrite_input",
			wantOutcome: "rewrite_input",
			wantSubstrs: []string{`"hookEventName":"BeforeTool"`, `"tool_input":{"file_path":"README.md"}`},
		},
		{
			name:        "GeminiAfterTool",
			hook:        "AfterTool",
			payload:     `{"session_id":"s","cwd":".","hook_event_name":"AfterTool","tool_name":"read_file","tool_input":{"file_path":"README.md"},"tool_response":{"llmContent":"ok","returnDisplay":"ok"}}`,
			envKey:      "PLUGIN_KIT_AI_E2E_GEMINI_AFTER_TOOL",
			envValue:    "context:redacted",
			wantOutcome: "add_context",
			wantSubstrs: []string{`"hookEventName":"AfterTool"`, `"additionalContext":"redacted"`},
		},
	}

	for _, tc := range cases {
		tracePath := filepath.Join(t.TempDir(), tc.hook+"-transform.jsonl")
		cmd := launcherCommand(e2eBin, tc.name)
		cmd.Env = append(os.Environ(),
			"PLUGIN_KIT_AI_E2E_TRACE="+tracePath,
			tc.envKey+"="+tc.envValue,
		)
		cmd.Stdin = strings.NewReader(tc.payload)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", tc.name, err, out)
		}
		got := strings.TrimSpace(string(out))
		for _, want := range tc.wantSubstrs {
			if !strings.Contains(got, want) {
				t.Fatalf("%s stdout = %q, want substring %q", tc.name, got, want)
			}
		}

		lines := waitForTraceHooks(t, tracePath, 2*time.Second, tc.hook)
		rec, ok := traceFind(t, lines, tc.hook)
		if !ok {
			t.Fatalf("%s trace missing; lines=\n%s", tc.name, strings.Join(lines, "\n"))
		}
		if strings.TrimSpace(rec.Outcome) != tc.wantOutcome {
			t.Fatalf("%s outcome = %q, want %q\ntrace_lines:\n%s", tc.name, rec.Outcome, tc.wantOutcome, strings.Join(lines, "\n"))
		}
	}
}

func parseGeminiJSONEnvelope(t *testing.T, out []byte) geminiCLIEnvelope {
	t.Helper()
	start := bytes.Index(out, []byte("{"))
	if start < 0 {
		t.Fatalf("gemini live output missing JSON envelope:\n%s", truncateRunes(string(out), 4000))
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(out); i++ {
		b := out[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				var envelope geminiCLIEnvelope
				body := out[start : i+1]
				if err := json.Unmarshal(body, &envelope); err != nil {
					t.Fatalf("parse gemini live JSON envelope: %v\n%s", err, truncateRunes(string(out), 4000))
				}
				return envelope
			}
		}
	}
	t.Fatalf("gemini live output ended before JSON envelope closed:\n%s", truncateRunes(string(out), 4000))
	return geminiCLIEnvelope{}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func geminiBinaryOrSkip(t *testing.T) string {
	t.Helper()
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_SKIP_GEMINI_CLI")) == "1" {
		t.Skip("PLUGIN_KIT_AI_SKIP_GEMINI_CLI=1")
	}
	geminiBin := resolveGeminiBinaryEnv()
	if geminiBin == "" {
		var err error
		geminiBin, err = exec.LookPath("gemini")
		if err != nil {
			t.Skip("set PLUGIN_KIT_AI_E2E_GEMINI or install gemini in PATH to run local Gemini CLI extension e2e")
		}
	}
	if out, err := exec.Command(geminiVersionCommand(geminiBin)[0], geminiVersionCommand(geminiBin)[1:]...).CombinedOutput(); err != nil {
		t.Skipf("Gemini CLI is not runnable in this environment: %v\n%s", err, out)
	}
	return geminiBin
}

func geminiVersionCommand(geminiBin string) []string {
	return []string{geminiBin, "--version"}
}

func resolveGeminiBinaryEnv() string {
	for _, key := range []string{"PLUGIN_KIT_AI_E2E_GEMINI"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func isGeminiSessionEndReason(reason string) bool {
	switch strings.TrimSpace(reason) {
	case "exit", "clear", "logout", "prompt_input_exit", "other":
		return true
	default:
		return false
	}
}

func runGeminiLink(t *testing.T, geminiBin, homeDir, extensionDir string) string {
	return runGeminiCommandWithInput(t, geminiBin, homeDir, extensionDir, "stable\n", "extensions", "link", extensionDir, "--consent")
}

func runGeminiConfig(t *testing.T, geminiBin, homeDir, extensionDir, name, setting, input string) string {
	return runGeminiCommandWithInput(t, geminiBin, homeDir, extensionDir, input, "extensions", "config", name, setting, "--scope", "user")
}

func runGeminiCommand(t *testing.T, geminiBin, homeDir, extensionDir string, args ...string) string {
	return runGeminiCommandWithInput(t, geminiBin, homeDir, extensionDir, "", args...)
}

func runGeminiCommandWithInput(t *testing.T, geminiBin, homeDir, extensionDir, input string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, geminiBin, args...)
	cmd.Dir = extensionDir
	cmd.Env = geminiCLIEnv(homeDir)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("gemini command %q timed out; %s\n%s", strings.Join(args, " "), geminiAuthRecoveryHint(string(out)), truncateRunes(string(out), 4000))
	}
	if err != nil {
		if geminiEnvironmentIssue(string(out)) {
			t.Skipf("gemini environment is not ready for isolated live e2e; %s\n%s", geminiAuthRecoveryHint(string(out)), truncateRunes(string(out), 4000))
		}
		t.Fatalf("gemini command %q: %v\nhint=%s\n%s", strings.Join(args, " "), err, geminiCommandRecoveryHint(args), truncateRunes(string(out), 4000))
	}
	text := string(out)
	t.Logf("gemini %s output: %s", strings.Join(args, " "), truncateRunes(text, 4000))
	return text
}

func geminiEnvironmentIssue(output string) bool {
	lower := strings.ToLower(output)
	markers := []string{
		"please set an auth method",
		"not authenticated",
		"authentication required",
		"login required",
		"unauthorized",
		"forbidden",
		"failed to sign in",
		"current account is not eligible",
		"not currently available in your location",
		"please contact your administrator to request an entitlement",
		"unable_to_get_issuer_cert_locally",
		"unable to get local issuer certificate",
		"safe mode",
		"untrusted workspace",
		"extension management is restricted",
		"workspace settings are ignored",
		"mcp servers do not connect",
		"resource_exhausted",
		"model_capacity_exhausted",
		"no capacity available for model",
		"you have exhausted your capacity on this model",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func geminiAuthRecoveryHint(output string) string {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "unable_to_get_issuer_cert_locally"),
		strings.Contains(lower, "unable to get local issuer certificate"):
		return "per Gemini CLI troubleshooting, corporate TLS interception may require NODE_USE_SYSTEM_CA=1 or NODE_EXTRA_CA_CERTS; then retry."
	case strings.Contains(lower, "safe mode"),
		strings.Contains(lower, "untrusted workspace"),
		strings.Contains(lower, "extension management is restricted"),
		strings.Contains(lower, "workspace settings are ignored"),
		strings.Contains(lower, "mcp servers do not connect"):
		return "per Gemini CLI trusted-folders docs, trust this workspace or parent folder in ~/.gemini/trustedFolders.json or via /permissions, then rerun the Gemini live smoke."
	case strings.Contains(lower, "google_cloud_project"),
		strings.Contains(lower, "google_cloud_project_id"),
		strings.Contains(lower, "gemini code assist"),
		strings.Contains(lower, "current account is not eligible"),
		strings.Contains(lower, "request contains an invalid argument"),
		strings.Contains(lower, "administrator to request an entitlement"):
		return "per Gemini CLI auth docs, headless mode needs cached auth or env-based auth, and Workspace/Code Assist accounts often also need GOOGLE_CLOUD_PROJECT."
	case strings.Contains(lower, "resource_exhausted"),
		strings.Contains(lower, "model_capacity_exhausted"),
		strings.Contains(lower, "no capacity available for model"),
		strings.Contains(lower, "you have exhausted your capacity on this model"):
		return "Gemini CLI reached a vendor capacity limit for the current model; rerun the live smoke later or switch the account/model routing before treating this as a plugin-kit-ai regression."
	default:
		return "per Gemini CLI auth docs, headless mode needs cached auth or env-based auth (GEMINI_API_KEY or Vertex AI); verify auth and retry."
	}
}

func geminiCommandRecoveryHint(args []string) string {
	if len(args) >= 2 && args[0] == "extensions" {
		switch args[1] {
		case "validate":
			return "verify the workspace is trusted, rerender gemini-extension.json if needed, then rerun gemini extensions validate <path>."
		case "link":
			return "verify the extension repo renders cleanly; after a successful link, restart Gemini CLI before checking session/runtime behavior."
		case "config", "enable", "disable":
			return "verify the extension repo renders cleanly; after changing extension settings or enablement, restart Gemini CLI before checking the new behavior."
		case "list":
			return "after link or config changes, restart Gemini CLI before relying on extensions list or session-visible extension state."
		}
	}
	return "verify the extension repo renders cleanly, then rerun the Gemini extension command."
}

func TestGeminiEnvironmentIssue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{name: "auth method missing", output: "Please set an auth method before continuing.", want: true},
		{name: "workspace entitlement missing", output: "Please contact your administrator to request an entitlement.", want: true},
		{name: "tls interception", output: "UNABLE_TO_GET_ISSUER_CERT_LOCALLY", want: true},
		{name: "safe mode", output: "The CLI is running in safe mode because this is an untrusted workspace.", want: true},
		{name: "vendor capacity", output: "RESOURCE_EXHAUSTED: No capacity available for model gemini-3-flash-preview on the server", want: true},
		{name: "plain runtime failure", output: "hook command exited with status 1", want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := geminiEnvironmentIssue(tc.output); got != tc.want {
				t.Fatalf("geminiEnvironmentIssue(%q) = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}

func TestGeminiAuthRecoveryHint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		output       string
		wantContains string
	}{
		{name: "tls", output: "UNABLE_TO_GET_ISSUER_CERT_LOCALLY", wantContains: "NODE_USE_SYSTEM_CA=1"},
		{name: "trusted folder", output: "Extension management is restricted in safe mode for an untrusted workspace", wantContains: "trustedFolders.json"},
		{name: "workspace project", output: "Set GOOGLE_CLOUD_PROJECT before using Gemini Code Assist", wantContains: "GOOGLE_CLOUD_PROJECT"},
		{name: "capacity", output: "MODEL_CAPACITY_EXHAUSTED", wantContains: "vendor capacity limit"},
		{name: "default", output: "not authenticated", wantContains: "GEMINI_API_KEY"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := geminiAuthRecoveryHint(tc.output); !strings.Contains(got, tc.wantContains) {
				t.Fatalf("geminiAuthRecoveryHint(%q) = %q, want substring %q", tc.output, got, tc.wantContains)
			}
		})
	}
}

func TestGeminiCommandRecoveryHint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		args         []string
		wantContains string
	}{
		{name: "validate", args: []string{"extensions", "validate", "/tmp/demo"}, wantContains: "trusted"},
		{name: "link", args: []string{"extensions", "link", "/tmp/demo"}, wantContains: "restart Gemini CLI"},
		{name: "config", args: []string{"extensions", "config", "demo", "release-profile"}, wantContains: "settings or enablement"},
		{name: "list", args: []string{"extensions", "list"}, wantContains: "extensions list"},
		{name: "default", args: []string{"extensions", "unknown"}, wantContains: "renders cleanly"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := geminiCommandRecoveryHint(tc.args); !strings.Contains(got, tc.wantContains) {
				t.Fatalf("geminiCommandRecoveryHint(%v) = %q, want substring %q", tc.args, got, tc.wantContains)
			}
		})
	}
}

func TestResolveGeminiBinaryEnv(t *testing.T) {
	for _, key := range []string{"PLUGIN_KIT_AI_E2E_GEMINI"} {
		t.Setenv(key, "")
	}
	if got := resolveGeminiBinaryEnv(); got != "" {
		t.Fatalf("resolveGeminiBinaryEnv() = %q, want empty", got)
	}
	t.Setenv("PLUGIN_KIT_AI_E2E_GEMINI", "/primary/gemini")
	if got := resolveGeminiBinaryEnv(); got != "/primary/gemini" {
		t.Fatalf("resolveGeminiBinaryEnv() with primary env = %q, want %q", got, "/primary/gemini")
	}
}

func geminiCLIEnv(homeDir string) []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+5)
	for _, item := range env {
		switch {
		case strings.HasPrefix(item, "HOME="),
			strings.HasPrefix(item, "USERPROFILE="),
			strings.HasPrefix(item, "XDG_CONFIG_HOME="),
			strings.HasPrefix(item, "XDG_DATA_HOME="),
			strings.HasPrefix(item, "XDG_STATE_HOME="):
			continue
		default:
			out = append(out, item)
		}
	}
	out = append(out,
		"HOME="+homeDir,
		"USERPROFILE="+homeDir,
		"GEMINI_CLI_HOME="+homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(homeDir, ".config"),
		"XDG_DATA_HOME="+filepath.Join(homeDir, ".local", "share"),
		"XDG_STATE_HOME="+filepath.Join(homeDir, ".local", "state"),
	)
	return out
}

func seedGeminiHome(t *testing.T, homeDir string, trustedDirs ...string) {
	t.Helper()
	geminiDir := filepath.Join(homeDir, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"projects.json",
		"settings.json",
		"oauth_creds.json",
		"google_accounts.json",
		"installation_id",
		"state.json",
	} {
		src := filepath.Join(os.Getenv("HOME"), ".gemini", rel)
		body, err := os.ReadFile(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		dst := filepath.Join(geminiDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if path := filepath.Join(geminiDir, "projects.json"); !fileExists(path) {
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if len(trustedDirs) > 0 {
		trustedFolders := map[string]string{}
		trustedFoldersPath := filepath.Join(geminiDir, "trustedFolders.json")
		if body, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".gemini", "trustedFolders.json")); err == nil {
			if err := json.Unmarshal(body, &trustedFolders); err != nil {
				t.Fatalf("parse source trustedFolders.json: %v\n%s", err, body)
			}
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		for _, dir := range trustedDirs {
			dir = strings.TrimSpace(dir)
			if dir == "" {
				continue
			}
			absDir, err := filepath.Abs(dir)
			if err != nil {
				t.Fatal(err)
			}
			trustedFolders[filepath.Clean(absDir)] = "TRUST_FOLDER"
		}
		body, err := json.MarshalIndent(trustedFolders, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		body = append(body, '\n')
		if err := os.WriteFile(trustedFoldersPath, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSeedGeminiHomeAddsTrustedFolders(t *testing.T) {
	sourceHome := t.TempDir()
	t.Setenv("HOME", sourceHome)
	if err := os.MkdirAll(filepath.Join(sourceHome, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	destHome := t.TempDir()
	trustedDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(trustedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	seedGeminiHome(t, destHome, trustedDir)

	body, err := os.ReadFile(filepath.Join(destHome, ".gemini", "trustedFolders.json"))
	if err != nil {
		t.Fatal(err)
	}
	var trusted map[string]string
	if err := json.Unmarshal(body, &trusted); err != nil {
		t.Fatalf("parse trustedFolders.json: %v\n%s", err, body)
	}
	absTrustedDir, err := filepath.Abs(trustedDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := trusted[filepath.Clean(absTrustedDir)]; got != "TRUST_FOLDER" {
		t.Fatalf("trustedFolders[%q] = %q, want %q", filepath.Clean(absTrustedDir), got, "TRUST_FOLDER")
	}
}

func TestSeedGeminiHomeMergesSourceTrustedFolders(t *testing.T) {
	sourceHome := t.TempDir()
	t.Setenv("HOME", sourceHome)
	sourceGeminiDir := filepath.Join(sourceHome, ".gemini")
	if err := os.MkdirAll(sourceGeminiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existingTrusted := map[string]string{
		"/source/already-trusted": "TRUST_PARENT",
	}
	body, err := json.MarshalIndent(existingTrusted, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(filepath.Join(sourceGeminiDir, "trustedFolders.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}

	destHome := t.TempDir()
	newTrustedDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(newTrustedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	seedGeminiHome(t, destHome, newTrustedDir)

	mergedBody, err := os.ReadFile(filepath.Join(destHome, ".gemini", "trustedFolders.json"))
	if err != nil {
		t.Fatal(err)
	}
	var merged map[string]string
	if err := json.Unmarshal(mergedBody, &merged); err != nil {
		t.Fatalf("parse merged trustedFolders.json: %v\n%s", err, mergedBody)
	}
	if got := merged["/source/already-trusted"]; got != "TRUST_PARENT" {
		t.Fatalf("merged trustedFolders lost source entry: got %q", got)
	}
	absNewTrustedDir, err := filepath.Abs(newTrustedDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := merged[filepath.Clean(absNewTrustedDir)]; got != "TRUST_FOLDER" {
		t.Fatalf("merged trustedFolders[%q] = %q, want %q", filepath.Clean(absNewTrustedDir), got, "TRUST_FOLDER")
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), want) {
		t.Fatalf("file %s missing %q:\n%s", path, want, body)
	}
}

func assertEnablementRule(t *testing.T, path, extensionName, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]struct {
		Overrides []string `json:"overrides"`
	}
	if err := json.Unmarshal(body, &config); err != nil {
		t.Fatalf("parse extension enablement: %v\n%s", err, body)
	}
	entry, ok := config[extensionName]
	if !ok {
		t.Fatalf("enablement config missing %q:\n%s", extensionName, body)
	}
	for _, override := range entry.Overrides {
		if override == want {
			return
		}
	}
	t.Fatalf("enablement overrides for %q = %#v, want %q", extensionName, entry.Overrides, want)
}
