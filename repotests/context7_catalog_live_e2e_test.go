package pluginkitairepo_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const context7CatalogLiveEnvVar = "PLUGIN_KIT_AI_RUN_CONTEXT7_LIVE"
const context7CatalogDirEnvVar = "PLUGIN_KIT_AI_E2E_CONTEXT7_DIR"

func TestContext7CatalogLiveAcrossInstalledAgents(t *testing.T) {
	if strings.TrimSpace(os.Getenv(context7CatalogLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real context7 catalog live smoke", context7CatalogLiveEnvVar)
	}

	pluginDir := resolveContext7CatalogPluginDir(t)
	pluginKitAIBin := buildPluginKitAI(t)
	assertContext7CatalogRenderedAndValid(t, pluginKitAIBin, pluginDir)

	t.Run("Claude_plugin_dir_and_tool_call", func(t *testing.T) {
		claudeBin := installedClaudeBinaryOrSkip(t)

		validateCmd := exec.Command(claudeBin, "plugins", "validate", pluginDir)
		validateCmd.Dir = pluginDir
		validateOut, err := validateCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins validate context7: %v\n%s", err, validateOut)
		}

		listCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "plugins", "list")
		listCmd.Dir = pluginDir
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins list with --plugin-dir: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), "context7@inline") {
			t.Fatalf("claude plugins list missing inline context7 plugin:\n%s", listOut)
		}

		mcpListCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "mcp", "list")
		mcpListCmd.Dir = pluginDir
		mcpListOut, err := mcpListCmd.CombinedOutput()
		if err != nil {
			if claudeEnvironmentIssue(string(mcpListOut)) {
				t.Skipf("claude environment is not ready for context7 live smoke:\n%s", truncateRunes(string(mcpListOut), 4000))
			}
			t.Fatalf("claude mcp list with --plugin-dir: %v\n%s", err, mcpListOut)
		}
		if !strings.Contains(string(mcpListOut), "plugin:context7:context7:") {
			t.Fatalf("claude mcp list missing plugin-projected context7 server:\n%s", mcpListOut)
		}

		configPath := writeClaudePortableMCPConfig(t, pluginDir)
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, claudeBin,
			"-p",
			"--model", *claudeModel,
			"--mcp-config", configPath,
			"--strict-mcp-config",
			"--dangerously-skip-permissions",
			"Use the MCP tool context7 exactly once to resolve the library ID for React. Then answer with only the resolved library ID.",
		)
		cmd.Dir = pluginDir
		out, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("claude context7 live prompt timed out:\n%s", out)
		}
		if err != nil {
			if claudeEnvironmentIssue(string(out)) {
				t.Skipf("claude environment is not ready for context7 live prompt:\n%s", truncateRunes(string(out), 4000))
			}
			t.Fatalf("claude context7 live prompt: %v\n%s", err, out)
		}
		assertContext7LibraryIDOutput(t, string(out))
	})

	t.Run("Codex_sidecar_add_get_list_and_exec", func(t *testing.T) {
		codexBin := installedCodexBinaryOrSkip(t)
		homeDir := filepath.Join(t.TempDir(), "codex-home")
		if err := os.MkdirAll(homeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		env := append(os.Environ(), "HOME="+homeDir)

		server := readRenderedSharedMCPServer(t, pluginDir, "context7")
		command := strings.TrimSpace(fmt.Sprint(server["command"]))
		if command == "" {
			t.Fatalf("generated context7 .mcp.json missing command:\n%v", server)
		}
		args := []string{"mcp", "add", "context7", "--", command}
		if rawArgs, ok := server["args"].([]any); ok {
			for _, arg := range rawArgs {
				args = append(args, strings.TrimSpace(fmt.Sprint(arg)))
			}
		}
		addCmd := exec.Command(codexBin, args...)
		addCmd.Env = env
		addOut, err := addCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp add context7: %v\n%s", err, addOut)
		}

		getCmd := exec.Command(codexBin, "mcp", "get", "context7", "--json")
		getCmd.Env = env
		getOut, err := getCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp get context7: %v\n%s", err, getOut)
		}
		if !strings.Contains(string(getOut), `"name": "context7"`) && !strings.Contains(string(getOut), `"name":"context7"`) {
			t.Fatalf("codex mcp get context7 missing server name:\n%s", getOut)
		}

		listCmd := exec.Command(codexBin, "mcp", "list", "--json")
		listCmd.Env = env
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp list: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), `"name": "context7"`) && !strings.Contains(string(listOut), `"name":"context7"`) {
			t.Fatalf("codex mcp list missing context7:\n%s", listOut)
		}

		models := context7CodexModels()
		for idx, model := range models {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			cmd := exec.CommandContext(ctx,
				codexBin,
				"exec",
				"--skip-git-repo-check",
				"--ephemeral",
				"-C", pluginDir,
				"-m", model,
				"--color", "never",
				"--dangerously-bypass-approvals-and-sandbox",
				"Do not inspect files or run shell commands. Use the MCP tool context7 exactly once to resolve the library ID for React. Then answer with only the resolved library ID.",
			)
			cmd.Env = env
			out, err := cmd.CombinedOutput()
			cancel()
			if err == nil {
				assertContext7LibraryIDOutput(t, string(out))
				return
			}
			text := string(out)
			if codexRuntimeUnhealthy(text) || codexPortableMCPToolUnavailable(text) {
				if idx == len(models)-1 {
					t.Skipf("codex runtime did not reach a stable live MCP exec session after trying models %v:\n%s", models, truncateRunes(text, 4000))
				}
				t.Logf("codex live prompt with model %q did not reach a stable MCP session; retrying fallback model:\n%s", model, truncateRunes(text, 2000))
				continue
			}
			t.Fatalf("codex context7 live exec with model %q: %v\n%s", model, err, text)
		}
	})

	t.Run("Gemini_extension_link_list_and_prompt", func(t *testing.T) {
		geminiBin := geminiBinaryOrSkip(t)
		homeDir := filepath.Join(t.TempDir(), "gemini-home")
		if err := os.MkdirAll(homeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		seedGeminiHome(t, homeDir, pluginDir)

		validateOutput := runGeminiCommand(t, geminiBin, homeDir, pluginDir, "extensions", "validate", pluginDir)
		if !strings.Contains(validateOutput, "successfully validated") {
			t.Fatalf("gemini validate output missing success marker:\n%s", validateOutput)
		}
		linkOutput := runGeminiLink(t, geminiBin, homeDir, pluginDir)
		if !strings.Contains(linkOutput, `linked successfully and enabled`) {
			t.Fatalf("gemini link output missing success marker:\n%s", linkOutput)
		}
		listOutput := runGeminiCommand(t, geminiBin, homeDir, pluginDir, "extensions", "list")
		if !strings.Contains(listOutput, "context7") || !strings.Contains(listOutput, "MCP servers:") {
			t.Fatalf("gemini extensions list missing context7 MCP projection:\n%s", listOutput)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx,
			geminiBin,
			"-p", "Use the MCP tool context7 exactly once to resolve the library ID for React. Then answer with only the resolved library ID.",
			"-e", "context7",
			"--allowed-mcp-server-names", "context7",
			"--approval-mode", "yolo",
		)
		cmd.Dir = pluginDir
		cmd.Env = geminiCLIEnv(homeDir)
		out, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("gemini context7 live prompt timed out:\n%s", out)
		}
		if err != nil {
			if geminiEnvironmentIssue(string(out)) {
				t.Skipf("gemini environment is not ready for context7 live prompt; %s\n%s", geminiAuthRecoveryHint(string(out)), truncateRunes(string(out), 4000))
			}
			t.Fatalf("gemini context7 live prompt: %v\n%s", err, out)
		}
		assertContext7LibraryIDOutput(t, string(out))
	})

	t.Run("Cursor_isolated_config_list_enable_and_list_tools", func(t *testing.T) {
		cursorBin := installedCursorBinaryOrSkip(t)
		cursorHome := newCursorIsolatedMCPHome(t, pluginDir)

		listArgs := cursorCLIArgs(cursorBin, "mcp", "list")
		listCmd := exec.Command(cursorBin, listArgs...)
		listCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp list with isolated config: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), "context7") {
			t.Fatalf("cursor mcp list missing context7 server from isolated config:\n%s", listOut)
		}
		if !strings.Contains(strings.ToLower(string(listOut)), "needs approval") {
			t.Fatalf("cursor mcp list should report context7 as pending approval before enable:\n%s", listOut)
		}

		enableArgs := cursorCLIArgs(cursorBin, "mcp", "enable", "context7")
		enableCmd := exec.Command(cursorBin, enableArgs...)
		enableCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		enableOut, err := enableCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp enable context7 with isolated config: %v\n%s", err, enableOut)
		}

		listCmd = exec.Command(cursorBin, listArgs...)
		listCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		listOut, err = listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp list after enabling context7: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), "context7") || !strings.Contains(strings.ToLower(string(listOut)), "ready") {
			t.Fatalf("cursor mcp list missing ready context7 server after isolated enable:\n%s", listOut)
		}

		toolsArgs := cursorCLIArgs(cursorBin, "mcp", "list-tools", "context7")
		toolsCmd := exec.Command(cursorBin, toolsArgs...)
		toolsCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		toolsOut, err := toolsCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp list-tools context7 with isolated config: %v\n%s", err, toolsOut)
		}
		if !strings.Contains(string(toolsOut), "resolve-library-id") || !strings.Contains(string(toolsOut), "query-docs") {
			t.Fatalf("cursor mcp list-tools missing context7 tools:\n%s", toolsOut)
		}
	})

	t.Run("OpenCode_workspace_serve_startup", func(t *testing.T) {
		opencodeBin := installedOpenCodeBinaryOrSkip(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, opencodeBin, "serve")
		cmd.Dir = pluginDir
		cmd.Env = append(os.Environ(), "OPENCODE_SERVER_PASSWORD=plugin-kit-ai-context7-live")
		outPath := filepath.Join(t.TempDir(), "opencode-context7.log")
		logFile, err := os.Create(outPath)
		if err != nil {
			t.Fatal(err)
		}
		defer logFile.Close()
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			t.Fatalf("start opencode serve in context7 workspace: %v", err)
		}

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- cmd.Wait()
		}()

		select {
		case err := <-waitCh:
			body, _ := os.ReadFile(outPath)
			t.Fatalf("opencode serve exited before startup stabilization: %v\n%s", err, body)
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-waitCh
		}
	})
}

func resolveContext7CatalogPluginDir(t *testing.T) string {
	t.Helper()
	if dir := strings.TrimSpace(os.Getenv(context7CatalogDirEnvVar)); dir != "" {
		if fileExists(filepath.Join(dir, "plugin", "plugin.yaml")) {
			return dir
		}
		t.Fatalf("%s=%q does not point to a context7 plugin with plugin/plugin.yaml", context7CatalogDirEnvVar, dir)
	}
	root := RepoRoot(t)
	candidate := filepath.Join(filepath.Dir(root), "universal-plugins-for-ai-agents", "plugins", "context7")
	if fileExists(filepath.Join(candidate, "plugin", "plugin.yaml")) {
		return candidate
	}
	t.Skipf("context7 catalog plugin not found; set %s=/abs/path/to/universal-plugins-for-ai-agents/plugins/context7", context7CatalogDirEnvVar)
	return ""
}

func assertContext7CatalogRenderedAndValid(t *testing.T, pluginKitAIBin, pluginDir string) {
	t.Helper()
	root := RepoRoot(t)
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir, "--check"))
	for _, platform := range []string{"claude", "codex-package", "gemini", "opencode", "cursor"} {
		runCmd(t, root, exec.Command(pluginKitAIBin, "validate", pluginDir, "--platform", platform, "--strict"))
	}
}

func installedClaudeBinaryOrSkip(t *testing.T) string {
	t.Helper()
	if path := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_E2E_CLAUDE")); path != "" {
		if out, err := exec.Command(path, "--version").CombinedOutput(); err == nil {
			return path
		} else {
			t.Skipf("configured Claude binary is not runnable: %v\n%s", err, out)
		}
	}
	if claudeBin, err := exec.LookPath("claude"); err == nil {
		if out, err := exec.Command(claudeBin, "--version").CombinedOutput(); err == nil {
			return claudeBin
		} else {
			t.Skipf("claude binary is not runnable in this environment: %v\n%s", err, out)
		}
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, ".claude", "local", "claude"),
		"/opt/homebrew/bin/claude",
		"/usr/local/bin/claude",
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" || !fileExists(candidate) {
			continue
		}
		if _, err := exec.Command(candidate, "--version").CombinedOutput(); err == nil {
			return candidate
		}
	}
	t.Skip("claude not installed or not reachable in PATH/common fallback locations")
	return ""
}

func installedCodexBinaryOrSkip(t *testing.T) string {
	t.Helper()
	codexBin := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_E2E_CODEX"))
	if codexBin == "" {
		var err error
		codexBin, err = exec.LookPath("codex")
		if err != nil {
			t.Skip("codex not installed")
		}
	}
	if out, err := exec.Command(codexBin, "login", "status").CombinedOutput(); err != nil {
		t.Skipf("codex installed but not ready for live smoke: %v\n%s", err, out)
	}
	return codexBin
}

func installedCursorBinaryOrSkip(t *testing.T) string {
	t.Helper()
	cursorBin := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_E2E_CURSOR"))
	if cursorBin == "" {
		home, _ := os.UserHomeDir()
		var candidates []string
		if bin, err := exec.LookPath("cursor-agent"); err == nil {
			candidates = append(candidates, bin)
		}
		for _, candidate := range []string{
			filepath.Join(home, ".local", "bin", "cursor-agent"),
			"/Applications/Cursor.app/Contents/Resources/app/bin/cursor",
		} {
			if strings.TrimSpace(candidate) != "" && fileExists(candidate) {
				candidates = append(candidates, candidate)
			}
		}
		if bin, err := exec.LookPath("cursor"); err == nil {
			candidates = append(candidates, bin)
		}
		for _, candidate := range candidates {
			if out, err := cursorCommandWithTimeout(candidate, 12*time.Second, cursorCLIArgs(candidate, "status")...); err == nil {
				return candidate
			} else if cursorEnvironmentIssue(string(out)) {
				t.Skipf("cursor installed but not ready for live smoke:\n%s", truncateRunes(string(out), 4000))
			}
		}
		t.Skip("cursor not installed")
	}
	if out, err := cursorCommandWithTimeout(cursorBin, 12*time.Second, cursorCLIArgs(cursorBin, "status")...); err != nil {
		if cursorEnvironmentIssue(string(out)) {
			t.Skipf("cursor installed but not ready for live smoke:\n%s", truncateRunes(string(out), 4000))
		}
		t.Skipf("cursor status failed:\n%s", out)
	}
	return cursorBin
}

func cursorCommandWithTimeout(cursorBin string, timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, cursorBin, args...).CombinedOutput()
}

func cursorCLIArgs(cursorBin string, args ...string) []string {
	if filepath.Base(cursorBin) == "cursor" {
		return append([]string{"agent"}, args...)
	}
	return args
}

func newCursorIsolatedMCPHome(t *testing.T, pluginDir string) string {
	t.Helper()
	homeDir := filepath.Join(t.TempDir(), "cursor-home")
	configDir := filepath.Join(homeDir, ".cursor")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(pluginDir, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("read generated Cursor MCP config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "mcp.json"), body, 0o644); err != nil {
		t.Fatalf("write isolated Cursor MCP config: %v", err)
	}
	return homeDir
}

func installedOpenCodeBinaryOrSkip(t *testing.T) string {
	t.Helper()
	opencodeBin := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_E2E_OPENCODE"))
	if opencodeBin == "" {
		var err error
		opencodeBin, err = exec.LookPath("opencode")
		if err != nil {
			t.Skip("opencode not installed")
		}
	}
	if out, err := exec.Command(opencodeBin, "--version").CombinedOutput(); err != nil {
		t.Skipf("opencode binary is not runnable in this environment: %v\n%s", err, out)
	}
	return opencodeBin
}

func context7CodexModels() []string {
	models := []string{strings.TrimSpace(*codexModel)}
	if models[0] == "" {
		models[0] = "gpt-5.4-mini"
	}
	if fallback := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_CONTEXT7_CODEX_FALLBACK_MODEL")); fallback != "" && fallback != models[0] {
		models = append(models, fallback)
	} else if models[0] != "gpt-5.4" {
		models = append(models, "gpt-5.4")
	}
	return models
}

func assertContext7LibraryIDOutput(t *testing.T, output string) {
	t.Helper()
	text := strings.TrimSpace(output)
	allowed := []string{
		"/reactjs/react.dev",
		"/facebook/react",
		"/websites/react_dev",
	}
	for _, want := range allowed {
		if strings.Contains(text, want) {
			return
		}
	}
	t.Fatalf("context7 live output missing expected React library id:\n%s", output)
}
