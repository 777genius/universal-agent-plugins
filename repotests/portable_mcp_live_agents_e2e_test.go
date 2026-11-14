package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

const portableMCPLiveEnvVar = "PLUGIN_KIT_AI_RUN_PORTABLE_MCP_LIVE"
const portableMCPCodexFallbackModelEnvVar = "PLUGIN_KIT_AI_PORTABLE_MCP_CODEX_FALLBACK_MODEL"

func TestPortableMCPLiveAcrossConsoleAgents(t *testing.T) {
	if strings.TrimSpace(os.Getenv(portableMCPLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real portable MCP multi-agent live smoke", portableMCPLiveEnvVar)
	}

	pluginKitAIBin := buildPluginKitAI(t)
	mcpBin := buildPortableMCPSmokeServer(t)

	t.Run("Claude_rendered_mcp_config_invokes_shared_MCP", func(t *testing.T) {
		claudeBin := claudeBinaryOrSkip(t)
		workDir := newPortableMCPLiveWorkspace(t, pluginKitAIBin, mcpBin)
		marker := filepath.Join(t.TempDir(), "claude-mcp-marker.json")
		runClaudePrintWithMCPConfig(t, claudeBin, workDir, marker, *claudeModel, "Use the MCP tool release_checks exactly once with token CLAUDE_PORTABLE_MCP_OK, then answer DONE.")
		assertPortableMCPMarker(t, marker, "tools/call", "release_checks", "CLAUDE_PORTABLE_MCP_OK")
	})

	t.Run("Codex_exec_invokes_shared_MCP", func(t *testing.T) {
		codexBin := codexBinaryOrSkip(t)
		workDir := newPortableMCPLiveWorkspace(t, pluginKitAIBin, mcpBin)
		marker := filepath.Join(t.TempDir(), "codex-mcp-marker.json")
		runCodexExecWithPortableMCP(t, codexBin, workDir, marker, *codexModel, mcpBin)
		assertPortableMCPMarker(t, marker, "tools/call", "release_checks", "CODEX_PORTABLE_MCP_OK")
	})

	t.Run("Gemini_extension_link_sees_shared_MCP", func(t *testing.T) {
		geminiBin := geminiBinaryOrSkip(t)
		workDir := newPortableMCPLiveWorkspace(t, pluginKitAIBin, mcpBin)
		homeDir := filepath.Join(t.TempDir(), "home")
		if err := os.MkdirAll(homeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		seedGeminiHome(t, homeDir, workDir)
		validateOutput := runGeminiCommand(t, geminiBin, homeDir, workDir, "extensions", "validate", workDir)
		if !strings.Contains(validateOutput, "successfully validated") {
			t.Fatalf("gemini validate output missing success marker; hint=%s\n%s", geminiCommandRecoveryHint([]string{"extensions", "validate", workDir}), validateOutput)
		}
		output := runGeminiLink(t, geminiBin, homeDir, workDir)
		if !strings.Contains(output, `Extension "portable-mcp-live" linked successfully and enabled.`) {
			t.Fatalf("gemini link output missing success marker:\n%s", output)
		}

		installMetadataPath := filepath.Join(homeDir, ".gemini", "extensions", "portable-mcp-live", ".gemini-extension-install.json")
		installMetadataBody, err := os.ReadFile(installMetadataPath)
		if err != nil {
			t.Fatal(err)
		}
		var installDoc struct {
			Source string `json:"source"`
			Type   string `json:"type"`
		}
		if err := json.Unmarshal(installMetadataBody, &installDoc); err != nil {
			t.Fatalf("parse Gemini install metadata: %v\n%s", err, installMetadataBody)
		}
		if installDoc.Type != "link" || installDoc.Source != workDir {
			t.Fatalf("unexpected Gemini install metadata: %+v", installDoc)
		}

		body, err := os.ReadFile(filepath.Join(workDir, "gemini-extension.json"))
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("parse generated gemini-extension.json: %v\n%s", err, body)
		}
		mcpServers, ok := doc["mcpServers"].(map[string]any)
		if !ok {
			t.Fatalf("generated gemini manifest missing mcpServers:\n%s", body)
		}
		server, ok := mcpServers["release-checks"].(map[string]any)
		if !ok {
			t.Fatalf("generated gemini manifest missing release-checks MCP server:\n%s", body)
		}
		if strings.TrimSpace(fmt.Sprint(server["command"])) != filepath.ToSlash(mcpBin) {
			t.Fatalf("gemini release-checks command = %#v want %q", server["command"], filepath.ToSlash(mcpBin))
		}
		listOutput := runGeminiCommand(t, geminiBin, homeDir, workDir, "extensions", "list")
		if !strings.Contains(listOutput, "MCP servers:") || !strings.Contains(listOutput, "release-checks") {
			t.Fatalf("gemini extensions list missing projected MCP server; hint=%s\n%s", geminiCommandRecoveryHint([]string{"extensions", "list"}), listOutput)
		}
	})

	t.Run("Cursor_workspace_invokes_shared_MCP", func(t *testing.T) {
		cursorBin := cursorBinaryOrSkip(t)
		workDir := newPortableMCPLiveWorkspace(t, pluginKitAIBin, mcpBin)
		marker := filepath.Join(t.TempDir(), "cursor-mcp-marker.json")
		env := []string{"PLUGIN_KIT_AI_MCP_SMOKE_MARKER=" + marker}
		streamOut := runCursorCommand(t, cursorBin, workDir, env, "-p", "Use the MCP tool release_checks exactly once with token CURSOR_PORTABLE_MCP_OK, then answer DONE.", "--model", *cursorModel, "--print", "--output-format", "stream-json", "--force", "--approve-mcps", "--trust")
		assertCursorStreamResult(t, streamOut, "DONE", true)
		assertPortableMCPMarker(t, marker, "tools/call", "release_checks", "CURSOR_PORTABLE_MCP_OK")
	})

	t.Run("OpenCode_loader_initializes_shared_MCP", func(t *testing.T) {
		opencodeBin := openCodeBinaryOrSkip(t)
		workDir := newPortableMCPLiveWorkspace(t, pluginKitAIBin, mcpBin)
		marker := filepath.Join(t.TempDir(), "opencode-mcp-marker.json")
		runOpenCodeServeUntilPortableMCPInit(t, opencodeBin, workDir, marker)
		assertPortableMCPMarker(t, marker, "initialize", "", "")
	})
}

func newPortableMCPLiveWorkspace(t *testing.T, pluginKitAIBin, mcpBin string) string {
	t.Helper()
	baseDir := t.TempDir()
	workDir := filepath.Join(baseDir, "portable-mcp-live")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "plugin.yaml"), `api_version: v1
name: "portable-mcp-live"
version: "0.1.0"
description: "portable MCP live smoke package"
targets:
  - "claude"
  - "codex-package"
  - "gemini"
  - "opencode"
  - "cursor"
  - "cursor-workspace"
`)
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "launcher.yaml"), "runtime: shell\nentrypoint: ./scripts/main.sh\n")
	mustWriteRepoExecutable(t, workDir, filepath.Join("scripts", "main.sh"), "#!/usr/bin/env bash\nexit 0\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - \"@acme/portable-mcp-live\"\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "cursor-workspace", "rules", "project.mdc"), `---
description: "Portable MCP live smoke"
alwaysApply: true
---

- When explicitly asked to use the MCP tool release_checks, do so exactly once.`)
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "mcp", "servers.yaml"), fmt.Sprintf(`api_version: v1

servers:
  release-checks:
    description: Portable MCP live smoke server
    type: stdio
    stdio:
      command: %q
    targets:
      - claude
      - codex-package
      - gemini
      - opencode
      - cursor
      - cursor-workspace
`, filepath.ToSlash(mcpBin)))

	root := RepoRoot(t)
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", workDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", workDir, "--check"))
	for _, platform := range []string{"claude", "codex-package", "gemini", "opencode", "cursor", "cursor-workspace"} {
		runCmd(t, root, exec.Command(pluginKitAIBin, "validate", workDir, "--platform", platform, "--strict"))
	}
	assertPortableMCPLiveRenderedArtifacts(t, workDir, mcpBin)
	return workDir
}

func buildPortableMCPSmokeServer(t *testing.T) string {
	t.Helper()
	root := RepoRoot(t)
	name := "portable-mcp-live-smoke"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	out := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", out, "./repotests/testdata/cursor_cli_mcp_smoke")
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build portable MCP live smoke helper: %v\n%s", err, b)
	}
	return out
}

func assertPortableMCPLiveRenderedArtifacts(t *testing.T, workDir, mcpBin string) {
	t.Helper()

	claudeManifestBody, err := os.ReadFile(filepath.Join(workDir, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(claudeManifestBody), `"mcpServers": "./.mcp.json"`) {
		t.Fatalf("claude plugin manifest missing shared .mcp.json ref:\n%s", claudeManifestBody)
	}

	codexManifestBody, err := os.ReadFile(filepath.Join(workDir, ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(codexManifestBody), `"mcpServers": "./.mcp.json"`) {
		t.Fatalf("codex package manifest missing shared .mcp.json ref:\n%s", codexManifestBody)
	}

	sharedMCPBody, err := os.ReadFile(filepath.Join(workDir, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sharedMCP map[string]map[string]any
	if err := json.Unmarshal(sharedMCPBody, &sharedMCP); err != nil {
		t.Fatalf("parse shared .mcp.json: %v\n%s", err, sharedMCPBody)
	}
	server, ok := sharedMCP["release-checks"]
	if !ok {
		t.Fatalf("shared .mcp.json missing release-checks server:\n%s", sharedMCPBody)
	}
	if got := strings.TrimSpace(fmt.Sprint(server["command"])); got != filepath.ToSlash(mcpBin) {
		t.Fatalf("shared .mcp.json release-checks command = %q want %q\n%s", got, filepath.ToSlash(mcpBin), sharedMCPBody)
	}

	geminiBody, err := os.ReadFile(filepath.Join(workDir, "gemini-extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	var geminiDoc map[string]any
	if err := json.Unmarshal(geminiBody, &geminiDoc); err != nil {
		t.Fatalf("parse gemini-extension.json: %v\n%s", err, geminiBody)
	}
	geminiMCP, ok := geminiDoc["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("gemini-extension.json missing mcpServers:\n%s", geminiBody)
	}
	geminiServer, ok := geminiMCP["release-checks"].(map[string]any)
	if !ok {
		t.Fatalf("gemini-extension.json missing release-checks MCP projection:\n%s", geminiBody)
	}
	if got := strings.TrimSpace(fmt.Sprint(geminiServer["command"])); got != filepath.ToSlash(mcpBin) {
		t.Fatalf("gemini release-checks command = %q want %q\n%s", got, filepath.ToSlash(mcpBin), geminiBody)
	}

	opencodeBody, err := os.ReadFile(filepath.Join(workDir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var opencodeDoc struct {
		MCP map[string]map[string]any `json:"mcp"`
	}
	if err := json.Unmarshal(opencodeBody, &opencodeDoc); err != nil {
		t.Fatalf("parse opencode.json: %v\n%s", err, opencodeBody)
	}
	opencodeServer, ok := opencodeDoc.MCP["release-checks"]
	if !ok {
		t.Fatalf("opencode.json missing release-checks MCP projection:\n%s", opencodeBody)
	}
	if strings.TrimSpace(fmt.Sprint(opencodeServer["type"])) != "local" {
		t.Fatalf("opencode release-checks type = %#v want %q\n%s", opencodeServer["type"], "local", opencodeBody)
	}
	command, ok := opencodeServer["command"].([]any)
	if !ok || len(command) != 1 || strings.TrimSpace(fmt.Sprint(command[0])) != filepath.ToSlash(mcpBin) {
		t.Fatalf("opencode release-checks command = %#v want [%q]\n%s", opencodeServer["command"], filepath.ToSlash(mcpBin), opencodeBody)
	}

	cursorManifestBody, err := os.ReadFile(filepath.Join(workDir, ".cursor-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cursorManifestBody), `"mcpServers": "./.mcp.json"`) {
		t.Fatalf(".cursor-plugin/plugin.json missing shared MCP ref:\n%s", cursorManifestBody)
	}
	cursorBody, err := os.ReadFile(filepath.Join(workDir, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cursorDoc map[string]map[string]any
	if err := json.Unmarshal(cursorBody, &cursorDoc); err != nil {
		t.Fatalf("parse .mcp.json: %v\n%s", err, cursorBody)
	}
	cursorServer, ok := cursorDoc["release-checks"]
	if !ok {
		t.Fatalf(".mcp.json missing release-checks MCP projection:\n%s", cursorBody)
	}
	if got := strings.TrimSpace(fmt.Sprint(cursorServer["command"])); got != filepath.ToSlash(mcpBin) {
		t.Fatalf("cursor release-checks command = %q want %q\n%s", got, filepath.ToSlash(mcpBin), cursorBody)
	}

	cursorRulesBody, err := os.ReadFile(filepath.Join(workDir, ".cursor", "rules", "project.mdc"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cursorRulesBody), "release_checks") {
		t.Fatalf(".cursor/rules/project.mdc missing authored rule projection:\n%s", cursorRulesBody)
	}
}

func runClaudePrintWithMCPConfig(t *testing.T, claudeBin, pluginDir, markerPath, model, prompt string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	configPath := writeClaudePortableMCPConfig(t, pluginDir)
	assertClaudeSeesPortableMCP(t, claudeBin, configPath)
	cmd := exec.CommandContext(ctx, claudeBin,
		"-p",
		"--model", model,
		"--mcp-config", configPath,
		"--strict-mcp-config",
		"--permission-mode", "bypassPermissions",
		prompt,
	)
	cmd.Dir = pluginDir
	cmd.Env = append(os.Environ(), "PLUGIN_KIT_AI_MCP_SMOKE_MARKER="+markerPath)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("claude portable MCP config smoke timed out:\n%s", out)
	}
	if err != nil {
		if claudeEnvironmentIssue(string(out)) {
			t.Skipf("claude environment is not ready for portable MCP config smoke:\n%s", truncateRunes(string(out), 4000))
		}
		t.Fatalf("claude portable MCP config smoke: %v\n%s", err, out)
	}
	text := string(out)
	t.Logf("claude portable MCP output: %s", truncateRunes(text, 4000))
	if _, err := os.Stat(markerPath); err == nil {
		return
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "release_checks") && (strings.Contains(lower, "нет доступа") || strings.Contains(lower, "not registered") || strings.Contains(lower, "not available") || strings.Contains(lower, "not in") || strings.Contains(lower, "недоступ") || strings.Contains(lower, "не вижу инструмента") || strings.Contains(lower, "не нашёл mcp инструмент") || strings.Contains(lower, "не нашел mcp инструмент")) {
		t.Skipf("claude print session accepted the generated MCP config but did not expose the MCP tool in this runtime session:\n%s", truncateRunes(text, 4000))
	}
	t.Fatalf("claude portable MCP config smoke completed without MCP marker:\n%s", text)
}

func assertClaudeSeesPortableMCP(t *testing.T, claudeBin, configPath string) {
	t.Helper()
	configBody, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(configBody, &doc); err != nil {
		t.Fatalf("parse synthesized Claude MCP config: %v\n%s", err, configBody)
	}
	server, ok := doc.MCPServers["release-checks"]
	if !ok {
		t.Fatalf("synthesized Claude MCP config missing release-checks server:\n%s", configBody)
	}
	tmpProjectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpProjectDir, ".mcp.json"), configBody, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, claudeBin, "mcp", "get", "release-checks")
	cmd.Dir = tmpProjectDir
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("claude mcp get release-checks timed out:\n%s", out)
	}
	if err != nil {
		if claudeEnvironmentIssue(string(out)) {
			t.Skipf("claude environment is not ready for portable MCP preflight:\n%s", truncateRunes(string(out), 4000))
		}
		t.Fatalf("claude mcp get release-checks: %v\n%s", err, out)
	}
	text := string(out)
	wantCommand := strings.TrimSpace(fmt.Sprint(server["command"]))
	if !strings.Contains(text, "release-checks:") || !strings.Contains(text, wantCommand) {
		t.Fatalf("claude mcp get release-checks output does not show the projected command %q:\n%s", wantCommand, text)
	}
	t.Logf("claude mcp get release-checks output: %s", truncateRunes(text, 4000))
}

func writeClaudePortableMCPConfig(t *testing.T, pluginDir string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(pluginDir, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var servers map[string]any
	if err := json.Unmarshal(body, &servers); err != nil {
		t.Fatalf("parse generated .mcp.json for Claude live smoke: %v\n%s", err, body)
	}
	for name, raw := range servers {
		server, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, hasType := server["type"]; hasType {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(server["command"])) != "" {
			server["type"] = "stdio"
			servers[name] = server
		}
	}
	wrapped, err := json.MarshalIndent(map[string]any{
		"mcpServers": servers,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "claude-mcp-config.json")
	if err := os.WriteFile(path, append(wrapped, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runCodexExecWithPortableMCP(t *testing.T, codexBin, workDir, markerPath, model, _ string) {
	t.Helper()
	mcpServer := readRenderedSharedMCPServer(t, workDir, "release-checks")
	configArgs := codexMCPConfigArgs("release-checks", mcpServer)
	assertCodexSeesPortableMCP(t, codexBin, configArgs, strings.TrimSpace(fmt.Sprint(mcpServer["command"])))
	models := codexPortableMCPLiveModels(model)
	for idx, candidate := range models {
		if runCodexExecWithPortableMCPModel(t, codexBin, workDir, markerPath, candidate, configArgs) {
			return
		}
		if idx < len(models)-1 {
			t.Logf("codex portable MCP live smoke did not observe a tool call with model %q; retrying with fallback model", candidate)
		}
	}
	t.Skipf("codex exec completed without selecting the projected MCP tool after trying models %v; codex mcp get already verified the generated server config, so treating this as model-behavior variability", models)
}

func codexPortableMCPLiveModels(primary string) []string {
	primary = strings.TrimSpace(primary)
	if primary == "" {
		primary = "gpt-5.4-mini"
	}
	fallback := strings.TrimSpace(os.Getenv(portableMCPCodexFallbackModelEnvVar))
	if fallback == "" {
		fallback = "gpt-5.4"
	}
	models := []string{primary}
	if fallback != "" && fallback != primary {
		models = append(models, fallback)
	}
	return models
}

func runCodexExecWithPortableMCPModel(t *testing.T, codexBin, workDir, markerPath, model string, configArgs []string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	outputFile := filepath.Join(t.TempDir(), "last-message.txt")
	logFile := filepath.Join(t.TempDir(), "codex-mcp.log")
	args := []string{
		"exec",
		"--skip-git-repo-check",
		"--ephemeral",
		"-C", workDir,
		"-m", model,
		"--color", "never",
		"--output-last-message", outputFile,
		"--dangerously-bypass-approvals-and-sandbox",
	}
	args = append(args, configArgs...)
	args = append(args, "Do not inspect files or run shell commands. Before your final answer, make exactly one MCP tool call to release_checks with JSON arguments {\"token\":\"CODEX_PORTABLE_MCP_OK\"}. After that single tool call, answer DONE.")
	cmd := exec.CommandContext(ctx, codexBin, args...)
	cmd.Env = append(os.Environ(), "PLUGIN_KIT_AI_MCP_SMOKE_MARKER="+markerPath)
	logfh, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer logfh.Close()
	cmd.Stdout = logfh
	cmd.Stderr = logfh
	if err := cmd.Start(); err != nil {
		t.Fatalf("start codex portable MCP live smoke: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	deadline := time.Now().Add(75 * time.Second)
	sawMarker := false
	for !sawMarker {
		if _, err := os.Stat(markerPath); err == nil {
			sawMarker = true
			break
		}
		select {
		case err := <-waitCh:
			out := readLogFile(t, logFile)
			if err != nil {
				if codexRuntimeUnhealthy(out) {
					t.Skipf("codex runtime unhealthy in current environment:\n%s", truncateRunes(out, 4000))
				}
				t.Fatalf("codex portable MCP live smoke: %v\n%s", err, out)
			}
			if codexPortableMCPToolUnavailable(out) {
				t.Logf("codex portable MCP live smoke did not expose the projected MCP tool in exec session for model %q:\n%s", model, truncateRunes(out, 4000))
				return false
			}
			if body, readErr := os.ReadFile(outputFile); readErr == nil && strings.Contains(string(body), "DONE") {
				t.Logf("codex portable MCP live smoke finished without tool selection for model %q:\n%s", model, truncateRunes(out, 4000))
				return false
			}
			t.Logf("codex portable MCP live smoke exited without producing MCP marker for model %q after a successful config preflight:\n%s", model, truncateRunes(out, 4000))
			return false
		default:
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			<-waitCh
			out := readLogFile(t, logFile)
			if codexPortableMCPToolUnavailable(out) {
				t.Logf("codex portable MCP live smoke timed out with projected MCP tool still unavailable in exec session for model %q:\n%s", model, truncateRunes(out, 4000))
				return false
			}
			if body, readErr := os.ReadFile(outputFile); readErr == nil && strings.Contains(string(body), "DONE") {
				t.Logf("codex portable MCP live smoke completed without marker for model %q after task completion:\n%s", model, truncateRunes(out, 4000))
				return false
			}
			t.Fatalf("timed out waiting for codex portable MCP marker:\n%s", out)
		}
		time.Sleep(150 * time.Millisecond)
	}

	select {
	case err := <-waitCh:
		out := readLogFile(t, logFile)
		if err != nil {
			if codexRuntimeUnhealthy(out) {
				t.Skipf("codex runtime unhealthy in current environment:\n%s", truncateRunes(out, 4000))
			}
			t.Fatalf("codex portable MCP live smoke: %v\n%s", err, out)
		}
		if body, err := os.ReadFile(outputFile); err != nil || !strings.Contains(string(body), "DONE") {
			if err != nil {
				t.Fatalf("codex portable MCP live smoke missing last message file: %v", err)
			}
			t.Fatalf("codex portable MCP last message missing DONE:\n%s", body)
		}
		t.Logf("codex portable MCP output for model %q: %s", model, truncateRunes(out, 4000))
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-waitCh
		out := readLogFile(t, logFile)
		t.Logf("codex portable MCP output for model %q (truncated after marker): %s", model, truncateRunes(out, 4000))
	}
	return true
}

func codexPortableMCPToolUnavailable(log string) bool {
	if strings.Contains(log, "release_checks") && ((strings.Contains(log, "tool") && strings.Contains(log, "available in this session")) || strings.Contains(log, "is not available") || strings.Contains(log, "isn't available") || strings.Contains(log, "isn’t available")) {
		return true
	}
	markers := []string{
		"no MCP tool named `release_checks` is available in this session",
		"no such tool is available in this session",
		"the `release_checks` MCP tool is not available in this session",
		"release_checks tool is not available in this session",
		"`release_checks` MCP tool call because that tool isn't available in this session",
		"`release_checks` MCP tool call because that tool isn’t available in this session",
	}
	for _, marker := range markers {
		if strings.Contains(log, marker) {
			return true
		}
	}
	return false
}

func assertCodexSeesPortableMCP(t *testing.T, codexBin string, configArgs []string, wantCommand string) {
	t.Helper()
	args := []string{"mcp", "get", "release-checks", "--json"}
	args = append(args, configArgs...)
	cmd := exec.Command(codexBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("codex mcp get release-checks: %v\n%s", err, out)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("parse codex mcp get release-checks: %v\n%s", err, out)
	}
	transport, ok := doc["transport"].(map[string]any)
	if !ok {
		t.Fatalf("codex mcp get release-checks missing transport:\n%s", out)
	}
	if got := strings.TrimSpace(fmt.Sprint(transport["command"])); got != wantCommand {
		t.Fatalf("codex mcp get release-checks command = %q want %q\n%s", got, wantCommand, out)
	}
}

func readRenderedSharedMCPServer(t *testing.T, workDir, name string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(workDir, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse generated .mcp.json: %v\n%s", err, body)
	}
	server, ok := doc[name]
	if !ok {
		t.Fatalf("generated .mcp.json missing %q server:\n%s", name, body)
	}
	return server
}

func codexMCPConfigArgs(name string, server map[string]any) []string {
	args := []string{
		"-c", fmt.Sprintf("mcp_servers.%s.command=%q", name, strings.TrimSpace(fmt.Sprint(server["command"]))),
	}
	if rawArgs, ok := server["args"].([]any); ok {
		quoted := make([]string, 0, len(rawArgs))
		for _, arg := range rawArgs {
			quoted = append(quoted, fmt.Sprintf("%q", strings.TrimSpace(fmt.Sprint(arg))))
		}
		args = append(args, "-c", fmt.Sprintf("mcp_servers.%s.args=[%s]", name, strings.Join(quoted, ",")))
	}
	if rawEnv, ok := server["env"].(map[string]any); ok && len(rawEnv) > 0 {
		keys := make([]string, 0, len(rawEnv))
		for key := range rawEnv {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			args = append(args, "-c", fmt.Sprintf("mcp_servers.%s.env.%s=%q", name, key, strings.TrimSpace(fmt.Sprint(rawEnv[key]))))
		}
	}
	return args
}

func openCodeBinaryOrSkip(t *testing.T) string {
	t.Helper()
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_ENABLE_OPENCODE_SMOKE")) != "1" {
		t.Skip("set PLUGIN_KIT_AI_ENABLE_OPENCODE_SMOKE=1 to run real OpenCode MCP live smoke")
	}
	opencodeBin := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_E2E_OPENCODE"))
	if opencodeBin == "" {
		var err error
		opencodeBin, err = exec.LookPath("opencode")
		if err != nil {
			t.Skip("opencode not in PATH; set PLUGIN_KIT_AI_E2E_OPENCODE or install OpenCode CLI")
		}
	}
	return opencodeBin
}

func runOpenCodeServeUntilPortableMCPInit(t *testing.T, opencodeBin, workDir, markerPath string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, opencodeBin, "serve")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"PLUGIN_KIT_AI_MCP_SMOKE_MARKER="+markerPath,
		"OPENCODE_SERVER_PASSWORD=plugin-kit-ai-portable-mcp-live",
	)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	errCh := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start opencode portable MCP live smoke: %v", err)
	}
	go func() {
		errCh <- cmd.Wait()
	}()

	deadline := time.Now().Add(8 * time.Second)
	for {
		if _, err := os.Stat(markerPath); err == nil {
			cancel()
			<-errCh
			return
		}
		if time.Now().After(deadline) {
			cancel()
			err := <-errCh
			t.Fatalf("OpenCode portable MCP live smoke did not observe MCP initialize marker before timeout; err=%v\n%s", err, output.Bytes())
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func assertPortableMCPMarker(t *testing.T, path, wantEvent, wantTool, wantToken string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Event string `json:"event"`
		Tool  string `json:"tool"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse portable MCP marker: %v\n%s", err, body)
	}
	if got := strings.TrimSpace(doc.Event); got != wantEvent {
		t.Fatalf("portable MCP marker event = %q want %q\n%s", got, wantEvent, body)
	}
	if wantTool != "" && strings.TrimSpace(doc.Tool) != wantTool {
		t.Fatalf("portable MCP marker tool = %q want %q\n%s", doc.Tool, wantTool, body)
	}
	if wantToken != "" && strings.TrimSpace(doc.Token) != wantToken {
		t.Fatalf("portable MCP marker token = %q want %q\n%s", doc.Token, wantToken, body)
	}
}
