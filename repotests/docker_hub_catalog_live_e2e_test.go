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

const dockerHubCatalogLiveEnvVar = "PLUGIN_KIT_AI_RUN_DOCKER_HUB_LIVE"
const dockerHubCatalogDirEnvVar = "PLUGIN_KIT_AI_E2E_DOCKER_HUB_DIR"
const dockerHubCatalogServerName = "dockerhub"

func TestDockerHubCatalogLiveAcrossInstalledAgents(t *testing.T) {
	if strings.TrimSpace(os.Getenv(dockerHubCatalogLiveEnvVar)) != "1" {
		t.Skipf("set %s=1 to run real docker-hub catalog live smoke", dockerHubCatalogLiveEnvVar)
	}

	pluginDir := resolveDockerHubCatalogPluginDir(t)
	pluginKitAIBin := buildPluginKitAI(t)
	assertDockerHubCatalogRenderedAndValid(t, pluginKitAIBin, pluginDir)

	server := readRenderedSharedMCPServer(t, pluginDir, dockerHubCatalogServerName)
	assertDockerHubRenderedServer(t, server)

	t.Run("Docker_binary_is_usable_for_local_runtime", func(t *testing.T) {
		dockerUsableOrSkip(t)
	})

	t.Run("Claude_plugin_dir_list_and_mcp_status", func(t *testing.T) {
		claudeBin := installedClaudeBinaryOrSkip(t)

		validateCmd := exec.Command(claudeBin, "plugins", "validate", pluginDir)
		validateCmd.Dir = pluginDir
		validateOut, err := validateCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins validate docker-hub: %v\n%s", err, validateOut)
		}

		listCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "plugins", "list")
		listCmd.Dir = pluginDir
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude plugins list with --plugin-dir: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), "docker-hub@inline") {
			t.Fatalf("claude plugins list missing inline docker-hub plugin:\n%s", listOut)
		}

		mcpListCmd := exec.Command(claudeBin, "--plugin-dir", pluginDir, "mcp", "list")
		mcpListCmd.Dir = pluginDir
		mcpListOut, err := mcpListCmd.CombinedOutput()
		if err != nil {
			if claudeEnvironmentIssue(string(mcpListOut)) {
				t.Skipf("claude environment is not ready for docker-hub live smoke:\n%s", truncateRunes(string(mcpListOut), 4000))
			}
			t.Fatalf("claude mcp list with --plugin-dir: %v\n%s", err, mcpListOut)
		}
		if !strings.Contains(string(mcpListOut), "plugin:docker-hub:dockerhub:") {
			t.Fatalf("claude mcp list missing plugin-projected docker-hub server:\n%s", mcpListOut)
		}
		if !strings.Contains(strings.ToLower(string(mcpListOut)), "stdio") {
			t.Fatalf("claude mcp list should expose docker-hub as stdio:\n%s", mcpListOut)
		}
	})

	t.Run("Codex_get_and_list_rendered_stdio_server", func(t *testing.T) {
		codexBin := installedCodexBinaryOrSkip(t)
		configArgs := codexMCPConfigArgs(dockerHubCatalogServerName, server)

		getArgs := append([]string{"mcp", "get", dockerHubCatalogServerName, "--json"}, configArgs...)
		getCmd := exec.Command(codexBin, getArgs...)
		getOut, err := getCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp get docker-hub: %v\n%s", err, getOut)
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
			t.Fatalf("parse codex mcp get docker-hub: %v\n%s", err, getOut)
		}
		assertDockerHubCodexGet(t, getDoc)

		listArgs := append([]string{"mcp", "list", "--json"}, configArgs...)
		listCmd := exec.Command(codexBin, listArgs...)
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codex mcp list docker-hub: %v\n%s", err, listOut)
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
			t.Fatalf("parse codex mcp list docker-hub: %v\n%s", err, listOut)
		}
		assertDockerHubCodexList(t, listDoc)
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
		if !strings.Contains(listOutput, "docker-hub") || !strings.Contains(listOutput, "MCP servers:") {
			t.Fatalf("gemini extensions list missing docker-hub MCP projection:\n%s", listOutput)
		}
	})

	t.Run("Cursor_isolated_config_discovers_workspace_server", func(t *testing.T) {
		cursorBin := installedCursorBinaryOrSkip(t)
		cursorHome := newCursorIsolatedMCPHome(t, pluginDir)

		listArgs := cursorCLIArgs(cursorBin, "mcp", "list")
		listCmd := exec.Command(cursorBin, listArgs...)
		listCmd.Env = append(os.Environ(), "HOME="+cursorHome, "HUB_PAT_TOKEN=dummy", "HUB_USERNAME=dummy")
		listOut, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cursor mcp list with isolated config: %v\n%s", err, listOut)
		}
		if !strings.Contains(string(listOut), dockerHubCatalogServerName) || !strings.Contains(strings.ToLower(string(listOut)), "needs approval") {
			t.Fatalf("cursor mcp list missing docker-hub approval state:\n%s", listOut)
		}
	})

	t.Run("OpenCode_workspace_serve_startup", func(t *testing.T) {
		opencodeBin := installedOpenCodeBinaryOrSkip(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, opencodeBin, "serve")
		cmd.Dir = pluginDir
		cmd.Env = append(os.Environ(), "OPENCODE_SERVER_PASSWORD=plugin-kit-ai-docker-hub-live", "HUB_PAT_TOKEN=dummy", "HUB_USERNAME=dummy")
		outPath := filepath.Join(t.TempDir(), "opencode-docker-hub.log")
		logFile, err := os.Create(outPath)
		if err != nil {
			t.Fatal(err)
		}
		defer logFile.Close()
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			t.Fatalf("start opencode serve in docker-hub workspace: %v", err)
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

func resolveDockerHubCatalogPluginDir(t *testing.T) string {
	t.Helper()
	if dir := strings.TrimSpace(os.Getenv(dockerHubCatalogDirEnvVar)); dir != "" {
		if fileExists(filepath.Join(dir, "plugin", "plugin.yaml")) {
			return dir
		}
		t.Fatalf("%s=%q does not point to a docker-hub plugin with plugin/plugin.yaml", dockerHubCatalogDirEnvVar, dir)
	}
	root := RepoRoot(t)
	candidate := filepath.Join(filepath.Dir(root), "universal-plugins-for-ai-agents", "plugins", "docker-hub")
	if fileExists(filepath.Join(candidate, "plugin", "plugin.yaml")) {
		return candidate
	}
	t.Skipf("docker-hub catalog plugin not found; set %s=/abs/path/to/universal-plugins-for-ai-agents/plugins/docker-hub", dockerHubCatalogDirEnvVar)
	return ""
}

func assertDockerHubCatalogRenderedAndValid(t *testing.T, pluginKitAIBin, pluginDir string) {
	t.Helper()
	root := RepoRoot(t)
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", pluginDir, "--check"))
	for _, platform := range []string{"claude", "codex-package", "gemini", "opencode", "cursor"} {
		runCmd(t, root, exec.Command(pluginKitAIBin, "validate", pluginDir, "--platform", platform, "--strict"))
	}
}

func assertDockerHubRenderedServer(t *testing.T, server map[string]any) {
	t.Helper()
	if got := anyString(server["type"]); got != "" && got != "stdio" {
		t.Fatalf("generated docker-hub .mcp.json type = %q want empty-or-stdio:\n%v", got, server)
	}
	if got := anyString(server["command"]); got != "docker" {
		t.Fatalf("generated docker-hub .mcp.json command = %q want docker:\n%v", got, server)
	}
	wantArgs := []string{"run", "-i", "--rm", "-e", "HUB_PAT_TOKEN", "mcp/dockerhub", "--transport=stdio", "--username=${env:HUB_USERNAME}"}
	args := anyStrings(server["args"])
	if strings.Join(args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("generated docker-hub .mcp.json args = %#v want %#v", args, wantArgs)
	}
	env, ok := server["env"].(map[string]any)
	if !ok {
		t.Fatalf("generated docker-hub .mcp.json env missing map shape:\n%v", server)
	}
	if got := anyString(env["HUB_PAT_TOKEN"]); got != "${env:HUB_PAT_TOKEN}" {
		t.Fatalf("generated docker-hub env HUB_PAT_TOKEN = %q want ${env:HUB_PAT_TOKEN}:\n%v", got, server)
	}
	if got := anyString(env["HUB_USERNAME"]); got != "${env:HUB_USERNAME}" {
		t.Fatalf("generated docker-hub env HUB_USERNAME = %q want ${env:HUB_USERNAME}:\n%v", got, server)
	}
}

func assertDockerHubCodexGet(t *testing.T, doc struct {
	Name      string `json:"name"`
	Transport struct {
		Type    string            `json:"type"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	} `json:"transport"`
}) {
	t.Helper()
	if doc.Name != dockerHubCatalogServerName {
		t.Fatalf("codex get returned unexpected server name %#v", doc)
	}
	if doc.Transport.Type != "stdio" || strings.TrimSpace(doc.Transport.Command) != "docker" {
		t.Fatalf("codex get returned unexpected stdio transport %#v", doc)
	}
	wantArgs := []string{"run", "-i", "--rm", "-e", "HUB_PAT_TOKEN", "mcp/dockerhub", "--transport=stdio", "--username=${env:HUB_USERNAME}"}
	if strings.Join(doc.Transport.Args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("codex get returned unexpected args %#v want %#v", doc.Transport.Args, wantArgs)
	}
	if doc.Transport.Env["HUB_PAT_TOKEN"] != "${env:HUB_PAT_TOKEN}" {
		t.Fatalf("codex get returned unexpected env %#v", doc.Transport.Env)
	}
	if doc.Transport.Env["HUB_USERNAME"] != "${env:HUB_USERNAME}" {
		t.Fatalf("codex get returned unexpected env %#v", doc.Transport.Env)
	}
}

func assertDockerHubCodexList(t *testing.T, entries []struct {
	Name      string `json:"name"`
	Transport struct {
		Type    string   `json:"type"`
		Command string   `json:"command"`
		Args    []string `json:"args"`
	} `json:"transport"`
}) {
	t.Helper()
	for _, entry := range entries {
		if entry.Name != dockerHubCatalogServerName {
			continue
		}
		if entry.Transport.Type != "stdio" || strings.TrimSpace(entry.Transport.Command) != "docker" {
			t.Fatalf("codex list entry for %s has unexpected transport: %#v", dockerHubCatalogServerName, entry)
		}
		wantArgs := []string{"run", "-i", "--rm", "-e", "HUB_PAT_TOKEN", "mcp/dockerhub", "--transport=stdio", "--username=${env:HUB_USERNAME}"}
		if strings.Join(entry.Transport.Args, "\x00") != strings.Join(wantArgs, "\x00") {
			t.Fatalf("codex list entry for %s has unexpected args: %#v", dockerHubCatalogServerName, entry.Transport.Args)
		}
		return
	}
	t.Fatalf("codex list missing %s entry", dockerHubCatalogServerName)
}

func dockerUsableOrSkip(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("docker binary is not installed")
	}
	cmd := exec.Command(path, "version", "--format", "{{.Server.Version}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("docker is installed but not usable in this environment:\n%s", out)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Skipf("docker version output is empty:\n%s", out)
	}
	return path
}
