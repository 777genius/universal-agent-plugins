package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableMCPOneConfigProjectsAcrossAgentTargets(t *testing.T) {
	root := RepoRoot(t)
	pluginKitAIBin := buildPluginKitAI(t)
	baseDir := t.TempDir()
	workDir := filepath.Join(baseDir, "portable-mcp-e2e")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWriteRepoFile(t, workDir, authoredRel("plugin.yaml"), `api_version: v1
name: "portable-mcp-e2e"
version: "0.1.0"
description: "portable MCP multi-target e2e"
targets:
  - "codex-package"
  - "gemini"
  - "opencode"
  - "cursor"
`)
	mustWriteRepoFile(t, workDir, authoredRel("targets", "gemini", "package.yaml"), "context_file_name: GEMINI.md\n")
	mustWriteRepoFile(t, workDir, authoredRel("targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteRepoFile(t, workDir, authoredRel("targets", "opencode", "package.yaml"), "plugins:\n  - \"@acme/portable-mcp-e2e\"\n")
	mustWriteRepoFile(t, workDir, authoredRel("mcp", "servers.yaml"), `api_version: v1

servers:
  docs:
    description: Shared remote docs server
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
    targets:
      - codex-package
      - gemini
      - opencode
      - cursor
    overrides:
      gemini:
        excludeTools:
          - delete_docs

  release-checks:
    type: stdio
    stdio:
      command: node
      args:
        - "${package.root}/bin/release-checks.mjs"
    targets:
      - codex-package
      - gemini
      - opencode
      - cursor
`)

	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", workDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", workDir, "--check"))
	for _, platform := range []string{"codex-package", "gemini", "opencode", "cursor"} {
		runCmd(t, root, exec.Command(pluginKitAIBin, "validate", workDir, "--platform", platform, "--strict"))
	}

	codexManifestBody, err := os.ReadFile(filepath.Join(workDir, ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(codexManifestBody), `"mcpServers": "./.mcp.json"`) {
		t.Fatalf("codex package manifest missing shared .mcp.json ref:\n%s", codexManifestBody)
	}

	sharedMCPBody, err := os.ReadFile(filepath.Join(workDir, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sharedMCP map[string]map[string]any
	if err := json.Unmarshal(sharedMCPBody, &sharedMCP); err != nil {
		t.Fatalf("parse shared .mcp.json: %v\n%s", err, sharedMCPBody)
	}
	if _, ok := sharedMCP["docs"]; !ok {
		t.Fatalf("shared .mcp.json missing docs server:\n%s", sharedMCPBody)
	}
	sharedChecks, ok := sharedMCP["release-checks"]
	if !ok {
		t.Fatalf("shared .mcp.json missing release-checks server:\n%s", sharedMCPBody)
	}
	sharedArgs, _ := sharedChecks["args"].([]any)
	if len(sharedArgs) != 1 || sharedArgs[0] != "./bin/release-checks.mjs" {
		t.Fatalf("shared .mcp.json release-checks args = %#v", sharedChecks["args"])
	}

	geminiBody, err := os.ReadFile(filepath.Join(workDir, "gemini-extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	var geminiDoc map[string]any
	if err := json.Unmarshal(geminiBody, &geminiDoc); err != nil {
		t.Fatalf("parse gemini-extension.json: %v\n%s", err, geminiBody)
	}
	geminiMCP, ok := geminiDoc["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("gemini-extension.json missing mcpServers:\n%s", geminiBody)
	}
	geminiDocs, ok := geminiMCP["docs"].(map[string]any)
	if !ok || geminiDocs["httpUrl"] != "https://example.com/mcp" {
		t.Fatalf("gemini docs projection = %#v", geminiMCP["docs"])
	}
	geminiChecks, ok := geminiMCP["release-checks"].(map[string]any)
	if !ok {
		t.Fatalf("gemini release-checks projection missing:\n%s", geminiBody)
	}
	args, _ := geminiChecks["args"].([]any)
	if len(args) != 1 || args[0] != "${extensionPath}/bin/release-checks.mjs" {
		t.Fatalf("gemini release-checks args = %#v", geminiChecks["args"])
	}

	opencodeBody, err := os.ReadFile(filepath.Join(workDir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var opencodeDoc struct {
		MCP map[string]map[string]any `json:"mcp"`
	}
	if err := json.Unmarshal(opencodeBody, &opencodeDoc); err != nil {
		t.Fatalf("parse opencode.json: %v\n%s", err, opencodeBody)
	}
	if opencodeDoc.MCP["docs"]["type"] != "remote" {
		t.Fatalf("opencode docs projection = %#v", opencodeDoc.MCP["docs"])
	}
	if opencodeDoc.MCP["release-checks"]["type"] != "local" {
		t.Fatalf("opencode release-checks projection = %#v", opencodeDoc.MCP["release-checks"])
	}

	cursorManifestBody, err := os.ReadFile(filepath.Join(workDir, ".cursor-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cursorManifestBody), `"mcpServers": "./.mcp.json"`) {
		t.Fatalf(".cursor-plugin/plugin.json missing shared MCP ref:\n%s", cursorManifestBody)
	}
	cursorBody, err := os.ReadFile(filepath.Join(workDir, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cursorDoc map[string]map[string]any
	if err := json.Unmarshal(cursorBody, &cursorDoc); err != nil {
		t.Fatalf("parse .mcp.json: %v\n%s", err, cursorBody)
	}
	cursorDocs := cursorDoc["docs"]
	if cursorDocs["type"] != "http" || cursorDocs["url"] != "https://example.com/mcp" {
		t.Fatalf("cursor docs projection = %#v", cursorDocs)
	}
	cursorChecks := cursorDoc["release-checks"]
	cursorArgs, _ := cursorChecks["args"].([]any)
	if len(cursorArgs) != 1 || cursorArgs[0] != "./bin/release-checks.mjs" {
		t.Fatalf("cursor release-checks args = %#v", cursorChecks["args"])
	}
}

func mustWriteRepoFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWriteRepoExecutable(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}
