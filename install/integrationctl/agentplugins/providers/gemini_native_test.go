package providers

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers/nativeconfig"
)

func TestGeminiSkillRollbackRetainsOnlyBackupWhenRestoreRenameFails(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), ".gemini")
	activeV1, desiredV1 := geminiNativeFixture(t, configRoot, "v1", "https://docs.test/v1")
	if err := applyGeminiNativeMutation(configRoot, activeV1, nil, desiredV1); err != nil {
		t.Fatal(err)
	}
	activeV2, desiredV2 := geminiNativeFixture(t, configRoot, "v2", "https://docs.test/v2")
	activationErr := errors.New("injected Gemini activation rename failure")
	restoreErr := errors.New("injected Gemini restore rename failure")
	rename := func(oldPath, newPath string) error {
		switch {
		case strings.HasPrefix(filepath.Base(oldPath), "new-"):
			return activationErr
		case strings.HasPrefix(filepath.Base(oldPath), "old-"):
			return restoreErr
		default:
			return os.Rename(oldPath, newPath)
		}
	}

	err := applyGeminiNativeMutationWithRename(configRoot, activeV2, desiredV1, desiredV2, rename)
	if err == nil || !strings.Contains(err.Error(), activationErr.Error()) || !strings.Contains(err.Error(), restoreErr.Error()) || !strings.Contains(err.Error(), "recovery retained at") {
		t.Fatalf("rollback error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(configRoot, "skills", "docs")); !os.IsNotExist(statErr) {
		t.Fatalf("failed restore unexpectedly recreated target: %v", statErr)
	}
	transactions, globErr := filepath.Glob(filepath.Join(configRoot, "skills", ".agentplugins-native-*"))
	if globErr != nil || len(transactions) != 1 {
		t.Fatalf("retained Gemini transactions = %v, %v", transactions, globErr)
	}
	backup := filepath.Join(transactions[0], "old-docs", "SKILL.md")
	body, readErr := os.ReadFile(backup)
	if readErr != nil || string(body) != "v1\n" {
		t.Fatalf("recoverable Gemini backup = %q, %v", body, readErr)
	}
}

func TestGeminiSkillBackupDigestMismatchRestoresLiveDirectoryAndAborts(t *testing.T) {
	configRoot := filepath.Join(t.TempDir(), ".gemini")
	activeV1, desiredV1 := geminiNativeFixture(t, configRoot, "v1", "https://docs.test/v1")
	if err := applyGeminiNativeMutation(configRoot, activeV1, nil, desiredV1); err != nil {
		t.Fatal(err)
	}
	activeV2, desiredV2 := geminiNativeFixture(t, configRoot, "v2", "https://docs.test/v2")
	liveSkill := filepath.Join(configRoot, "skills", "docs")
	rename := func(oldPath, newPath string) error {
		if sameCleanPath(oldPath, liveSkill) && strings.HasPrefix(filepath.Base(newPath), "old-") {
			writeTestFile(t, filepath.Join(oldPath, "SKILL.md"), "concurrent user change\n")
		}
		return os.Rename(oldPath, newPath)
	}

	err := applyGeminiNativeMutationWithRename(configRoot, activeV2, desiredV1, desiredV2, rename)
	if err == nil || !strings.Contains(err.Error(), "isolated Gemini skill backup") {
		t.Fatalf("TOCTOU activation error = %v", err)
	}
	body, readErr := os.ReadFile(filepath.Join(liveSkill, "SKILL.md"))
	if readErr != nil || string(body) != "concurrent user change\n" {
		t.Fatalf("restored live Gemini skill = %q, %v", body, readErr)
	}
	document := readObject(t, filepath.Join(configRoot, "settings.json"))
	server := document["mcpServers"].(map[string]any)["docs"].(map[string]any)
	if server["httpUrl"] != "https://docs.test/v1" {
		t.Fatalf("MCP configuration changed after backup mismatch: %+v", server)
	}
}

func TestRenameGeminiDirectoryNoReplacePreservesLateTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "staged")
	target := filepath.Join(root, "live")
	writeTestFile(t, filepath.Join(source, "SKILL.md"), "managed\n")
	writeTestFile(t, filepath.Join(target, "SKILL.md"), "late unmanaged\n")
	renameCalls := 0
	err := renameGeminiDirectoryNoReplace(source, target, func(oldPath, newPath string) error {
		renameCalls++
		return os.Rename(oldPath, newPath)
	})
	if err == nil || renameCalls != 0 {
		t.Fatalf("no-replace result = %v, rename calls = %d", err, renameCalls)
	}
	body, readErr := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if readErr != nil || string(body) != "late unmanaged\n" {
		t.Fatalf("late target = %q, %v", body, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(source, "SKILL.md")); statErr != nil {
		t.Fatalf("staged source was consumed: %v", statErr)
	}
}

func TestRenameDirectoryExclusiveDoesNotReplaceExistingDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "staged")
	target := filepath.Join(root, "live")
	writeTestFile(t, filepath.Join(source, "SKILL.md"), "managed\n")
	writeTestFile(t, filepath.Join(target, "SKILL.md"), "unmanaged\n")
	if err := renameDirectoryExclusive(source, target); err == nil {
		t.Fatal("exclusive rename replaced an existing target")
	}
	body, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil || string(body) != "unmanaged\n" {
		t.Fatalf("existing target = %q, %v", body, err)
	}
}

func TestGeminiNativeLifecyclePreservesUnmanagedConfiguration(t *testing.T) {
	t.Parallel()
	configRoot := filepath.Join(t.TempDir(), ".gemini")
	writeTestFile(t, filepath.Join(configRoot, "settings.json"), `{"theme":"night","mcpServers":{"unmanaged":{"url":"https://foreign.test/sse"}}}`)
	activeV1, desiredV1 := geminiNativeFixture(t, configRoot, "v1", "https://docs.test/v1")
	if err := applyGeminiNativeMutation(configRoot, activeV1, nil, desiredV1); err != nil {
		t.Fatal(err)
	}
	assertGeminiNativeState(t, configRoot, "v1", "https://docs.test/v1", true)
	activeV2, desiredV2 := geminiNativeFixture(t, configRoot, "v2", "https://docs.test/v2")
	if err := applyGeminiNativeMutation(configRoot, activeV2, desiredV1, desiredV2); err != nil {
		t.Fatal(err)
	}
	assertGeminiNativeState(t, configRoot, "v2", "https://docs.test/v2", true)

	// Repair a managed entry removed outside the manager.
	if _, err := nativeconfig.New().Apply(nativeconfig.Request{Paths: geminiConfigPaths(configRoot), Codec: nativeconfig.CodecGemini, Action: nativeconfig.ActionRemove, Name: "docs", Owned: geminiReceipt(desiredV2[1])}); err != nil {
		t.Fatal(err)
	}
	if err := applyGeminiNativeMutation(configRoot, activeV2, desiredV2, desiredV2); err != nil {
		t.Fatal(err)
	}
	assertGeminiNativeState(t, configRoot, "v2", "https://docs.test/v2", true)
	if err := applyGeminiNativeMutation(configRoot, "", desiredV2, nil); err != nil {
		t.Fatal(err)
	}
	assertGeminiNativeState(t, configRoot, "", "", false)
}

func TestGeminiNativeLifecycleRejectsCollisionsAndUserChanges(t *testing.T) {
	t.Parallel()
	configRoot := filepath.Join(t.TempDir(), ".gemini")
	writeTestFile(t, filepath.Join(configRoot, "skills", "docs", "SKILL.md"), "unmanaged\n")
	active, desired := geminiNativeFixture(t, configRoot, "owned", "https://docs.test")
	if err := applyGeminiNativeMutation(configRoot, active, nil, desired); err == nil {
		t.Fatal("unmanaged Gemini skill collision was accepted")
	}
	body, _ := os.ReadFile(filepath.Join(configRoot, "skills", "docs", "SKILL.md"))
	if string(body) != "unmanaged\n" {
		t.Fatalf("unmanaged Gemini skill changed: %q", body)
	}

	configRoot = filepath.Join(t.TempDir(), ".gemini")
	active, desired = geminiNativeFixture(t, configRoot, "owned", "https://docs.test")
	if err := applyGeminiNativeMutation(configRoot, active, nil, desired); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(configRoot, "skills", "docs", "SKILL.md"), "user change\n")
	if err := applyGeminiNativeMutation(configRoot, "", desired, nil); err == nil {
		t.Fatal("modified managed Gemini skill was silently removed")
	}
	body, _ = os.ReadFile(filepath.Join(configRoot, "skills", "docs", "SKILL.md"))
	if string(body) != "user change\n" {
		t.Fatalf("modified Gemini skill was not retained: %q", body)
	}
}

func TestGeminiTransportProjection(t *testing.T) {
	t.Parallel()
	stdio, err := geminiNativeServer(domain.MCPServer{Name: "local", Type: "stdio", Decoded: map[string]any{"type": "stdio", "command": "${PLUGIN_ROOT}", "args": []any{"${PLUGIN_ROOT}/server.js", "${PLUGIN_CACHE}"}, "env": map[string]any{"DATA": "${PLUGIN_DATA}", "UNKNOWN": "${HOME}"}, "cwd": "./${PLUGIN_CACHE}"}})
	if err != nil || stdio.CWD != "${PLUGIN_ROOT}/${PLUGIN_CACHE}" {
		t.Fatalf("stdio projection = %+v, %v", stdio, err)
	}
	packageRoot := filepath.Join(t.TempDir(), "plugin", "${PLUGIN_DATA}")
	if err := os.MkdirAll(filepath.Join(packageRoot, "${PLUGIN_CACHE}"), 0700); err != nil {
		t.Fatal(err)
	}
	dataRoot := filepath.Join(t.TempDir(), "data")
	stdio, err = materializeGeminiServer(stdio, packageRoot, dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	if stdio.Command != "${PLUGIN_ROOT}" || stdio.CWD != filepath.Join(packageRoot, "${PLUGIN_CACHE}") {
		t.Fatalf("Gemini command/cwd projection = %+v", stdio)
	}
	if strings.Contains(stdio.CWD, dataRoot) {
		t.Fatalf("Gemini recursively expanded replacement text: %+v", stdio)
	}
	for _, transport := range []string{"streamable-http", "sse"} {
		server, err := geminiNativeServer(domain.MCPServer{Name: "remote", Type: transport, Decoded: map[string]any{"type": transport, "url": "https://example.test/${PLUGIN_ROOT}", "headers": map[string]any{"X-Path": "${PLUGIN_DATA}"}}})
		server, materializeErr := materializeGeminiServer(server, packageRoot, dataRoot)
		if err != nil || materializeErr != nil || server.Type != "remote" || server.RemoteTransport != transport || server.URL != "https://example.test/${PLUGIN_ROOT}" || server.Headers["X-Path"] != "${PLUGIN_DATA}" {
			t.Fatalf("%s projection = %+v, %v", transport, server, err)
		}
	}
}

func geminiNativeFixture(t *testing.T, configRoot, marker, url string) (string, []domain.NativeObjectOwnership) {
	t.Helper()
	active := filepath.Join(t.TempDir(), "active")
	skillRoot := filepath.Join(active, "skills", "docs")
	writeTestFile(t, filepath.Join(skillRoot, "SKILL.md"), marker+"\n")
	digest, err := digestKiroSkillDirectory(skillRoot)
	if err != nil {
		t.Fatal(err)
	}
	mcp := map[string]any{"type": "streamable-http", "url": url}
	body, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"docs": mcp}})
	writeTestFile(t, filepath.Join(active, "mcp.json"), string(body))
	dataRoot := filepath.Join(t.TempDir(), "data")
	descriptor, _ := json.Marshal(geminiDescriptor{DataRoot: dataRoot})
	writeTestFile(t, filepath.Join(active, geminiDescriptorName), string(descriptor))
	server, err := geminiNativeServer(domain.MCPServer{Name: "docs", Type: "streamable-http", Decoded: mcp})
	if err != nil {
		t.Fatal(err)
	}
	server, err = materializeGeminiServer(server, active, dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := nativeconfig.DesiredReceipt(filepath.Join(configRoot, "settings.json"), nativeconfig.CodecGemini, "docs", server, nativeconfig.Placeholders{PackageRoot: active, DataRoot: dataRoot})
	if err != nil {
		t.Fatal(err)
	}
	return active, []domain.NativeObjectOwnership{
		{ObjectID: "gemini-skill:docs", Kind: geminiSkillObjectKind, LogicalName: "docs", Path: filepath.Join(configRoot, "skills", "docs"), SourceRelative: "skills/docs", ManagedDigest: digest, ProtectionClass: "managed"},
		{ObjectID: "gemini-mcp:docs", Kind: geminiMCPObjectKind, LogicalName: "docs", Path: filepath.Join(configRoot, "settings.json"), ManagedDigest: receipt.Digest, ProtectionClass: "managed"},
	}
}

func assertGeminiNativeState(t *testing.T, configRoot, marker, url string, present bool) {
	t.Helper()
	skillPath := filepath.Join(configRoot, "skills", "docs", "SKILL.md")
	if present {
		body, err := os.ReadFile(skillPath)
		if err != nil || string(body) != marker+"\n" {
			t.Fatalf("Gemini skill = %q, %v", body, err)
		}
	} else if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Fatalf("removed Gemini skill remains: %v", err)
	}
	document := readObject(t, filepath.Join(configRoot, "settings.json"))
	if document["theme"] != "night" {
		t.Fatalf("unmanaged Gemini config was not preserved: %+v", document)
	}
	servers := document["mcpServers"].(map[string]any)
	if _, ok := servers["unmanaged"]; !ok {
		t.Fatalf("unmanaged Gemini MCP entry lost: %+v", servers)
	}
	managed, exists := servers["docs"].(map[string]any)
	if exists != present {
		t.Fatalf("managed Gemini MCP presence = %v, want %v", exists, present)
	}
	if present && managed["httpUrl"] != url {
		t.Fatalf("managed Gemini URL = %v, want %v", managed["httpUrl"], url)
	}
}
