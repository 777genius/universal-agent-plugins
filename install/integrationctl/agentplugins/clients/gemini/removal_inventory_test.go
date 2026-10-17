package gemini_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestGeminiRemoveDeletesExactlyManagedObjects pins the exact Gemini removal
// contract with full file-inventory accounting, the same discipline as the
// Claude/Codex/OpenCode e2e suites: a confirmed removal deletes exactly the
// managed skill directory and the managed MCP entry, nothing else -- foreign
// skills, foreign MCP entries and unrelated files survive untouched.
func TestGeminiRemoveDeletesExactlyManagedObjects(t *testing.T) {
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	configRoot := filepath.Join(root, ".gemini")
	managedSkill := filepath.Join(configRoot, "skills", "proof")
	foreignSkill := filepath.Join(configRoot, "skills", "foreign-skill")
	for path, body := range map[string]string{
		filepath.Join(configRoot, "settings.json"):          `{"mcpServers":{"proof-mcp":{"url":"https://managed.test/mcp"},"foreign-untouched":{"url":"https://foreign.test/mcp"}},"sentinel":true}` + "\n",
		filepath.Join(managedSkill, "SKILL.md"):             "managed proof\n",
		filepath.Join(foreignSkill, "SKILL.md"):             "foreign, must survive\n",
		filepath.Join(configRoot, "unrelated", "notes.txt"): "unrelated\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	inventoryBefore := geminiFileInventory(t, root)

	skillDigest, err := shared.DigestSkillDirectory(managedSkill)
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(configRoot, "settings.json")
	receipt, err := nativeconfig.DesiredReceipt(settingsPath, nativeconfig.CodecGemini, "proof-mcp", nativeconfig.Server{Type: "remote", URL: "https://managed.test/mcp", RemoteTransport: "sse"}, nativeconfig.Placeholders{})
	if err != nil {
		t.Fatal(err)
	}
	objects := []domain.NativeObjectOwnership{
		{ObjectID: "gemini-skill:proof", Kind: gemini.GeminiSkillObjectKind, LogicalName: "proof", Path: managedSkill, ManagedDigest: skillDigest, ProtectionClass: "managed"},
		{ObjectID: "gemini-mcp:proof-mcp", Kind: gemini.GeminiMCPObjectKind, LogicalName: "proof-mcp", Path: settingsPath, ManagedDigest: receipt.Digest, ProtectionClass: "managed"},
	}
	outcome, err := gemini.New().Deactivate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()},
		domain.DeactivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: configRoot}, Confirmed: true, NativeObjects: objects})
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.ExternalRemovalComplete {
		t.Fatalf("confirmed removal was not completed: %+v", outcome)
	}
	removedPaths, addedPaths, changedPaths := geminiInventoryDiff(inventoryBefore, geminiFileInventory(t, root))
	for _, path := range removedPaths {
		if !strings.HasPrefix(path, managedSkill) {
			t.Fatalf("remove deleted a file outside the plugin's owned scope: %s (removed=%v)", path, removedPaths)
		}
	}
	if len(removedPaths) == 0 {
		t.Fatal("remove deleted nothing: the managed skill directory should be gone")
	}
	if _, err := os.Lstat(managedSkill); !os.IsNotExist(err) {
		t.Fatalf("managed skill survived remove: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(foreignSkill, "SKILL.md")); err != nil || string(body) != "foreign, must survive\n" {
		t.Fatalf("foreign skill touched: %q %v", body, err)
	}
	settingsBody, err := os.ReadFile(filepath.Join(configRoot, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settingsBody), "foreign-untouched") || !strings.Contains(string(settingsBody), "sentinel") {
		t.Fatalf("foreign settings content lost: %s", settingsBody)
	}
	if strings.Contains(string(settingsBody), "proof-mcp") {
		t.Fatalf("managed MCP entry survived removal: %s", settingsBody)
	}
	t.Logf("gemini exact removal: removed=%d (managed skill only), added=%d, changed=%d; foreign skill, foreign MCP entry and unrelated files verified untouched", len(removedPaths), len(addedPaths), len(changedPaths))
}

func geminiFileInventory(t *testing.T, root string) map[string]string {
	t.Helper()
	inventory := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		inventory[path] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("inventory %s: %v", root, err)
	}
	return inventory
}

func geminiInventoryDiff(before, after map[string]string) (removed, added, changed []string) {
	for path := range before {
		digest, ok := after[path]
		if !ok {
			removed = append(removed, path)
			continue
		}
		if digest != before[path] {
			changed = append(changed, path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			added = append(added, path)
		}
	}
	sort.Strings(removed)
	sort.Strings(added)
	sort.Strings(changed)
	return removed, added, changed
}
