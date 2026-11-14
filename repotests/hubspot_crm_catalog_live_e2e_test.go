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

const hubspotCRMCatalogLiveEnvVar = "PLUGIN_KIT_AI_RUN_HUBSPOT_CRM_LIVE"
const hubspotCRMCatalogDirEnvVar = "PLUGIN_KIT_AI_E2E_HUBSPOT_CRM_DIR"
const hubspotCRMCatalogServerName = "hubspot-crm"
const hubspotCRMCatalogServerURL = "https://mcp.hubspot.com"

func TestHubSpotCRMCatalogLiveAcrossInstalledAgents(t *testing.T) {
	if strings.TrimSpace(os.Getenv(hubspotCRMCatalogLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real hubspot-crm catalog live smoke", hubspotCRMCatalogLiveEnvVar)
	}

	pluginDir := resolveHubSpotCRMCatalogPluginDir(t)
	pluginKitAIBin := buildPluginKitAI(t)
	assertHubSpotCRMCatalogRenderedAndValid(t, pluginKitAIBin, pluginDir)

	server := readRenderedSharedMCPServer(t, pluginDir, hubspotCRMCatalogServerName)
	assertHubSpotCRMRenderedServer(t, server)

	t.Run("Claude_plugin_dir_list_and_mcp_status", func(t *testing.T) {
		claudeBin := installedClaudeBinaryOrSkip(t)
		validateCmd := exec.Command(claudeBin, "plugins", "validate", pluginDir)
		validateCmd.Dir = pluginDir
		validateOut, err := validateCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins validate hubspot-crm: %v\n%s", err, validateOut)
		}
		listCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "plugins", "list")
		listCmd.Dir = pluginDir
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins list with --plugin-dir: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), "hubspot-crm@inline") {
			t.Fatalf("claude plugins list missing inline hubspot-crm plugin:\n%s", listOut)
		}
		mcpListCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "mcp", "list")
		mcpListCmd.Dir = pluginDir
		mcpListOut, err := mcpListCmd.CombinedOutput()
		if err != nil {
			if claudeEnvironmentIssue(string(mcpListOut)) {
				t.Skipf("claude environment is not ready for hubspot-crm live smoke:\n%s", truncateRunes(string(mcpListOut), 4000))
			}
			t.Fatalf("claude mcp list with --plugin-dir: %v\n%s", err, mcpListOut)
		}
		assertClaudeRemoteCatalogStatus(t, string(mcpListOut), "plugin:hubspot-crm:hubspot-crm:", hubspotCRMCatalogServerURL)
	})

	t.Run("Codex_get_and_list_rendered_remote_server", func(t *testing.T) {
		codexBin := installedCodexBinaryOrSkip(t)
		getCmd := exec.Command(codexBin, "mcp", "get", hubspotCRMCatalogServerName, "--json", "-c", fmt.Sprintf("mcp_servers.%s.url=%q", hubspotCRMCatalogServerName, hubspotCRMCatalogServerURL))
		getOut, err := getCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp get hubspot-crm: %v\n%s", err, getOut)
		}
		var getDoc struct {
			Name      string `json:"name"`
			Transport struct {
				Type string `json:"type"`
				URL  string `json:"url"`
			} `json:"transport"`
		}
		if err := json.Unmarshal(getOut, &getDoc); err != nil {
			t.Fatalf("parse codex mcp get hubspot-crm: %v\n%s", err, getOut)
		}
		if getDoc.Name != hubspotCRMCatalogServerName || getDoc.Transport.Type != "streamable_http" || strings.TrimSpace(getDoc.Transport.URL) != hubspotCRMCatalogServerURL {
			t.Fatalf("unexpected codex mcp get hubspot-crm output:\n%s", getOut)
		}
		listCmd := exec.Command(codexBin, "mcp", "list", "--json", "-c", fmt.Sprintf("mcp_servers.%s.url=%q", hubspotCRMCatalogServerName, hubspotCRMCatalogServerURL))
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp list hubspot-crm: %v\n%s", err, listOut)
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
			t.Fatalf("parse codex mcp list hubspot-crm: %v\n%s", err, listOut)
		}
		assertHubSpotCRMCodexRemoteCatalogEntry(t, listDoc, hubspotCRMCatalogServerName, hubspotCRMCatalogServerURL)
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
		if !strings.Contains(listOutput, "hubspot-crm") || !strings.Contains(listOutput, "MCP servers:") {
			t.Fatalf("gemini extensions list missing hubspot-crm MCP projection:\n%s", listOutput)
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
		if !strings.Contains(string(listOut), hubspotCRMCatalogServerName) || !strings.Contains(strings.ToLower(string(listOut)), "needs approval") {
			t.Fatalf("cursor mcp list missing hubspot-crm approval state:\n%s", listOut)
		}
		enableArgs := cursorCLIArgs(cursorBin, "mcp", "enable", hubspotCRMCatalogServerName)
		enableCmd := exec.Command(cursorBin, enableArgs...)
		enableCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		enableOut, err := enableCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp enable hubspot-crm with isolated config: %v\n%s", err, enableOut)
		}
		listCmd = exec.Command(cursorBin, listArgs...)
		listCmd.Env = append(os.Environ(), "HOME="+cursorHome)
		listOut, err = listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp list after enabling hubspot-crm: %v\n%s", err, listOut)
		}
		assertHubSpotCRMCursorCatalogState(t, string(listOut))
	})

	t.Run("OpenCode_workspace_serve_startup", func(t *testing.T) {
		opencodeBin := installedOpenCodeBinaryOrSkip(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, opencodeBin, "serve")
		cmd.Dir = pluginDir
		cmd.Env = append(os.Environ(), "OPENCODE_SERVER_PASSWORD=plugin-kit-ai-hubspot-crm-live")
		outPath := filepath.Join(t.TempDir(), "opencode-hubspot-crm.log")
		logFile, err := os.Create(outPath)
		if err != nil {
			t.Fatal(err)
		}
		defer logFile.Close()
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			t.Fatalf("start opencode serve in hubspot-crm workspace: %v", err)
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

func resolveHubSpotCRMCatalogPluginDir(t *testing.T) string {
	t.Helper()
	if dir := strings.TrimSpace(os.Getenv(hubspotCRMCatalogDirEnvVar)); dir != "" {
		if fileExists(filepath.Join(dir, "plugin", "plugin.yaml")) {
			return dir
		}
		t.Fatalf("%s=%q does not point to a hubspot-crm plugin with plugin/plugin.yaml", hubspotCRMCatalogDirEnvVar, dir)
	}
	root := RepoRoot(t)
	candidate := filepath.Join(filepath.Dir(root), "universal-plugins-for-ai-agents", "plugins", "hubspot-crm")
	if fileExists(filepath.Join(candidate, "plugin", "plugin.yaml")) {
		return candidate
	}
	t.Skipf("hubspot-crm catalog plugin not found; set %s=/abs/path/to/universal-plugins-for-ai-agents/plugins/hubspot-crm", hubspotCRMCatalogDirEnvVar)
	return ""
}

func assertHubSpotCRMCatalogRenderedAndValid(t *testing.T, pluginKitAIBin, pluginDir string) {
	t.Helper()
	root := RepoRoot(t)
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir, "--check"))
	for _, platform := range []string{"claude", "codex-package", "gemini", "opencode", "cursor"} {
		runCmd(t, root, exec.Command(pluginKitAIBin, "validate", pluginDir, "--platform", platform, "--strict"))
	}
}

func assertHubSpotCRMRenderedServer(t *testing.T, server map[string]any) {
	t.Helper()
	if got := anyString(server["type"]); got != "http" {
		t.Fatalf("generated hubspot-crm .mcp.json type = %q want http:\n%v", got, server)
	}
	if got := anyString(server["url"]); got != hubspotCRMCatalogServerURL {
		t.Fatalf("generated hubspot-crm .mcp.json url = %q want %s:\n%v", got, hubspotCRMCatalogServerURL, server)
	}
	if _, ok := server["headers"]; ok {
		t.Fatalf("generated hubspot-crm .mcp.json should not embed headers:\n%v", server)
	}
}

func assertHubSpotCRMCodexRemoteCatalogEntry(t *testing.T, entries []struct {
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

func assertHubSpotCRMCursorCatalogState(t *testing.T, output string) {
	t.Helper()
	lower := strings.ToLower(output)
	if !strings.Contains(output, hubspotCRMCatalogServerName) {
		t.Fatalf("cursor mcp list missing %s entry:\n%s", hubspotCRMCatalogServerName, output)
	}
	if strings.Contains(lower, "auth") || strings.Contains(lower, "ready") || strings.Contains(lower, "connection failed") {
		return
	}
	t.Fatalf("cursor mcp list missing acceptable hubspot-crm hosted boundary state:\n%s", output)
}
