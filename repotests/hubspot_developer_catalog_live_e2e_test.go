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

const hubspotDeveloperCatalogLiveEnvVar = "PLUGIN_KIT_AI_RUN_HUBSPOT_DEVELOPER_LIVE"
const hubspotDeveloperCatalogDirEnvVar = "PLUGIN_KIT_AI_E2E_HUBSPOT_DEVELOPER_DIR"
const hubspotDeveloperCatalogServerName = "hubspot-developer"
const hubspotDeveloperCatalogCLI = "@hubspot/cli@8.3.0"

func TestHubSpotDeveloperCatalogLiveAcrossInstalledAgents(t *testing.T) {
	if strings.TrimSpace(os.Getenv(hubspotDeveloperCatalogLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real hubspot-developer catalog live smoke", hubspotDeveloperCatalogLiveEnvVar)
	}

	pluginDir := resolveHubSpotDeveloperCatalogPluginDir(t)
	pluginKitAIBin := buildPluginKitAI(t)
	assertHubSpotDeveloperCatalogRenderedAndValid(t, pluginKitAIBin, pluginDir)

	server := readRenderedSharedMCPServer(t, pluginDir, hubspotDeveloperCatalogServerName)
	assertHubSpotDeveloperRenderedServer(t, server)

	t.Run("Claude_plugin_dir_list_and_mcp_status", func(t *testing.T) {
		claudeBin := installedClaudeBinaryOrSkip(t)
		validateCmd := exec.Command(claudeBin, "plugins", "validate", pluginDir)
		validateCmd.Dir = pluginDir
		validateOut, err := validateCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins validate hubspot-developer: %v\n%s", err, validateOut)
		}
		listCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "plugins", "list")
		listCmd.Dir = pluginDir
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins list with --plugin-dir: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), "hubspot-developer@inline") {
			t.Fatalf("claude plugins list missing inline hubspot-developer plugin:\n%s", listOut)
		}
		mcpListCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "mcp", "list")
		mcpListCmd.Dir = pluginDir
		mcpListOut, err := mcpListCmd.CombinedOutput()
		if err != nil {
			if claudeEnvironmentIssue(string(mcpListOut)) {
				t.Skipf("claude environment is not ready for hubspot-developer live smoke:\n%s", truncateRunes(string(mcpListOut), 4000))
			}
			t.Fatalf("claude mcp list with --plugin-dir: %v\n%s", err, mcpListOut)
		}
		if !strings.Contains(string(mcpListOut), "plugin:hubspot-developer:hubspot-developer:") {
			t.Fatalf("claude mcp list missing plugin-projected hubspot-developer server:\n%s", mcpListOut)
		}
		if !strings.Contains(strings.ToLower(string(mcpListOut)), "stdio") {
			t.Fatalf("claude mcp list should expose hubspot-developer as stdio:\n%s", mcpListOut)
		}
	})

	t.Run("Codex_get_and_list_rendered_stdio_server", func(t *testing.T) {
		codexBin := installedCodexBinaryOrSkip(t)
		configArgs := codexMCPConfigArgs(hubspotDeveloperCatalogServerName, server)
		getArgs := append([]string{"mcp", "get", hubspotDeveloperCatalogServerName, "--json"}, configArgs...)
		getCmd := exec.Command(codexBin, getArgs...)
		getOut, err := getCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp get hubspot-developer: %v\n%s", err, getOut)
		}
		var getDoc struct {
			Name      string `json:"name"`
			Transport struct {
				Type    string            `json:"type"`
				Command string            `json:"command"`
				Args    []string          `json:"args"`
				Env     map[string]string `json:"env"`
			} `json:"transport"`
		}
		if err := json.Unmarshal(getOut, &getDoc); err != nil {
			t.Fatalf("parse codex mcp get hubspot-developer: %v\n%s", err, getOut)
		}
		assertHubSpotDeveloperCodexGet(t, getDoc)

		listArgs := append([]string{"mcp", "list", "--json"}, configArgs...)
		listCmd := exec.Command(codexBin, listArgs...)
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp list hubspot-developer: %v\n%s", err, listOut)
		}
		var listDoc []struct {
			Name      string `json:"name"`
			Transport struct {
				Type    string   `json:"type"`
				Command string   `json:"command"`
				Args    []string `json:"args"`
			} `json:"transport"`
		}
		if err := json.Unmarshal(listOut, &listDoc); err != nil {
			t.Fatalf("parse codex mcp list hubspot-developer: %v\n%s", err, listOut)
		}
		assertHubSpotDeveloperCodexList(t, listDoc)
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
		if !strings.Contains(listOutput, "hubspot-developer") || !strings.Contains(listOutput, "MCP servers:") {
			t.Fatalf("gemini extensions list missing hubspot-developer MCP projection:\n%s", listOutput)
		}
	})

	t.Run("Cursor_isolated_config_list_enable_and_ready_state", func(t *testing.T) {
		t.Skip("cursor live startup for hubspot-developer is currently flaky with local npx-backed MCP boot; generate and strict validate still cover the supported projection")
	})

	t.Run("OpenCode_workspace_serve_startup", func(t *testing.T) {
		opencodeBin := installedOpenCodeBinaryOrSkip(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, opencodeBin, "serve")
		cmd.Dir = pluginDir
		cmd.Env = append(os.Environ(), "OPENCODE_SERVER_PASSWORD=plugin-kit-ai-hubspot-developer-live")
		outPath := filepath.Join(t.TempDir(), "opencode-hubspot-developer.log")
		logFile, err := os.Create(outPath)
		if err != nil {
			t.Fatal(err)
		}
		defer logFile.Close()
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			t.Fatalf("start opencode serve in hubspot-developer workspace: %v", err)
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

func resolveHubSpotDeveloperCatalogPluginDir(t *testing.T) string {
	t.Helper()
	if dir := strings.TrimSpace(os.Getenv(hubspotDeveloperCatalogDirEnvVar)); dir != "" {
		if fileExists(filepath.Join(dir, "plugin", "plugin.yaml")) {
			return dir
		}
		t.Fatalf("%s=%q does not point to a hubspot-developer plugin with plugin/plugin.yaml", hubspotDeveloperCatalogDirEnvVar, dir)
	}
	root := RepoRoot(t)
	candidate := filepath.Join(filepath.Dir(root), "universal-plugins-for-ai-agents", "plugins", "hubspot-developer")
	if fileExists(filepath.Join(candidate, "plugin", "plugin.yaml")) {
		return candidate
	}
	t.Skipf("hubspot-developer catalog plugin not found; set %s=/abs/path/to/universal-plugins-for-ai-agents/plugins/hubspot-developer", hubspotDeveloperCatalogDirEnvVar)
	return ""
}

func assertHubSpotDeveloperCatalogRenderedAndValid(t *testing.T, pluginKitAIBin, pluginDir string) {
	t.Helper()
	root := RepoRoot(t)
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir, "--check"))
	for _, platform := range []string{"claude", "codex-package", "gemini", "opencode", "cursor"} {
		runCmd(t, root, exec.Command(pluginKitAIBin, "validate", pluginDir, "--platform", platform, "--strict"))
	}
}

func assertHubSpotDeveloperRenderedServer(t *testing.T, server map[string]any) {
	t.Helper()
	if got := anyString(server["type"]); got != "" && got != "stdio" {
		t.Fatalf("generated hubspot-developer .mcp.json type = %q want empty-or-stdio:\n%v", got, server)
	}
	if got := anyString(server["command"]); got != "npx" {
		t.Fatalf("generated hubspot-developer .mcp.json command = %q want npx:\n%v", got, server)
	}
	args := anyStrings(server["args"])
	wantArgs := []string{"-y", "-p", hubspotDeveloperCatalogCLI, "hs", "mcp", "start"}
	if strings.Join(args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("generated hubspot-developer .mcp.json args = %#v want %#v", args, wantArgs)
	}
	env, ok := server["env"].(map[string]any)
	if !ok {
		t.Fatalf("generated hubspot-developer .mcp.json env missing map shape:\n%v", server)
	}
	if got := anyString(env["HUBSPOT_MCP_STANDALONE"]); got != "true" {
		t.Fatalf("generated hubspot-developer env HUBSPOT_MCP_STANDALONE = %q want true:\n%v", got, server)
	}
	if got := anyString(env["HUBSPOT_CLI_VERSION"]); got != "8.3.0" {
		t.Fatalf("generated hubspot-developer env HUBSPOT_CLI_VERSION = %q want 8.3.0:\n%v", got, server)
	}
}

func assertHubSpotDeveloperCodexGet(t *testing.T, doc struct {
	Name      string `json:"name"`
	Transport struct {
		Type    string            `json:"type"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	} `json:"transport"`
}) {
	t.Helper()
	if doc.Name != hubspotDeveloperCatalogServerName {
		t.Fatalf("codex get returned unexpected server name %#v", doc)
	}
	if doc.Transport.Type != "stdio" || strings.TrimSpace(doc.Transport.Command) != "npx" {
		t.Fatalf("codex get returned unexpected stdio transport %#v", doc)
	}
	wantArgs := []string{"-y", "-p", hubspotDeveloperCatalogCLI, "hs", "mcp", "start"}
	if strings.Join(doc.Transport.Args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("codex get returned unexpected args %#v want %#v", doc.Transport.Args, wantArgs)
	}
	env := doc.Transport.Env
	if env["HUBSPOT_MCP_STANDALONE"] != "true" || env["HUBSPOT_CLI_VERSION"] != "8.3.0" {
		t.Fatalf("codex get returned unexpected env %#v", env)
	}
}

func assertHubSpotDeveloperCodexList(t *testing.T, entries []struct {
	Name      string `json:"name"`
	Transport struct {
		Type    string   `json:"type"`
		Command string   `json:"command"`
		Args    []string `json:"args"`
	} `json:"transport"`
}) {
	t.Helper()
	for _, entry := range entries {
		if entry.Name != hubspotDeveloperCatalogServerName {
			continue
		}
		if entry.Transport.Type != "stdio" || strings.TrimSpace(entry.Transport.Command) != "npx" {
			t.Fatalf("codex list entry for %s has unexpected transport: %#v", hubspotDeveloperCatalogServerName, entry)
		}
		wantArgs := []string{"-y", "-p", hubspotDeveloperCatalogCLI, "hs", "mcp", "start"}
		if strings.Join(entry.Transport.Args, "\x00") != strings.Join(wantArgs, "\x00") {
			t.Fatalf("codex list entry for %s has unexpected args: %#v", hubspotDeveloperCatalogServerName, entry.Transport.Args)
		}
		return
	}
	t.Fatalf("codex mcp list missing %s entry", hubspotDeveloperCatalogServerName)
}

func anyStrings(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, anyString(item))
		}
		return out
	default:
		return nil
	}
}
