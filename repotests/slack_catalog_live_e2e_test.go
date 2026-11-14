package pluginkitairepo_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const slackCatalogLiveEnvVar = "PLUGIN_KIT_AI_RUN_SLACK_LIVE"
const slackCatalogDirEnvVar = "PLUGIN_KIT_AI_E2E_SLACK_DIR"
const slackClaudeClientID = "1601185624273.8899143856786"
const slackClaudeCallbackPort = "3118"
const slackCursorClientID = "3660753192626.8903469228982"

func TestSlackCatalogLiveAcrossSupportedAgents(t *testing.T) {
	if strings.TrimSpace(os.Getenv(slackCatalogLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real slack catalog live smoke", slackCatalogLiveEnvVar)
	}

	pluginDir := resolveSlackCatalogPluginDir(t)
	pluginKitAIBin := buildPluginKitAI(t)
	assertSlackCatalogRenderedAndValid(t, pluginKitAIBin, pluginDir)

	t.Run("Claude_plugin_dir_list_and_mcp_status", func(t *testing.T) {
		claudeBin := installedClaudeBinaryOrSkip(t)

		validateCmd := exec.Command(claudeBin, "plugins", "validate", pluginDir)
		validateCmd.Dir = pluginDir
		validateOut, err := validateCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins validate slack: %v\n%s", err, validateOut)
		}

		listCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "plugins", "list")
		listCmd.Dir = pluginDir
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins list with --plugin-dir: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), "slack@inline") {
			t.Fatalf("claude plugins list missing inline slack plugin:\n%s", listOut)
		}

		mcpListCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "mcp", "list")
		mcpListCmd.Dir = pluginDir
		mcpListOut, err := mcpListCmd.CombinedOutput()
		if err != nil {
			if claudeEnvironmentIssue(string(mcpListOut)) {
				t.Skipf("claude environment is not ready for slack live smoke:\n%s", truncateRunes(string(mcpListOut), 4000))
			}
			t.Fatalf("claude mcp list with --plugin-dir: %v\n%s", err, mcpListOut)
		}
		assertClaudeRemoteCatalogStatus(t, string(mcpListOut), "plugin:slack:slack:", "mcp.slack.com")
	})

	t.Run("Cursor_isolated_config_shows_auth_boundary", func(t *testing.T) {
		cursorBin := installedCursorBinaryOrSkip(t)
		cursorHome := newCursorIsolatedMCPHome(t, pluginDir)

		listArgs := cursorCLIArgs(cursorBin, "mcp", "list")
		listCmd := exec.Command(cursorBin, listArgs...)
		listCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp list with isolated config: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), "slack") || !strings.Contains(strings.ToLower(string(listOut)), "needs approval") {
			t.Fatalf("cursor mcp list missing slack approval state:\n%s", listOut)
		}

		enableArgs := cursorCLIArgs(cursorBin, "mcp", "enable", "slack")
		enableCmd := exec.Command(cursorBin, enableArgs...)
		enableCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		enableOut, err := enableCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp enable slack with isolated config: %v\n%s", err, enableOut)
		}

		listCmd = exec.Command(cursorBin, listArgs...)
		listCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		listOut, err = listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp list after enabling slack: %v\n%s", err, listOut)
		}
		assertSlackCursorCatalogState(t, string(listOut))
	})
}

func resolveSlackCatalogPluginDir(t *testing.T) string {
	t.Helper()
	if dir := strings.TrimSpace(os.Getenv(slackCatalogDirEnvVar)); dir != "" {
		if fileExists(filepath.Join(dir, "plugin", "plugin.yaml")) {
			return dir
		}
		t.Fatalf("%s=%q does not point to a slack plugin with plugin/plugin.yaml", slackCatalogDirEnvVar, dir)
	}
	root := RepoRoot(t)
	candidate := filepath.Join(filepath.Dir(root), "universal-plugins-for-ai-agents", "plugins", "slack")
	if fileExists(filepath.Join(candidate, "plugin", "plugin.yaml")) {
		return candidate
	}
	t.Skipf("slack catalog plugin not found; set %s=/abs/path/to/universal-plugins-for-ai-agents/plugins/slack", slackCatalogDirEnvVar)
	return ""
}

func assertSlackCatalogRenderedAndValid(t *testing.T, pluginKitAIBin, pluginDir string) {
	t.Helper()
	root := RepoRoot(t)
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir, "--check"))
	for _, platform := range []string{"claude", "cursor"} {
		runCmd(t, root, exec.Command(pluginKitAIBin, "validate", pluginDir, "--platform", platform, "--strict"))
	}

	if fileExists(filepath.Join(pluginDir, ".codex-plugin", "plugin.json")) {
		t.Fatalf("slack plugin should not generate a codex-package bundle")
	}
	if fileExists(filepath.Join(pluginDir, "gemini-extension.json")) {
		t.Fatalf("slack plugin should not generate a gemini extension bundle")
	}
	if fileExists(filepath.Join(pluginDir, "opencode.json")) {
		t.Fatalf("slack plugin should not generate an opencode bundle")
	}

	server := readRenderedSharedMCPServer(t, pluginDir, "slack")
	oauth, ok := server["oauth"].(map[string]any)
	if !ok {
		t.Fatalf("generated slack .mcp.json missing claude oauth block:\n%v", server)
	}
	if got := anyString(oauth["clientId"]); got != slackClaudeClientID {
		t.Fatalf("generated slack .mcp.json oauth.clientId = %q want %q:\n%v", got, slackClaudeClientID, server)
	}
	if got := anyString(oauth["callbackPort"]); got != slackClaudeCallbackPort {
		t.Fatalf("generated slack .mcp.json oauth.callbackPort = %q want %q:\n%v", got, slackClaudeCallbackPort, server)
	}
	if got := anyString(server["type"]); got != "http" {
		t.Fatalf("generated slack .mcp.json type = %q want http:\n%v", got, server)
	}
	if got := anyString(server["url"]); got != "https://mcp.slack.com/mcp" {
		t.Fatalf("generated slack .mcp.json url = %q want https://mcp.slack.com/mcp:\n%v", got, server)
	}

	cursorBody, err := os.ReadFile(filepath.Join(pluginDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read generated cursor slack config: %v", err)
	}
	if !strings.Contains(string(cursorBody), `"CLIENT_ID": "`+slackCursorClientID+`"`) {
		t.Fatalf("generated .mcp.json missing exact Slack CLIENT_ID auth block:\n%s", cursorBody)
	}
}

func assertSlackCursorCatalogState(t *testing.T, output string) {
	t.Helper()
	lower := strings.ToLower(output)
	if !strings.Contains(output, "slack") {
		t.Fatalf("cursor mcp list missing slack entry:\n%s", output)
	}
	if strings.Contains(lower, "auth") || strings.Contains(lower, "connect") || strings.Contains(lower, "ready") {
		return
	}
	t.Fatalf("cursor mcp list missing acceptable slack hosted boundary state:\n%s", output)
}

func anyString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
