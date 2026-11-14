package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const cloudflareDocsCatalogLiveEnvVar = "PLUGIN_KIT_AI_RUN_CLOUDFLARE_DOCS_LIVE"
const cloudflareDocsCatalogDirEnvVar = "PLUGIN_KIT_AI_E2E_CLOUDFLARE_DOCS_DIR"
const cloudflareDocsCatalogServerName = "cloudflare-docs"
const cloudflareDocsCatalogServerURL = "https://docs.mcp.cloudflare.com/mcp"

func TestCloudflareDocsCatalogLiveAcrossInstalledAgents(t *testing.T) {
	if strings.TrimSpace(os.Getenv(cloudflareDocsCatalogLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real cloudflare-docs catalog live smoke", cloudflareDocsCatalogLiveEnvVar)
	}

	pluginDir := resolveCloudflareDocsCatalogPluginDir(t)
	pluginKitAIBin := buildPluginKitAI(t)
	assertCloudflareDocsCatalogRenderedAndValid(t, pluginKitAIBin, pluginDir)

	server := readRenderedSharedMCPServer(t, pluginDir, cloudflareDocsCatalogServerName)
	assertCloudflareDocsRenderedServer(t, server)

	t.Run("Claude_plugin_dir_list_and_mcp_status", func(t *testing.T) {
		claudeBin := installedClaudeBinaryOrSkip(t)
		validateCmd := exec.Command(claudeBin, "plugins", "validate", pluginDir)
		validateCmd.Dir = pluginDir
		validateOut, err := validateCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins validate cloudflare-docs: %v\n%s", err, validateOut)
		}
		listCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "plugins", "list")
		listCmd.Dir = pluginDir
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins list with --plugin-dir: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), "cloudflare-docs@inline") {
			t.Fatalf("claude plugins list missing inline cloudflare-docs plugin:\n%s", listOut)
		}
		mcpListCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "mcp", "list")
		mcpListCmd.Dir = pluginDir
		mcpListOut, err := mcpListCmd.CombinedOutput()
		if err != nil {
			if claudeEnvironmentIssue(string(mcpListOut)) {
				t.Skipf("claude environment is not ready for cloudflare-docs live smoke:\n%s", truncateRunes(string(mcpListOut), 4000))
			}
			t.Fatalf("claude mcp list with --plugin-dir: %v\n%s", err, mcpListOut)
		}
		assertClaudeRemoteCatalogStatus(t, string(mcpListOut), "plugin:cloudflare-docs:cloudflare-docs:", cloudflareDocsCatalogServerURL)
	})

	t.Run("Codex_get_and_list_rendered_remote_server", func(t *testing.T) {
		codexBin := installedCodexBinaryOrSkip(t)
		getCmd := exec.Command(codexBin, "mcp", "get", cloudflareDocsCatalogServerName, "--json", "-c", fmt.Sprintf("mcp_servers.%s.url=%q", cloudflareDocsCatalogServerName, cloudflareDocsCatalogServerURL))
		getOut, err := getCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp get cloudflare-docs: %v\n%s", err, getOut)
		}
		var getDoc struct {
			Name      string `json:"name"`
			Transport struct {
				Type string `json:"type"`
				URL  string `json:"url"`
			} `json:"transport"`
		}
		if err := json.Unmarshal(getOut, &getDoc); err != nil {
			t.Fatalf("parse codex mcp get cloudflare-docs: %v\n%s", err, getOut)
		}
		if getDoc.Name != cloudflareDocsCatalogServerName || getDoc.Transport.Type != "streamable_http" || strings.TrimSpace(getDoc.Transport.URL) != cloudflareDocsCatalogServerURL {
			t.Fatalf("unexpected codex mcp get cloudflare-docs output:\n%s", getOut)
		}
		listCmd := exec.Command(codexBin, "mcp", "list", "--json", "-c", fmt.Sprintf("mcp_servers.%s.url=%q", cloudflareDocsCatalogServerName, cloudflareDocsCatalogServerURL))
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp list cloudflare-docs: %v\n%s", err, listOut)
		}
		var listDoc []struct {
			Name       string `json:"name"`
			AuthStatus string `json:"auth_status"`
			Transport  struct {
				Type string `json:"type"`
				URL  string `json:"url"`
			} `json:"transport"`
		}
		if err := json.Unmarshal(listOut, &listDoc); err != nil {
			t.Fatalf("parse codex mcp list cloudflare-docs: %v\n%s", err, listOut)
		}
		assertCloudflareDocsCodexRemoteCatalogEntry(t, listDoc, cloudflareDocsCatalogServerName, cloudflareDocsCatalogServerURL)
	})

	t.Run("Gemini_extension_validate_link_and_list", func(t *testing.T) {
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
		if !strings.Contains(listOutput, "cloudflare-docs") || !strings.Contains(listOutput, "MCP servers:") {
			t.Fatalf("gemini extensions list missing cloudflare-docs MCP projection:\n%s", listOutput)
		}
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
		if !strings.Contains(string(listOut), cloudflareDocsCatalogServerName) || !strings.Contains(strings.ToLower(string(listOut)), "needs approval") {
			t.Fatalf("cursor mcp list missing cloudflare-docs approval state:\n%s", listOut)
		}
		enableArgs := cursorCLIArgs(cursorBin, "mcp", "enable", cloudflareDocsCatalogServerName)
		enableCmd := exec.Command(cursorBin, enableArgs...)
		enableCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		enableOut, err := enableCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp enable cloudflare-docs with isolated config: %v\n%s", err, enableOut)
		}
		listCmd = exec.Command(cursorBin, listArgs...)
		listCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		listOut, err = listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp list after enabling cloudflare-docs: %v\n%s", err, listOut)
		}
		assertCursorRemoteCatalogState(t, string(listOut), cloudflareDocsCatalogServerName)
	})

	t.Run("OpenCode_workspace_serve_startup", func(t *testing.T) {
		opencodeBin := installedOpenCodeBinaryOrSkip(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, opencodeBin, "serve")
		cmd.Dir = pluginDir
		cmd.Env = append(os.Environ(), "OPENCODE_SERVER_PASSWORD=plugin-kit-ai-cloudflare-docs-live")
		outPath := filepath.Join(t.TempDir(), "opencode-cloudflare-docs.log")
		logFile, err := os.Create(outPath)
		if err != nil {
			t.Fatal(err)
		}
		defer logFile.Close()
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			t.Fatalf("start opencode serve in cloudflare-docs workspace: %v", err)
		}
		waitCh := make(chan error, 1)
		go func() { waitCh <- cmd.Wait() }()
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

func resolveCloudflareDocsCatalogPluginDir(t *testing.T) string {
	t.Helper()
	if dir := strings.TrimSpace(os.Getenv(cloudflareDocsCatalogDirEnvVar)); dir != "" {
		if fileExists(filepath.Join(dir, "plugin", "plugin.yaml")) {
			return dir
		}
		t.Fatalf("%s=%q does not point to a cloudflare-docs plugin with plugin/plugin.yaml", cloudflareDocsCatalogDirEnvVar, dir)
	}
	root := RepoRoot(t)
	candidate := filepath.Join(filepath.Dir(root), "universal-plugins-for-ai-agents", "plugins", "cloudflare-docs")
	if fileExists(filepath.Join(candidate, "plugin", "plugin.yaml")) {
		return candidate
	}
	t.Skipf("cloudflare-docs catalog plugin not found; set %s=/abs/path/to/universal-plugins-for-ai-agents/plugins/cloudflare-docs", cloudflareDocsCatalogDirEnvVar)
	return ""
}

func assertCloudflareDocsCatalogRenderedAndValid(t *testing.T, pluginKitAIBin, pluginDir string) {
	t.Helper()
	root := RepoRoot(t)
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir, "--check"))
	for _, platform := range []string{"claude", "codex-package", "gemini", "opencode", "cursor"} {
		runCmd(t, root, exec.Command(pluginKitAIBin, "validate", pluginDir, "--platform", platform, "--strict"))
	}
}

func assertCloudflareDocsRenderedServer(t *testing.T, server map[string]any) {
	t.Helper()
	if got := anyString(server["type"]); got != "http" {
		t.Fatalf("generated cloudflare-docs .mcp.json type = %q want http:\n%v", got, server)
	}
	if got := anyString(server["url"]); got != cloudflareDocsCatalogServerURL {
		t.Fatalf("generated cloudflare-docs .mcp.json url = %q want %s:\n%v", got, cloudflareDocsCatalogServerURL, server)
	}
	if _, ok := server["headers"]; ok {
		t.Fatalf("generated cloudflare-docs .mcp.json should not embed headers:\n%v", server)
	}
}

func assertCloudflareDocsCodexRemoteCatalogEntry(t *testing.T, entries []struct {
	Name       string `json:"name"`
	AuthStatus string `json:"auth_status"`
	Transport  struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"transport"`
}, wantName, wantURL string) {
	t.Helper()
	for _, entry := range entries {
		if entry.Name != wantName {
			continue
		}
		if entry.Transport.Type != "streamable_http" || strings.TrimSpace(entry.Transport.URL) != wantURL {
			t.Fatalf("codex list entry for %s has unexpected transport: %#v", wantName, entry)
		}
		if strings.TrimSpace(entry.AuthStatus) == "" {
			t.Fatalf("codex list entry for %s missing auth boundary: %#v", wantName, entry)
		}
		return
	}
	t.Fatalf("codex mcp list missing %s entry", wantName)
}
