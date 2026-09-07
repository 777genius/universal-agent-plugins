package providers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestStagerBuildsOpenAIProjectionWithoutMutatingPortableSnapshot(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	writeTestFile(t, filepath.Join(envelope.SnapshotRoot, "hooks", "hooks.json"), "{}\n")
	plan := stagingPlan(t, domain.ClientCodex, domain.PackageProjection)
	delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "operation-1", domain.CompatibilityHints{
		OpenAIMCPAuth: map[string]domain.OpenAIMCPAuthHint{
			"notion": {OAuthResource: "https://mcp.notion.com"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(delivery.ArtifactDigest, "sha256:") {
		t.Fatalf("artifact digest = %q", delivery.ArtifactDigest)
	}
	assertMissing(t, filepath.Join(delivery.StagingPath, "skills", "bad"))
	assertExecutable(t, filepath.Join(delivery.StagingPath, "bin", "run"))

	portableManifest := readObject(t, filepath.Join(envelope.SnapshotRoot, "plugin.json"))
	if _, ok := portableManifest["extensions"]; !ok {
		t.Fatal("source snapshot was mutated")
	}
	if _, err := os.Stat(filepath.Join(envelope.SnapshotRoot, "hooks", "hooks.json")); err != nil {
		t.Fatalf("source hooks were mutated: %v", err)
	}
	assertMissing(t, filepath.Join(delivery.StagingPath, "hooks"))
	stagedManifest := readObject(t, filepath.Join(delivery.StagingPath, "plugin.json"))
	if _, ok := stagedManifest["extensions"]; ok {
		t.Fatal("unsupported extension survived staged standard manifest")
	}
	openAIManifest := readObject(t, filepath.Join(delivery.StagingPath, ".codex-plugin", "plugin.json"))
	if openAIManifest["mcpServers"] != "./.mcp.json" || openAIManifest["skills"] != "./skills/" {
		t.Fatalf("OpenAI manifest = %+v", openAIManifest)
	}
	openAIMCP := readObject(t, filepath.Join(delivery.StagingPath, ".mcp.json"))
	servers := openAIMCP["mcpServers"].(map[string]any)
	notion := servers["notion"].(map[string]any)
	if notion["type"] != "http" || notion["oauth_resource"] != "https://mcp.notion.com" {
		t.Fatalf("OpenAI MCP = %+v", notion)
	}
	codexMarketplace := readObject(t, filepath.Join(delivery.StagingPath, ".agents", "plugins", "marketplace.json"))
	if codexMarketplace["name"] != managedMarketplaceName(plan.PhysicalArtifactID) {
		t.Fatalf("Codex marketplace = %+v", codexMarketplace)
	}
	plugins := codexMarketplace["plugins"].([]any)
	source := plugins[0].(map[string]any)["source"].(map[string]any)
	if source["source"] != "local" || source["path"] != "./" {
		t.Fatalf("Codex marketplace source = %+v", source)
	}
	standardMCP := readObject(t, filepath.Join(delivery.StagingPath, "mcp.json"))
	standardServers := standardMCP["mcpServers"].(map[string]any)
	if _, exists := standardServers["broken"]; exists {
		t.Fatal("invalid MCP server survived staging")
	}
}

func TestStagerRemovesReportedIgnoredV1Extensions(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	writeTestFile(t, filepath.Join(envelope.SnapshotRoot, "plugin.json"), `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "demo",
  "extensions": []
}`)
	envelope.Manifest.Extensions = nil
	envelope.Diagnostics = []domain.Diagnostic{{
		Severity: domain.SeverityWarning,
		Boundary: domain.BoundaryPlugin,
		Code:     "plugin_extensions_ignored",
		Path:     "plugin.json",
		Item:     "extensions",
	}}
	plan := stagingPlan(t, domain.ClientCursor, domain.PackageNative)
	delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "ignored-extensions", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	manifest := readObject(t, filepath.Join(delivery.StagingPath, "plugin.json"))
	if _, exists := manifest["extensions"]; exists {
		t.Fatal("reported-and-ignored extensions survived staging")
	}
}

func TestStagerProjectsClaudeSkillsDirectoryPluginWithRootMCP(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	plan := stagingPlan(t, domain.ClientClaude, domain.PackageProjection)
	delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "operation-claude", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	manifest := readObject(t, filepath.Join(delivery.StagingPath, ".claude-plugin", "plugin.json"))
	if manifest["name"] != "demo" || manifest["skills"] != "./.agentplugins-runtime/skills/" || manifest["mcpServers"] != "./.mcp.json" {
		t.Fatalf("Claude manifest = %+v", manifest)
	}
	mcp := readObject(t, filepath.Join(delivery.StagingPath, ".mcp.json"))
	notion := mcp["notion"].(map[string]any)
	if notion["type"] != "http" || notion["url"] != "https://mcp.notion.com/mcp" {
		t.Fatalf("Claude root MCP = %+v", mcp)
	}
	assertMissing(t, filepath.Join(delivery.StagingPath, "mcp.json"))
	if _, err := os.Stat(filepath.Join(delivery.StagingPath, ".agentplugins-runtime", "skills", "good", "SKILL.md")); err != nil {
		t.Fatalf("Claude skill missing: %v", err)
	}
	assertMissing(t, filepath.Join(delivery.StagingPath, "skills"))
}

func TestClaudeProjectionExposesOnlyPlannedSurfacesAndRebindsRuntime(t *testing.T) {
	envelope := stagingEnvelope(t)
	for _, surface := range []string{"commands/run.md", "agents/helper.md", "hooks/hooks.json", ".lsp/config.json", "output-styles/style.md"} {
		writeTestFile(t, filepath.Join(envelope.SnapshotRoot, surface), "unplanned\n")
	}
	writeTestFile(t, filepath.Join(envelope.SnapshotRoot, "skills", "extra", "SKILL.md"), "---\nname: extra\ndescription: Extra\n---\n")
	server := domain.MCPServer{Name: "local", Type: "stdio", Decoded: map[string]any{
		"type": "stdio", "command": "node", "args": []any{"${PLUGIN_ROOT}/bin/run"},
		"env": map[string]any{"CACHE": "${PLUGIN_DATA}/cache"},
	}}
	envelope.MCP.Servers = map[string]domain.MCPServer{"local": server}
	plan := stagingPlan(t, domain.ClientClaude, domain.PackageProjection)
	plan.Components = []domain.ComponentDecision{
		{Kind: domain.ComponentSkill, Name: "good", Support: domain.SupportProjected},
		{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportProjected},
	}
	ownedData := filepath.Join(t.TempDir(), "data")
	delivery, err := windsurfFixtureStager(t).StageWithPluginData(context.Background(), envelope, plan, "claude-isolated", domain.CompatibilityHints{}, ownedData)
	if err != nil {
		t.Fatal(err)
	}
	if delivery.OwnedBase != plan.TargetRoot || filepath.Dir(delivery.StagingPath) != plan.TargetAnchor {
		t.Fatalf("Claude transaction paths = owned %q staging %q", delivery.OwnedBase, delivery.StagingPath)
	}
	entries, err := os.ReadDir(delivery.StagingPath)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	want := []string{".agentplugins-runtime", ".claude-plugin", ".mcp.json"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("Claude exposed root = %v, want %v", names, want)
	}
	runtimeRoot := filepath.Join(delivery.StagingPath, ".agentplugins-runtime")
	assertExecutable(t, filepath.Join(runtimeRoot, "bin", "run"))
	assertMissing(t, filepath.Join(runtimeRoot, "skills", "extra"))
	if _, err := os.Stat(filepath.Join(runtimeRoot, "skills", "good", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	document := readObject(t, filepath.Join(delivery.StagingPath, ".mcp.json"))
	local := document["local"].(map[string]any)
	activeRuntime := filepath.Join(plan.ActivePath, ".agentplugins-runtime")
	if local["cwd"] != nil || local["args"].([]any)[3] != activeRuntime {
		t.Fatalf("Claude stdio cwd = %v, want %v", local["cwd"], activeRuntime)
	}
	args := local["args"].([]any)
	if args[7] != filepath.Join(activeRuntime, "bin", "run") {
		t.Fatalf("Claude stdio args = %v", args)
	}
	env := local["env"].(map[string]any)
	if env["PLUGIN_ROOT"] != activeRuntime || env["PLUGIN_DATA"] != ownedData {
		t.Fatalf("Claude stdio env = %v", env)
	}
	if err := (Stager{}).Discard(context.Background(), delivery); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, delivery.StagingPath)
}

func TestClaudeProjectionKeepsMultiTargetStdioContractsIndependent(t *testing.T) {
	envelope := stagingEnvelope(t)
	envelope.MCP.Servers = map[string]domain.MCPServer{"local": {
		Name: "local", Type: "stdio", Decoded: map[string]any{
			"type": "stdio", "command": "node",
			"args": []any{"${PLUGIN_ROOT}/bin/run", "${PLUGIN_DATA}/cache"},
			"env":  map[string]any{"CACHE": "${PLUGIN_DATA}/cache"},
		},
	}}

	type target struct {
		root       string
		pluginRoot string
		dataPath   string
	}
	targets := []target{
		{root: filepath.Join(t.TempDir(), "first"), pluginRoot: filepath.Join(t.TempDir(), "first-runtime"), dataPath: filepath.Join(t.TempDir(), "first-data")},
		{root: filepath.Join(t.TempDir(), "second"), pluginRoot: filepath.Join(t.TempDir(), "second-runtime"), dataPath: filepath.Join(t.TempDir(), "second-data")},
	}
	for _, target := range targets {
		if err := os.MkdirAll(target.root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := projectClaudeMCP(target.root, envelope, []string{"local"}, target.pluginRoot, target.dataPath); err != nil {
			t.Fatal(err)
		}
	}

	for _, target := range targets {
		projected := readObject(t, filepath.Join(target.root, ".mcp.json"))["local"].(map[string]any)
		args := projected["args"].([]any)
		env := projected["env"].(map[string]any)
		if args[7] != filepath.Join(target.pluginRoot, "bin", "run") || args[8] != filepath.Join(target.dataPath, "cache") {
			t.Fatalf("target %s args = %v", target.root, args)
		}
		if env["PLUGIN_ROOT"] != target.pluginRoot || env["PLUGIN_DATA"] != target.dataPath || env["CACHE"] != filepath.Join(target.dataPath, "cache") {
			t.Fatalf("target %s env = %v", target.root, env)
		}
	}

	source := envelope.MCP.Servers["local"].Decoded
	if args := source["args"].([]any); args[0] != "${PLUGIN_ROOT}/bin/run" || args[1] != "${PLUGIN_DATA}/cache" {
		t.Fatalf("portable args mutated across targets: %v", args)
	}
	if env := source["env"].(map[string]any); len(env) != 1 || env["CACHE"] != "${PLUGIN_DATA}/cache" {
		t.Fatalf("portable env mutated across targets: %v", env)
	}
}

func TestStagerProjectsExactOwnedPluginDataContract(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	server := domain.MCPServer{Name: "local", Type: "stdio", Decoded: map[string]any{
		"type": "stdio", "command": "${PLUGIN_ROOT}/must-not-expand",
		"args": []any{"${PLUGIN_ROOT}/bin/run", "${PLUGIN_DATA}/cache"},
		"env":  map[string]any{"CACHE": "${PLUGIN_DATA}/cache"},
		"cwd":  "${PLUGIN_DATA}/work",
	}}
	envelope.MCP.Servers = map[string]domain.MCPServer{"local": server}
	plan := stagingPlan(t, domain.ClientCodex, domain.PackageProjection)
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportProjected}}
	ownedData := filepath.Join(t.TempDir(), "owned-plugin-data")
	if err := os.MkdirAll(filepath.Join(ownedData, "work"), 0700); err != nil {
		t.Fatal(err)
	}
	delivery, err := (Stager{}).StageWithPluginData(context.Background(), envelope, plan, "operation-data", domain.CompatibilityHints{}, ownedData)
	if err != nil {
		t.Fatal(err)
	}
	document := readObject(t, filepath.Join(delivery.StagingPath, ".mcp.json"))
	projected := document["mcpServers"].(map[string]any)["local"].(map[string]any)
	if projected["command"] != "${PLUGIN_ROOT}/must-not-expand" {
		t.Fatalf("stdio command placeholder was expanded: %+v", projected)
	}
	env := projected["env"].(map[string]any)
	if env["PLUGIN_ROOT"] != plan.ActivePath || env["PLUGIN_DATA"] != ownedData || env["CACHE"] != filepath.Join(ownedData, "cache") {
		t.Fatalf("stdio env = %+v", env)
	}
	args := projected["args"].([]any)
	if args[0] != filepath.Join(plan.ActivePath, "bin", "run") || args[1] != filepath.Join(ownedData, "cache") || projected["cwd"] != filepath.Join(ownedData, "work") {
		t.Fatalf("stdio args/cwd = %+v / %v", args, projected["cwd"])
	}
}

func TestStagerProjectsKiroStdioRuntimeContractBeforeNativeImport(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	server := domain.MCPServer{Name: "local", Type: "stdio", Decoded: map[string]any{
		"type": "stdio", "command": "node",
		"args": []any{"${PLUGIN_ROOT}/runtime/launcher.mjs", "${PLUGIN_DATA}/cache"},
		"env":  map[string]any{"CACHE": "${PLUGIN_DATA}/cache"},
	}}
	envelope.MCP.Servers = map[string]domain.MCPServer{"local": server}
	plan := stagingPlan(t, domain.ClientKiro, domain.PackageNative)
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportNative}}
	ownedData := filepath.Join(t.TempDir(), "owned-plugin-data")
	if err := os.MkdirAll(filepath.Join(ownedData, "work"), 0700); err != nil {
		t.Fatal(err)
	}
	delivery, err := (Stager{}).StageWithPluginData(context.Background(), envelope, plan, "operation-kiro-data", domain.CompatibilityHints{}, ownedData)
	if err != nil {
		t.Fatal(err)
	}
	document := readObject(t, filepath.Join(delivery.StagingPath, "mcp.json"))
	projected := document["mcpServers"].(map[string]any)["local"].(map[string]any)
	args := projected["args"].([]any)
	env := projected["env"].(map[string]any)
	if args[0] != filepath.Join(plan.ActivePath, "runtime", "launcher.mjs") || args[1] != filepath.Join(ownedData, "cache") {
		t.Fatalf("Kiro args = %+v", args)
	}
	if env["PLUGIN_ROOT"] != plan.ActivePath || env["PLUGIN_DATA"] != ownedData || env["CACHE"] != filepath.Join(ownedData, "cache") {
		t.Fatalf("Kiro env = %+v", env)
	}
	if projected["cwd"] != plan.ActivePath {
		t.Fatalf("Kiro cwd = %v, want %v", projected["cwd"], plan.ActivePath)
	}
	var foundMCP bool
	for _, object := range delivery.NativeObjects {
		if object.Kind == kiroMCPObjectKind && object.LogicalName == "local" {
			foundMCP = object.Path == filepath.Join(plan.NativeRegistryRoot, "settings", "mcp.json") && strings.HasPrefix(object.ManagedDigest, "sha256:")
		}
	}
	if !foundMCP {
		t.Fatalf("Kiro MCP ownership was not staged: %+v", delivery.NativeObjects)
	}
}

func TestStagerProjectsCursorNativeManifestAndRuntimeContract(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	envelope.Manifest.Author = &domain.Author{Name: "Demo Author", Email: "demo@example.com", URL: "https://example.com/author"}
	server := domain.MCPServer{Name: "local", Type: "stdio", Decoded: map[string]any{
		"type": "stdio", "command": "node",
		"args": []any{"${PLUGIN_ROOT}/runtime/launcher.mjs", "${PLUGIN_DATA}/cache"},
	}}
	envelope.MCP.Servers = map[string]domain.MCPServer{"local": server}
	plan := stagingPlan(t, domain.ClientCursor, domain.PackageNative)
	plan.Components = []domain.ComponentDecision{
		{Kind: domain.ComponentSkill, Name: "good", Support: domain.SupportNative},
		{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportNative},
	}
	ownedData := filepath.Join(t.TempDir(), "owned-plugin-data")
	if err := os.MkdirAll(filepath.Join(ownedData, "work"), 0700); err != nil {
		t.Fatal(err)
	}
	delivery, err := (Stager{}).StageWithPluginData(context.Background(), envelope, plan, "operation-cursor-data", domain.CompatibilityHints{}, ownedData)
	if err != nil {
		t.Fatal(err)
	}
	manifest := readObject(t, filepath.Join(delivery.StagingPath, ".cursor-plugin", "plugin.json"))
	if manifest["name"] != "demo" || manifest["mcpServers"] != "./mcp.json" || manifest["skills"] != "./skills/" {
		t.Fatalf("Cursor manifest = %+v", manifest)
	}
	author := manifest["author"].(map[string]any)
	if author["name"] != "Demo Author" || author["email"] != "demo@example.com" || author["url"] != nil {
		t.Fatalf("Cursor author = %+v", author)
	}
	document := readObject(t, filepath.Join(delivery.StagingPath, "mcp.json"))
	projected := document["mcpServers"].(map[string]any)["local"].(map[string]any)
	args := projected["args"].([]any)
	env := projected["env"].(map[string]any)
	if projected["type"] != nil || args[0] != filepath.Join(plan.ActivePath, "runtime", "launcher.mjs") || args[1] != filepath.Join(ownedData, "cache") {
		t.Fatalf("Cursor MCP = %+v", projected)
	}
	if env["PLUGIN_ROOT"] != plan.ActivePath || env["PLUGIN_DATA"] != ownedData || projected["cwd"] != plan.ActivePath {
		t.Fatalf("Cursor runtime contract = %+v", projected)
	}
}

func TestStagerResolvesBundledStdioCommandForNativeProjection(t *testing.T) {
	t.Parallel()
	config := map[string]any{"command": "./bin/server", "args": []any{"${PLUGIN_ROOT}/config"}}
	pluginRoot, dataPath := filepath.Join(t.TempDir(), "plugin", "${PLUGIN_DATA}"), filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(filepath.Join(pluginRoot, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "bin/server"), []byte("inert"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := applyStdioDataContract(config, pluginRoot, dataPath); err != nil {
		t.Fatal(err)
	}
	if config["command"] != filepath.Join(pluginRoot, "bin", "server") || config["cwd"] != pluginRoot {
		t.Fatalf("projected stdio config = %+v", config)
	}
	if got := config["args"].([]any)[0]; got != filepath.Join(pluginRoot, "config") || strings.Contains(got.(string), dataPath) {
		t.Fatalf("replacement text was recursively expanded: %v", got)
	}
}

func TestStagerBuildsChatGPTAppProjectionWithBundledMCPParity(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	app := `{"apps":{"notion":{"id":"asdk_app_notion_123","required":true}}}`
	writeTestFile(t, filepath.Join(envelope.SnapshotRoot, ".app.json"), app)
	envelope.App = domain.AppComponent{
		Present: true, Declared: true, Enabled: true, Raw: json.RawMessage(app),
		Bindings: map[string]domain.AppBinding{"notion": {Alias: "notion", ID: "asdk_app_notion_123", Required: true}},
	}
	envelope.Inventory.AppPresent = true
	envelope.Inventory.AppBindings = []string{"notion"}
	plan := stagingPlan(t, domain.ClientChatGPT, domain.PackageProjection)
	plan.Components = append(plan.Components, domain.ComponentDecision{Kind: domain.ComponentApp, Name: "notion", Support: domain.SupportProjected})

	delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "operation-chatgpt", domain.CompatibilityHints{
		OpenAIMCPAuth: map[string]domain.OpenAIMCPAuthHint{
			"notion": {OAuthResource: "https://mcp.notion.com"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := readObject(t, filepath.Join(delivery.StagingPath, ".codex-plugin", "plugin.json"))
	if manifest["apps"] != "./.app.json" || manifest["mcpServers"] != "./.mcp.json" || manifest["skills"] != "./skills/" {
		t.Fatalf("ChatGPT manifest = %+v", manifest)
	}
	openAIMCP := readObject(t, filepath.Join(delivery.StagingPath, ".mcp.json"))
	servers := openAIMCP["mcpServers"].(map[string]any)
	notion := servers["notion"].(map[string]any)
	if notion["type"] != "http" || notion["oauth_resource"] != "https://mcp.notion.com" {
		t.Fatalf("ChatGPT MCP parity projection = %+v", notion)
	}
	assertMissing(t, filepath.Join(delivery.StagingPath, "plugin.json"))
	assertMissing(t, filepath.Join(delivery.StagingPath, "mcp.json"))
	body, err := os.ReadFile(filepath.Join(delivery.StagingPath, ".app.json"))
	if err != nil || string(body) != app {
		t.Fatalf("lossless app projection = %q, %v", body, err)
	}
	if source, err := os.ReadFile(filepath.Join(envelope.SnapshotRoot, "mcp.json")); err != nil || len(source) == 0 {
		t.Fatal("portable source MCP was mutated")
	}
}

func TestStagerBuildsManagedCopilotMarketplaceForCopilotAndVSCode(t *testing.T) {
	t.Parallel()
	for _, client := range []domain.ClientID{domain.ClientCopilot, domain.ClientVSCode} {
		client := client
		t.Run(string(client), func(t *testing.T) {
			envelope := stagingEnvelope(t)
			plan := stagingPlan(t, client, domain.PackageNative)
			delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "operation-"+string(client), domain.CompatibilityHints{})
			if err != nil {
				t.Fatal(err)
			}
			marketplace := readObject(t, filepath.Join(delivery.StagingPath, ".github", "plugin", "marketplace.json"))
			if marketplace["name"] != managedMarketplaceName(plan.PhysicalArtifactID) {
				t.Fatalf("marketplace = %+v", marketplace)
			}
			plugins := marketplace["plugins"].([]any)
			plugin := plugins[0].(map[string]any)
			if plugin["name"] != "demo" || plugin["source"] != "." || plugin["version"] != "1.0.0" {
				t.Fatalf("plugin entry = %+v", plugin)
			}
			assertMissing(t, filepath.Join(envelope.SnapshotRoot, ".github"))
		})
	}
}

func TestCopilotMarketplaceVersionDefaultsExactlyOnce(t *testing.T) {
	t.Parallel()
	if got := copilotMarketplaceVersion("  "); got != "0.0.0" {
		t.Fatalf("empty projected version = %q", got)
	}
	if got := copilotMarketplaceVersion(" 1.7.0-uap.1 "); got != "1.7.0-uap.1" {
		t.Fatalf("projected version = %q", got)
	}
}

func TestStagerKeepsNativePackageAndSanitizesFailureBoundaries(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	plan := stagingPlan(t, domain.ClientCursor, domain.PackageNative)
	for index := range plan.Components {
		if plan.Components[index].Kind == domain.ComponentExtension {
			plan.Components[index].Support = domain.SupportNative
		}
	}
	delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "operation-2", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	assertMissing(t, filepath.Join(delivery.StagingPath, ".codex-plugin"))
	manifest := readObject(t, filepath.Join(delivery.StagingPath, "plugin.json"))
	if _, ok := manifest["extensions"]; !ok {
		t.Fatal("supported native extension was removed")
	}
	mcp := readObject(t, filepath.Join(delivery.StagingPath, "mcp.json"))
	servers := mcp["mcpServers"].(map[string]any)
	if len(servers) != 1 || servers["notion"] == nil {
		t.Fatalf("sanitized servers = %+v", servers)
	}
	cursorManifest := readObject(t, filepath.Join(delivery.StagingPath, ".cursor-plugin", "plugin.json"))
	if cursorManifest["name"] != "demo" || cursorManifest["mcpServers"] != "./mcp.json" || cursorManifest["skills"] != "./skills/" {
		t.Fatalf("Cursor compatibility manifest = %+v", cursorManifest)
	}
}

func TestStagerInvalidSkillNamedSkillsDoesNotRemoveSkillsRoot(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	writeTestFile(t, filepath.Join(envelope.SnapshotRoot, "skills", "skills", "SKILL.md"), "invalid\n")
	envelope.Inventory.InvalidSkills = append(envelope.Inventory.InvalidSkills, "skills")
	plan := stagingPlan(t, domain.ClientCursor, domain.PackageNative)
	delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "operation-skill-name", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(delivery.StagingPath, "skills", "good", "SKILL.md")); err != nil {
		t.Fatalf("valid sibling skill was removed: %v", err)
	}
	assertMissing(t, filepath.Join(delivery.StagingPath, "skills", "skills"))
}

func TestStagerRejectsUnsupportedPlanAndExistingOperationStaging(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	plan := stagingPlan(t, domain.ClientKiro, domain.PackageNative)
	plan.Status = domain.PlanUnsupported
	if _, err := (Stager{}).Stage(context.Background(), envelope, plan, "operation-3", domain.CompatibilityHints{}); err == nil {
		t.Fatal("unsupported plan was staged")
	}
	plan.Status = domain.PlanManualActivationRequired
	first, err := (Stager{}).Stage(context.Background(), envelope, plan, "operation-3", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if first.StagingPath == "" {
		t.Fatal("first staging path is empty")
	}
	if _, err := (Stager{}).Stage(context.Background(), envelope, plan, "operation-3", domain.CompatibilityHints{}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate staging error = %v", err)
	}
}

func TestStagerVerifiesAndSafelyDiscardsOnlyItsStagingDirectory(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	plan := stagingPlan(t, domain.ClientCursor, domain.PackageNative)
	for index := range plan.Components {
		if plan.Components[index].Kind == domain.ComponentExtension {
			plan.Components[index].Support = domain.SupportNative
		}
	}
	stager := Stager{}
	delivery, err := stager.Stage(context.Background(), envelope, plan, "operation-4", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if err := stager.Verify(context.Background(), delivery.StagingPath, delivery.ArtifactDigest); err != nil {
		t.Fatalf("verify: %v", err)
	}
	unsafe := delivery
	unsafe.StagingPath = delivery.ActivePath
	if err := stager.Discard(context.Background(), unsafe); err == nil {
		t.Fatal("active path accepted as staging cleanup target")
	}
	if err := stager.Discard(context.Background(), delivery); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, delivery.StagingPath)
}

func TestStagerVerifyRejectsMarkersExcludedFromPortableSnapshotDigest(t *testing.T) {
	t.Parallel()
	for _, marker := range []string{filepath.Join(".git", "config"), filepath.Join("nested", ".plugin-kit-ai.lock")} {
		marker := marker
		t.Run(marker, func(t *testing.T) {
			envelope := stagingEnvelope(t)
			plan := stagingPlan(t, domain.ClientCursor, domain.PackageNative)
			delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "operation-marker-"+strings.ReplaceAll(filepath.Base(marker), ".", "x"), domain.CompatibilityHints{})
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(delivery.StagingPath, marker), "user-owned\n")
			if err := (Stager{}).Verify(context.Background(), delivery.StagingPath, delivery.ArtifactDigest); err == nil || !strings.Contains(err.Error(), "excluded ownership marker") {
				t.Fatalf("verify error = %v", err)
			}
		})
	}
}

func stagingEnvelope(t *testing.T) domain.PackageEnvelope {
	t.Helper()
	root := filepath.Join(t.TempDir(), "snapshot")
	writeTestFile(t, filepath.Join(root, "plugin.json"), `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "demo",
  "version": "1.0.0",
  "description": "Demo plugin",
  "extensions": {"cursor": {"enabled": true}}
}`)
	writeTestFile(t, filepath.Join(root, "mcp.json"), `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
  "mcpServers": {
    "notion": {"type": "streamable-http", "url": "https://mcp.notion.com/mcp"},
    "broken": {"type": "unknown"}
  }
}`)
	writeTestFile(t, filepath.Join(root, "skills", "good", "SKILL.md"), "---\nname: good\ndescription: Good skill\n---\nUse it.\n")
	writeTestFile(t, filepath.Join(root, "skills", "bad", "SKILL.md"), "invalid\n")
	writeTestFile(t, filepath.Join(root, "bin", "run"), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(root, "bin", "run"), 0o755); err != nil {
		t.Fatal(err)
	}
	serverRaw := json.RawMessage(`{"type":"streamable-http","url":"https://mcp.notion.com/mcp"}`)
	return domain.PackageEnvelope{
		LoaderKind: domain.LoaderKindAgentPlugins,
		FormatID:   domain.FormatIDAgentPluginsV1,
		Manifest: domain.PluginManifest{
			Name: "demo", Version: "1.0.0", Description: "Demo plugin",
			Extensions: map[string]json.RawMessage{"cursor": json.RawMessage(`{"enabled":true}`)},
		},
		MCP: domain.MCPComponent{
			Present: true, Enabled: true,
			Servers: map[string]domain.MCPServer{
				"notion": {
					Name: "notion", Type: "streamable-http", Raw: serverRaw,
					Decoded: map[string]any{"type": "streamable-http", "url": "https://mcp.notion.com/mcp"},
				},
			},
		},
		Skills:       map[string]domain.Skill{"good": {Name: "good"}},
		Inventory:    domain.ComponentInventory{InvalidSkills: []string{"bad"}, InvalidMCPServer: []string{"broken"}},
		SnapshotRoot: root,
	}
}

func stagingPlan(t *testing.T, client domain.ClientID, mode domain.PackageMode) domain.DeliveryPlan {
	t.Helper()
	anchor := filepath.Join(t.TempDir(), "managed")
	target := filepath.Join(anchor, "clients", string(client))
	if client == domain.ClientClaude {
		target = filepath.Join(anchor, "skills")
	}
	active := filepath.Join(target, "demo-0123456789ab")
	status := domain.PlanReady
	activation := domain.ActivationActive
	if client == domain.ClientCodex || client == domain.ClientChatGPT || client == domain.ClientKiro {
		status = domain.PlanManualActivationRequired
		activation = domain.ActivationManual
	}
	plan := domain.DeliveryPlan{
		ClientID: client, Scope: domain.ScopeUser, Status: status, PackageMode: mode,
		Activation: activation, Authentication: domain.AuthenticationNotChecked,
		Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid,
		PhysicalArtifactID: "demo-0123456789ab", TargetAnchor: anchor, TargetRoot: target, ActivePath: active,
		Components: []domain.ComponentDecision{
			{Kind: domain.ComponentSkill, Name: "good", Support: supportFor(mode)},
			{Kind: domain.ComponentMCPServer, Name: "notion", Support: supportFor(mode)},
			{Kind: domain.ComponentExtension, Name: "cursor", Support: domain.SupportUnsupported},
		},
	}
	if client == domain.ClientKiro {
		plan.NativeRegistryRoot = filepath.Join(t.TempDir(), ".kiro")
	}
	return plan
}

func supportFor(mode domain.PackageMode) domain.SupportLevel {
	if mode == domain.PackageProjection {
		return domain.SupportProjected
	}
	return domain.SupportNative
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readObject(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("path %s exists or could not be checked: %v", path, err)
	}
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("path %s is not executable", path)
	}
}
