package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProductionExamples_RenderValidateBuildAndSmoke(t *testing.T) {
	root := RepoRoot(t)
	pluginKitAIBin := buildPluginKitAI(t)

	cases := []struct {
		name     string
		dir      string
		platform string
		binary   string
		buildGo  bool
		smoke    func(t *testing.T, binary string)
	}{
		{
			name:     "claude",
			dir:      filepath.Join(root, "examples", "plugins", "claude-basic-prod"),
			platform: "claude",
			binary:   "claude-basic-prod",
			buildGo:  true,
			smoke:    smokeClaudeStableSubset,
		},
		{
			name:     "codex",
			dir:      filepath.Join(root, "examples", "plugins", "codex-basic-prod"),
			platform: "codex-runtime",
			binary:   "codex-basic-prod",
			buildGo:  true,
			smoke:    smokeCodexNotify,
		},
		{
			name:     "codex-package",
			dir:      filepath.Join(root, "examples", "plugins", "codex-package-prod"),
			platform: "codex-package",
		},
		{
			name:     "gemini",
			dir:      filepath.Join(root, "examples", "plugins", "gemini-extension-package"),
			platform: "gemini",
		},
		{
			name:     "cursor",
			dir:      filepath.Join(root, "examples", "plugins", "cursor-basic"),
			platform: "cursor",
		},
		{
			name:     "opencode",
			dir:      filepath.Join(root, "examples", "plugins", "opencode-basic"),
			platform: "opencode",
		},
		{
			name:     "qoder",
			dir:      filepath.Join(root, "examples", "plugins", "qoder-basic"),
			platform: "qoder",
		},
		{
			name:     "grok",
			dir:      filepath.Join(root, "examples", "plugins", "grok-basic"),
			platform: "grok",
		},
		{
			name:     "kimi",
			dir:      filepath.Join(root, "examples", "plugins", "kimi-basic"),
			platform: "kimi",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := tc.dir
			if tc.buildGo || tc.platform == "codex-package" || tc.platform == "gemini" || tc.platform == "cursor" || tc.platform == "opencode" || tc.platform == "qoder" || tc.platform == "grok" || tc.platform == "kimi" {
				workDir = filepath.Join(t.TempDir(), filepath.Base(tc.dir))
				copyTree(t, tc.dir, workDir)
			}
			if tc.buildGo {
				bootstrapGeneratedGoPlugin(t, workDir)
			}

			runCmd(t, root, exec.Command(pluginKitAIBin, "generate", workDir, "--check"))
			runCmd(t, root, exec.Command(pluginKitAIBin, "validate", workDir, "--platform", tc.platform, "--strict"))
			if tc.platform == "codex-runtime" {
				assertCodexConfig(t, workDir, "gpt-5.4-mini", "./bin/codex-basic-prod")
			}
			if tc.platform == "codex-package" {
				assertCodexPackageManifest(t, workDir, "codex-package-prod")
			}
			if tc.platform == "gemini" {
				return
			}
			if tc.platform == "cursor" {
				assertCursorConfig(t, workDir)
				return
			}
			if tc.platform == "opencode" {
				assertOpenCodeConfig(t, workDir, "@acme/opencode-demo-plugin")
				return
			}
			if tc.platform == "qoder" {
				assertQoderConfig(t, workDir, "qoder-basic")
				return
			}
			if tc.platform == "grok" {
				assertGrokConfig(t, workDir, "grok-basic")
				return
			}
			if tc.platform == "kimi" {
				assertKimiConfig(t, workDir, "kimi-basic")
				return
			}
			if !tc.buildGo {
				return
			}

			testCmd := exec.Command("go", "test", "./...")
			testCmd.Dir = workDir
			testCmd.Env = append(os.Environ(), "GOWORK=off")
			runCmd(t, root, testCmd)

			binDir := filepath.Join(workDir, "bin")
			if err := os.MkdirAll(binDir, 0o755); err != nil {
				t.Fatal(err)
			}
			binName := tc.binary
			if runtime.GOOS == "windows" {
				binName += ".exe"
			}
			binPath := filepath.Join(binDir, binName)
			buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/"+tc.binary)
			buildCmd.Dir = workDir
			buildCmd.Env = append(os.Environ(), "GOWORK=off")
			runCmd(t, root, buildCmd)

			if tc.smoke != nil {
				tc.smoke(t, binPath)
			}
		})
	}
}

func assertCursorConfig(t *testing.T, root string) {
	t.Helper()
	manifestBody, err := os.ReadFile(filepath.Join(root, ".cursor-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(manifestBody, []byte(`"mcpServers": "./.mcp.json"`)) {
		t.Fatalf("unexpected cursor plugin manifest:\n%s", manifestBody)
	}
	if !bytes.Contains(manifestBody, []byte(`"skills": "./skills/"`)) {
		t.Fatalf("unexpected cursor plugin manifest:\n%s", manifestBody)
	}
	body, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse .mcp.json: %v\n%s", err, body)
	}
	server, ok := doc["release-checks"]
	if !ok {
		t.Fatalf(".mcp.json missing release-checks server: %s", body)
	}
	if server.Command != "node" {
		t.Fatalf("release-checks command = %q want %q", server.Command, "node")
	}
	if len(server.Args) != 1 || server.Args[0] != "./bin/release-checks.mjs" {
		t.Fatalf("release-checks args = %v", server.Args)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "release-checks", "SKILL.md")); err != nil {
		t.Fatalf("stat generated cursor skill: %v", err)
	}
	generatedBody, err := os.ReadFile(filepath.Join(root, "GENERATED.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generatedBody, []byte(".cursor-plugin/plugin.json")) {
		t.Fatalf("unexpected GENERATED.md:\n%s", generatedBody)
	}
	agentsBody, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatal(err)
	}
	if !bytes.Contains(agentsBody, []byte("[`GENERATED.md`](./GENERATED.md)")) {
		t.Fatalf("unexpected root AGENTS.md:\n%s", agentsBody)
	}
	if !bytes.Contains(agentsBody, []byte("[`plugin/README.md`](./plugin/README.md)")) {
		t.Fatalf("unexpected root AGENTS.md:\n%s", agentsBody)
	}
	claudeBody, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err == nil && !bytes.Contains(claudeBody, []byte("[`AGENTS.md`](./AGENTS.md)")) {
		t.Fatalf("unexpected root CLAUDE.md:\n%s", claudeBody)
	}
}

func assertOpenCodeConfig(t *testing.T, root, wantPlugin string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Schema string         `json:"$schema"`
		Plugin []string       `json:"plugin"`
		MCP    map[string]any `json:"mcp"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse opencode config: %v\n%s", err, body)
	}
	if doc.Schema != "https://opencode.ai/config.json" {
		t.Fatalf("opencode.json schema = %q", doc.Schema)
	}
	if len(doc.Plugin) != 1 || doc.Plugin[0] != wantPlugin {
		t.Fatalf("opencode.json plugin = %v want [%q]", doc.Plugin, wantPlugin)
	}
	if len(doc.MCP) == 0 {
		t.Fatalf("opencode.json missing mcp config: %s", body)
	}
	if _, err := os.Stat(filepath.Join(root, ".opencode", "skills", "opencode-basic", "SKILL.md")); err != nil {
		t.Fatalf("stat mirrored opencode skill: %v", err)
	}
	for _, rel := range []string{
		filepath.Join(".opencode", "commands", "ship.md"),
		filepath.Join(".opencode", "agents", "reviewer.md"),
		filepath.Join(".opencode", "themes", "midnight.json"),
		filepath.Join(".opencode", "tools", "echo.ts"),
		filepath.Join(".opencode", "plugins", "example.js"),
		filepath.Join(".opencode", "plugins", "custom-tool.js"),
		filepath.Join(".opencode", "package.json"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	pluginBody, err := os.ReadFile(filepath.Join(root, ".opencode", "plugins", "example.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(pluginBody, []byte("export const ExamplePlugin = async")) {
		t.Fatalf("unexpected opencode example plugin:\n%s", pluginBody)
	}
	if bytes.Contains(pluginBody, []byte("export default")) {
		t.Fatalf("opencode example plugin still uses deprecated export default shape:\n%s", pluginBody)
	}
	toolBody, err := os.ReadFile(filepath.Join(root, ".opencode", "plugins", "custom-tool.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(toolBody, []byte(`@opencode-ai/plugin`)) {
		t.Fatalf("opencode custom tool fixture missing helper import:\n%s", toolBody)
	}
	if !bytes.Contains(toolBody, []byte("export const CustomToolPlugin = async")) {
		t.Fatalf("unexpected opencode custom tool fixture:\n%s", toolBody)
	}
	standaloneToolBody, err := os.ReadFile(filepath.Join(root, ".opencode", "tools", "echo.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(standaloneToolBody, []byte(`@opencode-ai/plugin`)) {
		t.Fatalf("opencode standalone tool fixture missing helper import:\n%s", standaloneToolBody)
	}
	if !bytes.Contains(standaloneToolBody, []byte("export default tool({")) {
		t.Fatalf("unexpected opencode standalone tool fixture:\n%s", standaloneToolBody)
	}
	packageBody, err := os.ReadFile(filepath.Join(root, ".opencode", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(packageBody, []byte(`"@opencode-ai/plugin"`)) {
		t.Fatalf("opencode package.json missing helper dependency:\n%s", packageBody)
	}
}

func assertQoderConfig(t *testing.T, root, wantName string) {
	t.Helper()
	manifestBody, err := os.ReadFile(filepath.Join(root, ".qoder-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatalf("parse qoder plugin manifest: %v\n%s", err, manifestBody)
	}
	if manifest["name"] != wantName {
		t.Fatalf(".qoder-plugin/plugin.json name = %q want %q", manifest["name"], wantName)
	}
	if manifest["mcpServers"] != "./.mcp.json" {
		t.Fatalf(".qoder-plugin/plugin.json mcpServers = %+v", manifest["mcpServers"])
	}
	if manifest["skills"] != "./skills/" {
		t.Fatalf(".qoder-plugin/plugin.json skills = %+v", manifest["skills"])
	}
	body, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse .mcp.json: %v\n%s", err, body)
	}
	server, ok := doc.MCPServers["release-checks"]
	if !ok {
		t.Fatalf(".mcp.json missing release-checks server: %s", body)
	}
	if server.Command != "node" {
		t.Fatalf("release-checks command = %q want %q", server.Command, "node")
	}
	if len(server.Args) != 1 || server.Args[0] != "${QODER_PLUGIN_ROOT}/bin/release-checks.mjs" {
		t.Fatalf("release-checks args = %v", server.Args)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "release-checks", "SKILL.md")); err != nil {
		t.Fatalf("stat generated qoder skill: %v", err)
	}
	generatedBody, err := os.ReadFile(filepath.Join(root, "GENERATED.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generatedBody, []byte(".qoder-plugin/plugin.json")) {
		t.Fatalf("unexpected GENERATED.md:\n%s", generatedBody)
	}
}

func assertGrokConfig(t *testing.T, root, wantName string) {
	t.Helper()
	manifestBody, err := os.ReadFile(filepath.Join(root, ".grok-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatalf("parse grok plugin manifest: %v\n%s", err, manifestBody)
	}
	if manifest["name"] != wantName {
		t.Fatalf(".grok-plugin/plugin.json name = %q want %q", manifest["name"], wantName)
	}
	if manifest["mcpServers"] != "./.mcp.json" {
		t.Fatalf(".grok-plugin/plugin.json mcpServers = %+v", manifest["mcpServers"])
	}
	if manifest["skills"] != "./skills/" {
		t.Fatalf(".grok-plugin/plugin.json skills = %+v", manifest["skills"])
	}
	body, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse .mcp.json: %v\n%s", err, body)
	}
	server, ok := doc.MCPServers["release-checks"]
	if !ok {
		t.Fatalf(".mcp.json missing release-checks server: %s", body)
	}
	if server.Command != "node" {
		t.Fatalf("release-checks command = %q want %q", server.Command, "node")
	}
	if len(server.Args) != 1 || server.Args[0] != "./bin/release-checks.mjs" {
		t.Fatalf("release-checks args = %v", server.Args)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "release-checks", "SKILL.md")); err != nil {
		t.Fatalf("stat generated grok skill: %v", err)
	}
	generatedBody, err := os.ReadFile(filepath.Join(root, "GENERATED.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generatedBody, []byte(".grok-plugin/plugin.json")) {
		t.Fatalf("unexpected GENERATED.md:\n%s", generatedBody)
	}
}

func assertKimiConfig(t *testing.T, root, wantName string) {
	t.Helper()
	manifestBody, err := os.ReadFile(filepath.Join(root, ".kimi-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Name       string `json:"name"`
		Skills     string `json:"skills"`
		MCPServers struct {
			ReleaseChecks struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			} `json:"release-checks"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatalf("parse kimi plugin manifest: %v\n%s", err, manifestBody)
	}
	if manifest.Name != wantName {
		t.Fatalf(".kimi-plugin/plugin.json name = %q want %q", manifest.Name, wantName)
	}
	if manifest.Skills != "./skills/" {
		t.Fatalf(".kimi-plugin/plugin.json skills = %+v", manifest.Skills)
	}
	if manifest.MCPServers.ReleaseChecks.Command != "node" {
		t.Fatalf(".kimi-plugin/plugin.json must inline the release-checks MCP server: %s", manifestBody)
	}
	if len(manifest.MCPServers.ReleaseChecks.Args) != 1 || manifest.MCPServers.ReleaseChecks.Args[0] != "./bin/release-checks.mjs" {
		t.Fatalf("release-checks args = %v", manifest.MCPServers.ReleaseChecks.Args)
	}
	if _, err := os.Stat(filepath.Join(root, ".mcp.json")); err == nil {
		t.Fatal("Kimi must not generate an MCP sidecar; servers are inline in the manifest")
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "release-checks", "SKILL.md")); err != nil {
		t.Fatalf("stat generated kimi skill: %v", err)
	}
	generatedBody, err := os.ReadFile(filepath.Join(root, "GENERATED.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generatedBody, []byte(".kimi-plugin/plugin.json")) {
		t.Fatalf("unexpected GENERATED.md:\n%s", generatedBody)
	}
}

func assertCodexPackageManifest(t *testing.T, root, wantName string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatalf("parse codex package manifest: %v\n%s", err, body)
	}
	if manifest["name"] != wantName {
		t.Fatalf(".codex-plugin/plugin.json name = %q want %q", manifest["name"], wantName)
	}
	if manifest["apps"] != "./.app.json" {
		t.Fatalf(".codex-plugin/plugin.json apps = %+v", manifest["apps"])
	}
	if manifest["mcpServers"] != "./.mcp.json" {
		t.Fatalf(".codex-plugin/plugin.json mcpServers = %+v", manifest["mcpServers"])
	}
	if manifest["homepage"] == "" || manifest["repository"] == "" || manifest["license"] == "" {
		t.Fatalf(".codex-plugin/plugin.json missing package metadata: %+v", manifest)
	}
	if _, ok := manifest["interface"].(map[string]any); !ok {
		t.Fatalf(".codex-plugin/plugin.json missing interface: %+v", manifest)
	}
	if _, err := os.Stat(filepath.Join(root, ".mcp.json")); err != nil {
		t.Fatalf("stat .mcp.json: %v", err)
	}
}

func smokeClaudeStableSubset(t *testing.T, binary string) {
	t.Helper()
	cases := []struct {
		name    string
		payload string
	}{
		{name: "Stop", payload: `{"session_id":"s","cwd":"/tmp","hook_event_name":"Stop"}`},
		{name: "PreToolUse", payload: `{"session_id":"e2e-session","transcript_path":"/tmp/t.jsonl","cwd":"/tmp","permission_mode":"default","hook_event_name":"PreToolUse","tool_name":"Bash","tool_use_id":"toolu_e2e","tool_input":{"command":"echo ok"}}`},
		{name: "UserPromptSubmit", payload: `{"session_id":"e2e-session","transcript_path":"/tmp/t.jsonl","cwd":"/tmp","permission_mode":"default","hook_event_name":"UserPromptSubmit","prompt":"hello e2e"}`},
	}
	for _, tc := range cases {
		cmd := exec.Command(binary, tc.name)
		cmd.Stdin = bytes.NewBufferString(tc.payload)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("claude smoke %s: %v\n%s", tc.name, err, out)
		}
		if string(bytes.TrimSpace(out)) != "{}" {
			t.Fatalf("claude smoke %s stdout = %q", tc.name, out)
		}
	}
}

func smokeCodexNotify(t *testing.T, binary string) {
	t.Helper()
	cmd := exec.Command(binary, "notify", `{"client":"codex-tui"}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("codex smoke: %v\n%s", err, out)
	}
	if len(bytes.TrimSpace(out)) != 0 {
		t.Fatalf("codex smoke stdout = %q", out)
	}
}

func runCmd(t *testing.T, root string, cmd *exec.Cmd) {
	t.Helper()
	if cmd.Dir == "" {
		cmd.Dir = root
	}
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", cmd.String(), err, out)
	}
}
