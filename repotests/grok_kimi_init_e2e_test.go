package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestPluginKitAIInitGrokPackagedRoundTrip drives the packaged Grok lane the
// way the other init round-trips do: init --extras authors portable MCP,
// generate --check proves the rendered artifacts are current, and
// validate --strict passes. Grok wraps the portable MCP projection into a
// managed .mcp.json sidecar referenced from .grok-plugin/plugin.json.
func TestPluginKitAIInitGrokPackagedRoundTrip(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.CommandContext(context.Background(), pluginKitAIBin, "init", "demo", "--platform", "grok", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
	}

	env := append(os.Environ(), "GOWORK=off")
	generateCheck := exec.CommandContext(context.Background(), pluginKitAIBin, "generate", plugRoot, "--check")
	generateCheck.Env = env
	if out, err := generateCheck.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai generate --check: %v\n%s", err, out)
	}
	validateStrictProject(t, pluginKitAIBin, plugRoot, "grok", "grok init validate --strict", env)

	manifest := readInitManifest(t, filepath.Join(plugRoot, ".grok-plugin", "plugin.json"))
	if manifest["name"] != "demo" {
		t.Fatalf(".grok-plugin/plugin.json name = %v", manifest["name"])
	}
	if manifest["mcpServers"] != "./.mcp.json" {
		t.Fatalf(".grok-plugin/plugin.json must reference the managed MCP sidecar: %+v", manifest)
	}
	if manifest["skills"] != "./skills/" {
		t.Fatalf(".grok-plugin/plugin.json skills = %+v", manifest["skills"])
	}

	sidecar := readInitManifest(t, filepath.Join(plugRoot, ".mcp.json"))
	servers, ok := sidecar["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("generated Grok .mcp.json must wrap servers in mcpServers:\n%v", sidecar)
	}
	docs, ok := servers["docs"].(map[string]any)
	if !ok {
		t.Fatalf("generated Grok .mcp.json missing docs server:\n%v", sidecar)
	}
	if docs["type"] != "http" || docs["url"] != "https://example.com/mcp" {
		t.Fatalf("Grok docs MCP projection = %+v", docs)
	}
	if _, err := os.Stat(filepath.Join(plugRoot, "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("stat generated grok skill: %v", err)
	}
}

// TestPluginKitAIInitKimiPackagedRoundTrip drives the packaged Kimi lane: Kimi
// requires MCP servers inline in the managed manifest and never generates an
// .mcp.json sidecar.
func TestPluginKitAIInitKimiPackagedRoundTrip(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.CommandContext(context.Background(), pluginKitAIBin, "init", "demo", "--platform", "kimi", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
	}

	env := append(os.Environ(), "GOWORK=off")
	generateCheck := exec.CommandContext(context.Background(), pluginKitAIBin, "generate", plugRoot, "--check")
	generateCheck.Env = env
	if out, err := generateCheck.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai generate --check: %v\n%s", err, out)
	}
	validateStrictProject(t, pluginKitAIBin, plugRoot, "kimi", "kimi init validate --strict", env)

	manifest := readInitManifest(t, filepath.Join(plugRoot, ".kimi-plugin", "plugin.json"))
	if manifest["name"] != "demo" {
		t.Fatalf(".kimi-plugin/plugin.json name = %v", manifest["name"])
	}
	if manifest["skills"] != "./skills/" {
		t.Fatalf(".kimi-plugin/plugin.json skills = %+v", manifest["skills"])
	}
	servers, ok := manifest["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("Kimi manifest MCP must be inline, not a file ref:\n%v", manifest)
	}
	docs, ok := servers["docs"].(map[string]any)
	if !ok {
		t.Fatalf("Kimi manifest missing inline docs server:\n%v", manifest)
	}
	if docs["type"] != "http" || docs["url"] != "https://example.com/mcp" {
		t.Fatalf("Kimi docs MCP projection = %+v", docs)
	}
	if _, err := os.Stat(filepath.Join(plugRoot, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("Kimi must not generate an MCP sidecar; stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(plugRoot, "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("stat generated kimi skill: %v", err)
	}
}

func readInitManifest(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse %s: %v\n%s", path, err, body)
	}
	return doc
}
