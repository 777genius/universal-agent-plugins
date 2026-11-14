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

const chromeDevtoolsCatalogLiveEnvVar = "PLUGIN_KIT_AI_RUN_CHROME_DEVTOOLS_LIVE"
const chromeDevtoolsCatalogDirEnvVar = "PLUGIN_KIT_AI_E2E_CHROME_DEVTOOLS_DIR"

func TestChromeDevtoolsCatalogLiveAcrossInstalledAgents(t *testing.T) {
	if strings.TrimSpace(os.Getenv(chromeDevtoolsCatalogLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real chrome-devtools catalog live smoke", chromeDevtoolsCatalogLiveEnvVar)
	}

	pluginDir := resolveChromeDevtoolsCatalogPluginDir(t)
	pluginKitAIBin := buildPluginKitAI(t)
	assertChromeDevtoolsCatalogRenderedAndValid(t, pluginKitAIBin, pluginDir)

	t.Run("Claude_plugin_dir_and_browser_prompt", func(t *testing.T) {
		claudeBin := installedClaudeBinaryOrSkip(t)

		validateCmd := exec.Command(claudeBin, "plugins", "validate", pluginDir)
		validateCmd.Dir = pluginDir
		validateOut, err := validateCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins validate chrome-devtools: %v\n%s", err, validateOut)
		}

		listCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "plugins", "list")
		listCmd.Dir = pluginDir
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins list with --plugin-dir: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), "chrome-devtools@inline") {
			t.Fatalf("claude plugins list missing inline chrome-devtools plugin:\n%s", listOut)
		}

		mcpListCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "mcp", "list")
		mcpListCmd.Dir = pluginDir
		mcpListOut, err := mcpListCmd.CombinedOutput()
		if err != nil {
			if chromeDevtoolsEnvironmentIssue(string(mcpListOut)) || claudeEnvironmentIssue(string(mcpListOut)) {
				t.Skipf("claude environment is not ready for chrome-devtools live smoke:\n%s", truncateRunes(string(mcpListOut), 4000))
			}
			t.Fatalf("claude mcp list with --plugin-dir: %v\n%s", err, mcpListOut)
		}
		if !strings.Contains(string(mcpListOut), "plugin:chrome-devtools:chrome-devtools:") {
			t.Fatalf("claude mcp list missing plugin-projected chrome-devtools server:\n%s", mcpListOut)
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
			chromeDevtoolsLivePrompt(),
		)
		cmd.Dir = pluginDir
		out, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("claude chrome-devtools live prompt timed out:\n%s", out)
		}
		if err != nil {
			if chromeDevtoolsEnvironmentIssue(string(out)) || claudeEnvironmentIssue(string(out)) {
				t.Skipf("claude environment is not ready for chrome-devtools live prompt:\n%s", truncateRunes(string(out), 4000))
			}
			t.Fatalf("claude chrome-devtools live prompt: %v\n%s", err, out)
		}
		assertChromeDevtoolsExampleDomainOutput(t, string(out))
	})

	t.Run("Codex_sidecar_add_get_list_and_exec", func(t *testing.T) {
		codexBin := installedCodexBinaryOrSkip(t)
		homeDir := filepath.Join(t.TempDir(), "codex-home")
		if err := os.MkdirAll(homeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		env := append(os.Environ(), "HOME="+homeDir)

		server := readRenderedSharedMCPServer(t, pluginDir, "chrome-devtools")
		command := strings.TrimSpace(fmt.Sprint(server["command"]))
		if command == "" {
			t.Fatalf("generated chrome-devtools .mcp.json missing command:\n%v", server)
		}
		args := []string{"mcp", "add", "chrome-devtools", "--", command}
		if rawArgs, ok := server["args"].([]any); ok {
			for _, arg := range rawArgs {
				args = append(args, strings.TrimSpace(fmt.Sprint(arg)))
			}
		}
		addCmd := exec.Command(codexBin, args...)
		addCmd.Env = env
		addOut, err := addCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp add chrome-devtools: %v\n%s", err, addOut)
		}

		getCmd := exec.Command(codexBin, "mcp", "get", "chrome-devtools", "--json")
		getCmd.Env = env
		getOut, err := getCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp get chrome-devtools: %v\n%s", err, getOut)
		}
		if !strings.Contains(string(getOut), `"name": "chrome-devtools"`) && !strings.Contains(string(getOut), `"name":"chrome-devtools"`) {
			t.Fatalf("codex mcp get chrome-devtools missing server name:\n%s", getOut)
		}

		listCmd := exec.Command(codexBin, "mcp", "list", "--json")
		listCmd.Env = env
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp list: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), `"name": "chrome-devtools"`) && !strings.Contains(string(listOut), `"name":"chrome-devtools"`) {
			t.Fatalf("codex mcp list missing chrome-devtools:\n%s", listOut)
		}

		models := chromeDevtoolsCodexModels()
		for idx, model := range models {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			cmd := exec.CommandContext(ctx,
				codexBin,
				"exec",
				"--skip-git-repo-check",
				"--ephemeral",
				"-C", pluginDir,
				"-m", model,
				"--color", "never",
				"--dangerously-bypass-approvals-and-sandbox",
				chromeDevtoolsLivePrompt(),
			)
			cmd.Env = env
			out, err := cmd.CombinedOutput()
			cancel()
			if err == nil {
				assertChromeDevtoolsExampleDomainOutput(t, string(out))
				return
			}
			text := string(out)
			if codexRuntimeUnhealthy(text) || chromeDevtoolsEnvironmentIssue(text) || codexPortableMCPToolUnavailable(text) {
				if idx == len(models)-1 {
					t.Skipf("codex runtime did not reach a stable chrome-devtools live exec session after trying models %v:\n%s", models, truncateRunes(text, 4000))
				}
				t.Logf("codex live prompt with model %q did not reach a stable chrome-devtools session; retrying fallback model:\n%s", model, truncateRunes(text, 2000))
				continue
			}
			t.Fatalf("codex chrome-devtools live exec with model %q: %v\n%s", model, err, text)
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
		if !strings.Contains(listOutput, "chrome-devtools") || !strings.Contains(listOutput, "MCP servers:") {
			t.Fatalf("gemini extensions list missing chrome-devtools MCP projection:\n%s", listOutput)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx,
			geminiBin,
			"-p", chromeDevtoolsLivePrompt(),
			"-e", "chrome-devtools",
			"--allowed-mcp-server-names", "chrome-devtools",
			"--approval-mode", "yolo",
		)
		cmd.Dir = pluginDir
		cmd.Env = geminiCLIEnv(homeDir)
		out, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("gemini chrome-devtools live prompt timed out:\n%s", out)
		}
		if err != nil {
			if chromeDevtoolsEnvironmentIssue(string(out)) || geminiEnvironmentIssue(string(out)) {
				t.Skipf("gemini environment is not ready for chrome-devtools live prompt; %s\n%s", geminiAuthRecoveryHint(string(out)), truncateRunes(string(out), 4000))
			}
			t.Fatalf("gemini chrome-devtools live prompt: %v\n%s", err, out)
		}
		assertChromeDevtoolsExampleDomainOutput(t, string(out))
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
		if !strings.Contains(string(listOut), "chrome-devtools") {
			t.Fatalf("cursor mcp list missing chrome-devtools server from isolated config:\n%s", listOut)
		}
		if !strings.Contains(strings.ToLower(string(listOut)), "needs approval") {
			t.Fatalf("cursor mcp list should report chrome-devtools as pending approval before enable:\n%s", listOut)
		}

		enableArgs := cursorCLIArgs(cursorBin, "mcp", "enable", "chrome-devtools")
		enableCmd := exec.Command(cursorBin, enableArgs...)
		enableCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		enableOut, err := enableCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp enable chrome-devtools with isolated config: %v\n%s", err, enableOut)
		}

		listCmd = exec.Command(cursorBin, listArgs...)
		listCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		listOut, err = listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp list after enabling chrome-devtools: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), "chrome-devtools") || !strings.Contains(strings.ToLower(string(listOut)), "ready") {
			t.Fatalf("cursor mcp list missing ready chrome-devtools server after isolated enable:\n%s", listOut)
		}

		toolsArgs := cursorCLIArgs(cursorBin, "mcp", "list-tools", "chrome-devtools")
		toolsCmd := exec.Command(cursorBin, toolsArgs...)
		toolsCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		toolsOut, err := toolsCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp list-tools chrome-devtools with isolated config: %v\n%s", err, toolsOut)
		}
		if !strings.Contains(string(toolsOut), "new_page") && !strings.Contains(string(toolsOut), "navigate_page") && !strings.Contains(string(toolsOut), "take_snapshot") {
			t.Fatalf("cursor mcp list-tools missing expected chrome-devtools tools:\n%s", toolsOut)
		}
	})

	t.Run("OpenCode_workspace_serve_startup", func(t *testing.T) {
		opencodeBin := installedOpenCodeBinaryOrSkip(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, opencodeBin, "serve")
		cmd.Dir = pluginDir
		cmd.Env = append(os.Environ(), "OPENCODE_SERVER_PASSWORD=plugin-kit-ai-chrome-devtools-live")
		outPath := filepath.Join(t.TempDir(), "opencode-chrome-devtools.log")
		logFile, err := os.Create(outPath)
		if err != nil {
			t.Fatal(err)
		}
		defer logFile.Close()
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			t.Fatalf("start opencode serve in chrome-devtools workspace: %v", err)
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

func resolveChromeDevtoolsCatalogPluginDir(t *testing.T) string {
	t.Helper()
	if dir := strings.TrimSpace(os.Getenv(chromeDevtoolsCatalogDirEnvVar)); dir != "" {
		if fileExists(filepath.Join(dir, "plugin", "plugin.yaml")) {
			return dir
		}
		t.Fatalf("%s=%q does not point to a chrome-devtools plugin with plugin/plugin.yaml", chromeDevtoolsCatalogDirEnvVar, dir)
	}
	root := RepoRoot(t)
	candidate := filepath.Join(filepath.Dir(root), "universal-plugins-for-ai-agents", "plugins", "chrome-devtools")
	if fileExists(filepath.Join(candidate, "plugin", "plugin.yaml")) {
		return candidate
	}
	t.Skipf("chrome-devtools catalog plugin not found; set %s=/abs/path/to/universal-plugins-for-ai-agents/plugins/chrome-devtools", chromeDevtoolsCatalogDirEnvVar)
	return ""
}

func assertChromeDevtoolsCatalogRenderedAndValid(t *testing.T, pluginKitAIBin, pluginDir string) {
	t.Helper()
	root := RepoRoot(t)
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir, "--check"))
	for _, platform := range []string{"claude", "codex-package", "gemini", "opencode", "cursor"} {
		runCmd(t, root, exec.Command(pluginKitAIBin, "validate", pluginDir, "--platform", platform, "--strict"))
	}
}

func chromeDevtoolsLivePrompt() string {
	return "Do not inspect local files or run shell commands. Use the MCP tool chrome-devtools to open https://example.com, then answer with only the page title."
}

func chromeDevtoolsCodexModels() []string {
	models := []string{strings.TrimSpace(*codexModel)}
	if models[0] == "" {
		models[0] = "gpt-5.4-mini"
	}
	if fallback := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_CHROME_DEVTOOLS_CODEX_FALLBACK_MODEL")); fallback != "" && fallback != models[0] {
		models = append(models, fallback)
	} else if models[0] != "gpt-5.4" {
		models = append(models, "gpt-5.4")
	}
	return models
}

func assertChromeDevtoolsExampleDomainOutput(t *testing.T, output string) {
	t.Helper()
	text := strings.TrimSpace(output)
	if strings.Contains(text, "Example Domain") {
		return
	}
	t.Fatalf("chrome-devtools live output missing expected Example Domain title:\n%s", output)
}

func chromeDevtoolsEnvironmentIssue(output string) bool {
	lower := strings.ToLower(output)
	markers := []string{
		"could not find chrome",
		"failed to launch the browser process",
		"browser was not found",
		"chrome executable",
		"chrome stable or newer",
		"unable to locate chrome",
		"failed to connect to browser",
		"could not connect to the browser",
		"requires the remote debugging server",
		"failed to create browser context",
		"failed to launch chrome",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
