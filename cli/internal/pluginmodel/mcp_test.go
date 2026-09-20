package pluginmodel

import (
	"strings"
	"testing"
)

func TestParsePortableMCPStandardProjectsAcrossTargets(t *testing.T) {
	t.Parallel()
	parsed, err := ParsePortableMCP("mcp/servers.yaml", []byte(`api_version: v1

servers:
  docs:
    description: Remote docs
    type: remote
    remote:
      protocol: http
      url: "https://example.com/mcp"
      headers:
        Authorization: "Bearer ${env.DOCS_TOKEN}"
    targets:
      - claude
      - codex-package
      - gemini
      - opencode
      - cursor
    overrides:
      gemini:
        excludeTools:
          - delete_docs
      codex-package:
        startup_timeout_sec: 10

  local-tools:
    type: stdio
    stdio:
      command: node
      args:
        - "${package.root}/server.mjs"
      env:
        TOKEN: "${env.LOCAL_TOKEN}"
    targets:
      - opencode
      - cursor
      - cursor-workspace
`))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.File == nil {
		t.Fatal("expected parsed portable MCP file")
	}
	mcp := &PortableMCP{Path: "mcp/servers.yaml", Servers: parsed.Servers, File: parsed.File}

	gemini, err := mcp.RenderForTarget("gemini")
	if err != nil {
		t.Fatal(err)
	}
	docs, ok := gemini["docs"].(map[string]any)
	if !ok {
		t.Fatalf("gemini docs = %#v", gemini["docs"])
	}
	if _, ok := docs["httpUrl"].(string); !ok {
		t.Fatalf("gemini docs missing httpUrl: %#v", docs)
	}
	if _, ok := docs["excludeTools"].([]any); !ok {
		t.Fatalf("gemini docs missing excludeTools override: %#v", docs)
	}
	if _, ok := gemini["local-tools"]; ok {
		t.Fatalf("gemini projection unexpectedly included local-tools: %#v", gemini)
	}

	opencode, err := mcp.RenderForTarget("opencode")
	if err != nil {
		t.Fatal(err)
	}
	local, ok := opencode["local-tools"].(map[string]any)
	if !ok {
		t.Fatalf("opencode local-tools = %#v", opencode["local-tools"])
	}
	if got, _ := local["type"].(string); got != "local" {
		t.Fatalf("opencode local type = %q, want local", got)
	}
	command, ok := local["command"].([]any)
	if !ok || len(command) != 2 || command[0] != "node" || command[1] != "./server.mjs" {
		t.Fatalf("opencode local command = %#v", local["command"])
	}
	if _, ok := local["environment"].(map[string]any); !ok {
		t.Fatalf("opencode local missing environment: %#v", local)
	}

	cursor, err := mcp.RenderForTarget("cursor")
	if err != nil {
		t.Fatal(err)
	}
	cursorLocal, ok := cursor["local-tools"].(map[string]any)
	if !ok {
		t.Fatalf("cursor local-tools = %#v", cursor["local-tools"])
	}
	cursorArgs, ok := cursorLocal["args"].([]any)
	if !ok || len(cursorArgs) != 1 || cursorArgs[0] != "./server.mjs" {
		t.Fatalf("cursor local-tools args = %#v", cursorLocal["args"])
	}
	cursorWorkspace, err := mcp.RenderForTarget("cursor-workspace")
	if err != nil {
		t.Fatal(err)
	}
	cursorWorkspaceLocal, ok := cursorWorkspace["local-tools"].(map[string]any)
	if !ok {
		t.Fatalf("cursor-workspace local-tools = %#v", cursorWorkspace["local-tools"])
	}
	cursorWorkspaceArgs, ok := cursorWorkspaceLocal["args"].([]any)
	if !ok || len(cursorWorkspaceArgs) != 1 || cursorWorkspaceArgs[0] != "${workspaceFolder}/server.mjs" {
		t.Fatalf("cursor-workspace local-tools args = %#v", cursorWorkspaceLocal["args"])
	}

	claude, err := mcp.RenderForTarget("claude")
	if err != nil {
		t.Fatal(err)
	}
	claudeDocs, ok := claude["docs"].(map[string]any)
	if !ok {
		t.Fatalf("claude docs = %#v", claude["docs"])
	}
	if got, _ := claudeDocs["type"].(string); got != "http" {
		t.Fatalf("claude docs type = %q, want http", got)
	}

	codex, err := mcp.RenderForTarget("codex-package")
	if err != nil {
		t.Fatal(err)
	}
	codexDocs, ok := codex["docs"].(map[string]any)
	if !ok {
		t.Fatalf("codex docs = %#v", codex["docs"])
	}
	if got, _ := codexDocs["startup_timeout_sec"].(int); got != 10 {
		t.Fatalf("codex docs startup_timeout_sec = %#v", codexDocs["startup_timeout_sec"])
	}
}

func TestParsePortableMCPStandardRejectsInvalidAlias(t *testing.T) {
	t.Parallel()
	_, err := ParsePortableMCP("mcp/servers.yaml", []byte(`api_version: v1
servers:
  bad_alias:
    type: stdio
    stdio:
      command: node
`))
	if err == nil {
		t.Fatal("expected invalid alias error")
	}
}

func TestParsePortableMCPContext7StdioAcrossFiveTargets(t *testing.T) {
	t.Parallel()
	parsed, err := ParsePortableMCP("mcp/servers.yaml", []byte(`api_version: v1

servers:
  context7:
    type: stdio
    stdio:
      command: npx
      args:
        - -y
        - "@upstash/context7-mcp@2.1.6"
    targets:
      - claude
      - codex-package
      - gemini
      - opencode
      - cursor
`))
	if err != nil {
		t.Fatal(err)
	}
	mcp := &PortableMCP{Path: "mcp/servers.yaml", Servers: parsed.Servers, File: parsed.File}

	for _, target := range []string{"claude", "codex-package", "gemini", "cursor"} {
		projected, err := mcp.RenderForTarget(target)
		if err != nil {
			t.Fatalf("RenderForTarget(%q): %v", target, err)
		}
		server, ok := projected["context7"].(map[string]any)
		if !ok {
			t.Fatalf("%s context7 = %#v", target, projected["context7"])
		}
		if got, _ := server["command"].(string); got != "npx" {
			t.Fatalf("%s command = %q, want npx", target, got)
		}
		args, ok := server["args"].([]any)
		if !ok || len(args) != 2 || args[0] != "-y" || args[1] != "@upstash/context7-mcp@2.1.6" {
			t.Fatalf("%s args = %#v", target, server["args"])
		}
	}

	opencode, err := mcp.RenderForTarget("opencode")
	if err != nil {
		t.Fatal(err)
	}
	server, ok := opencode["context7"].(map[string]any)
	if !ok {
		t.Fatalf("opencode context7 = %#v", opencode["context7"])
	}
	if got, _ := server["type"].(string); got != "local" {
		t.Fatalf("opencode type = %q, want local", got)
	}
	command, ok := server["command"].([]any)
	if !ok || len(command) != 3 || command[0] != "npx" || command[1] != "-y" || command[2] != "@upstash/context7-mcp@2.1.6" {
		t.Fatalf("opencode command = %#v", server["command"])
	}
}

func TestParsePortableMCPStandardRejectsUnsupportedOverrideTarget(t *testing.T) {
	t.Parallel()
	_, err := ParsePortableMCP("mcp/servers.yaml", []byte(`api_version: v1
servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
    overrides:
      unknown-target:
        excludeTools:
          - delete_docs
`))
	if err == nil {
		t.Fatal("expected unsupported override target error")
	}
}

func TestParsePortableMCPStandardRejectsManagedOverrideConflict(t *testing.T) {
	t.Parallel()
	_, err := ParsePortableMCP("mcp/servers.yaml", []byte(`api_version: v1
servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
    overrides:
      gemini:
        httpUrl: "https://override.example.com/mcp"
`))
	if err == nil {
		t.Fatal("expected managed override conflict error")
	}
}

func TestParsePortableMCPStandardRejectsManagedPassthroughConflict(t *testing.T) {
	t.Parallel()
	_, err := ParsePortableMCP("mcp/servers.yaml", []byte(`api_version: v1
servers:
  local-tools:
    type: stdio
    stdio:
      command: node
    passthrough:
      opencode:
        command:
          - node
          - server.mjs
`))
	if err == nil {
		t.Fatal("expected managed passthrough conflict error")
	}
}

func TestParsePortableMCPLegacyFormatVersionStillLoads(t *testing.T) {
	t.Parallel()
	parsed, err := ParsePortableMCP("mcp/servers.yaml", []byte(`format: plugin-kit-ai/mcp
version: 1
servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
`))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.File == nil {
		t.Fatal("expected parsed portable MCP file")
	}
	if parsed.File.APIVersion != "" {
		t.Fatalf("legacy parse should preserve empty api_version, got %q", parsed.File.APIVersion)
	}
}

func TestParsePortableMCPRejectsMixedSchemaMarkers(t *testing.T) {
	t.Parallel()
	_, err := ParsePortableMCP("mcp/servers.yaml", []byte(`api_version: v1
format: plugin-kit-ai/mcp
version: 1
servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
`))
	if err == nil {
		t.Fatal("expected mixed schema marker error")
	}
}

func TestPortableMCPFileFromNativeOpenCodeLocalPreservesPassthrough(t *testing.T) {
	t.Parallel()
	file, err := PortableMCPFileFromNative("opencode", map[string]any{
		"docs": map[string]any{
			"type":        "local",
			"command":     []any{"node", "server.mjs"},
			"environment": map[string]any{"TOKEN": "${env.DOCS_TOKEN}"},
			"cwd":         "./tools",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := file.Servers["docs"]
	if server.Type != "stdio" || server.Stdio == nil {
		t.Fatalf("server = %#v", server)
	}
	if len(server.Targets) != 1 || server.Targets[0] != "opencode" {
		t.Fatalf("targets = %#v", server.Targets)
	}
	if got := server.Passthrough["opencode"]["cwd"]; got != "./tools" {
		t.Fatalf("passthrough cwd = %#v", got)
	}
}

func TestImportedPortableMCPYAMLCanonicalizesAPIVersionV1(t *testing.T) {
	t.Parallel()
	body, err := ImportedPortableMCPYAML("gemini", map[string]any{
		"docs": map[string]any{
			"url": "https://example.com/mcp",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(body)
	if !strings.Contains(rendered, "api_version: v1") {
		t.Fatalf("missing api_version in rendered YAML:\n%s", rendered)
	}
	if strings.Contains(rendered, "format:") || strings.Contains(rendered, "version: 1") {
		t.Fatalf("rendered YAML still contains legacy markers:\n%s", rendered)
	}
	if !strings.Contains(rendered, "protocol: sse") {
		t.Fatalf("gemini import did not normalize protocol:\n%s", rendered)
	}
}
