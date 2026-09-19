package kiro

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestKiroNativeLifecyclePreservesUnmanagedConfiguration(t *testing.T) {
	t.Parallel()
	configRoot := filepath.Join(t.TempDir(), ".kiro")
	mcpPath := filepath.Join(configRoot, "settings", "mcp.json")
	writeTestFile(t, mcpPath, `{
  "telemetry": {"enabled": false},
  "mcpServers": {"unmanaged": {"url": "https://example.test/mcp"}}
}`)

	activeV1, desiredV1 := kiroNativeFixture(t, configRoot, "v1", "https://docs.example.test/v1")
	if err := applyKiroNativeMutation(configRoot, activeV1, nil, desiredV1); err != nil {
		t.Fatal(err)
	}
	assertKiroNativeState(t, configRoot, "v1", "https://docs.example.test/v1", true)

	activeV2, desiredV2 := kiroNativeFixture(t, configRoot, "v2", "https://docs.example.test/v2")
	if err := applyKiroNativeMutation(configRoot, activeV2, desiredV1, desiredV2); err != nil {
		t.Fatal(err)
	}
	assertKiroNativeState(t, configRoot, "v2", "https://docs.example.test/v2", true)

	if err := applyKiroNativeMutation(configRoot, "", desiredV2, nil); err != nil {
		t.Fatal(err)
	}
	assertKiroNativeState(t, configRoot, "", "", false)
}

func TestKiroNativeLifecycleRejectsUnmanagedCollisionWithoutMutation(t *testing.T) {
	t.Parallel()
	configRoot := filepath.Join(t.TempDir(), ".kiro")
	skillPath := filepath.Join(configRoot, "skills", "docs")
	writeTestFile(t, filepath.Join(skillPath, "SKILL.md"), "unmanaged\n")
	active, desired := kiroNativeFixture(t, configRoot, "managed", "https://docs.example.test/managed")
	if err := applyKiroNativeMutation(configRoot, active, nil, desired); err == nil {
		t.Fatal("unmanaged Kiro skill collision was accepted")
	}
	body, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	if err != nil || string(body) != "unmanaged\n" {
		t.Fatalf("unmanaged skill changed: %q, %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(configRoot, "settings", "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("MCP config was mutated after collision: %v", err)
	}
}

func TestKiroNativeRemovalRefusesUserModifiedOwnedSkill(t *testing.T) {
	t.Parallel()
	configRoot := filepath.Join(t.TempDir(), ".kiro")
	active, desired := kiroNativeFixture(t, configRoot, "owned", "https://docs.example.test/owned")
	if err := applyKiroNativeMutation(configRoot, active, nil, desired); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(configRoot, "skills", "docs", "SKILL.md"), "user modification\n")
	if err := applyKiroNativeMutation(configRoot, "", desired, nil); err == nil {
		t.Fatal("modified managed skill was silently removed")
	}
	body, err := os.ReadFile(filepath.Join(configRoot, "skills", "docs", "SKILL.md"))
	if err != nil || string(body) != "user modification\n" {
		t.Fatalf("modified skill was not retained: %q, %v", body, err)
	}
}

func kiroNativeFixture(t *testing.T, configRoot, marker, url string) (string, []domain.NativeObjectOwnership) {
	t.Helper()
	active := filepath.Join(t.TempDir(), "active")
	skillRoot := filepath.Join(active, "skills", "docs")
	writeTestFile(t, filepath.Join(skillRoot, "SKILL.md"), marker+"\n")
	digest, err := shared.DigestSkillDirectory(skillRoot)
	if err != nil {
		t.Fatal(err)
	}
	server := map[string]any{"type": "streamable-http", "url": url}
	body, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"docs": server}})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(active, "mcp.json"), string(body))
	return active, []domain.NativeObjectOwnership{
		{ObjectID: "kiro-skill:docs", Kind: kiroSkillObjectKind, LogicalName: "docs", Path: filepath.Join(configRoot, "skills", "docs"), SourceRelative: "skills/docs", ManagedDigest: digest, ProtectionClass: "managed"},
		{ObjectID: "kiro-mcp:docs", Kind: kiroMCPObjectKind, LogicalName: "docs", Path: filepath.Join(configRoot, "settings", "mcp.json"), ManagedDigest: shared.DigestJSONObject(server), ProtectionClass: "managed"},
	}
}

func assertKiroNativeState(t *testing.T, configRoot, skillMarker, managedURL string, managedPresent bool) {
	t.Helper()
	skillPath := filepath.Join(configRoot, "skills", "docs", "SKILL.md")
	if managedPresent {
		body, err := os.ReadFile(skillPath)
		if err != nil || string(body) != skillMarker+"\n" {
			t.Fatalf("Kiro skill = %q, %v", body, err)
		}
	} else if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Fatalf("removed Kiro skill still exists: %v", err)
	}
	document := readObject(t, filepath.Join(configRoot, "settings", "mcp.json"))
	if telemetry, ok := document["telemetry"].(map[string]any); !ok || telemetry["enabled"] != false {
		t.Fatalf("unmanaged top-level Kiro config was not preserved: %+v", document)
	}
	servers := document["mcpServers"].(map[string]any)
	if _, ok := servers["unmanaged"]; !ok {
		t.Fatalf("unmanaged MCP server was not preserved: %+v", servers)
	}
	managed, exists := servers["docs"].(map[string]any)
	if exists != managedPresent {
		t.Fatalf("managed MCP presence = %v, want %v", exists, managedPresent)
	}
	if managedPresent && managed["url"] != managedURL {
		t.Fatalf("managed MCP URL = %v, want %v", managed["url"], managedURL)
	}
}
