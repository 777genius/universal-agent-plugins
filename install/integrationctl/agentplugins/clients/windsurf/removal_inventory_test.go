package windsurf

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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestWindsurfRemoveDeletesExactlyManagedObjects pins the exact Windsurf
// removal contract with full file-inventory accounting, the same discipline as
// the Claude/Codex/OpenCode e2e suites: a confirmed removal deletes exactly the
// managed MCP entry and nothing else -- the shared settings file keeps foreign
// MCP entries and sentinel keys, foreign skills and unrelated files survive
// byte-identical.
func TestWindsurfRemoveDeletesExactlyManagedObjects(t *testing.T) {
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	configRoot := filepath.Join(root, ".windsurf")
	foreignSkill := filepath.Join(configRoot, "skills", "foreign-skill")
	settingsPath, err := windsurfConfigPath(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		settingsPath:                                        `{"mcpServers":{"proof-mcp":{"url":"https://managed.test/mcp"},"foreign-untouched":{"url":"https://foreign.test/mcp"}},"sentinel":true}` + "\n",
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
	inventoryBefore := windsurfFileInventory(t, root)

	receipt, err := nativeconfig.DesiredReceipt(settingsPath, nativeconfig.CodecWindsurf, "proof-mcp", nativeconfig.Server{Type: "remote", URL: "https://managed.test/mcp", RemoteTransport: "sse"}, nativeconfig.Placeholders{})
	if err != nil {
		t.Fatal(err)
	}
	objects := []domain.NativeObjectOwnership{
		{ObjectID: "windsurf:mcp:proof-mcp", Kind: WindsurfMCPObjectKind, LogicalName: "proof-mcp", Path: settingsPath, ManagedDigest: receipt.Digest, ProtectionClass: "managed_entry"},
	}
	outcome, err := New().Deactivate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()},
		domain.DeactivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientWindsurf, ConfigRoot: configRoot}, Confirmed: true, NativeObjects: objects})
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.ExternalRemovalComplete {
		t.Fatalf("confirmed removal was not completed: %+v", outcome)
	}
	removedPaths, addedPaths, changedPaths := windsurfInventoryDiff(inventoryBefore, windsurfFileInventory(t, root))
	// The managed Windsurf object is one entry inside the shared settings file;
	// a correct removal deletes no files at all.
	for _, path := range removedPaths {
		t.Fatalf("remove deleted a file outside the plugin's owned scope: %s (removed=%v)", path, removedPaths)
	}
	for _, path := range changedPaths {
		if path != settingsPath {
			t.Fatalf("remove rewrote a file outside the managed MCP settings: %s (changed=%v)", path, changedPaths)
		}
	}
	lockPaths := map[string]bool{}
	if paths, err := nativeconfig.WriterLockPaths(nativeconfig.Paths{JSON: settingsPath}, nativeconfig.CodecWindsurf); err == nil {
		for _, path := range paths {
			lockPaths[path] = true
		}
	}
	for _, path := range addedPaths {
		if !lockPaths[path] {
			t.Fatalf("remove left a file behind outside the cooperating writer lock: %s (added=%v)", path, addedPaths)
		}
	}
	if body, err := os.ReadFile(filepath.Join(foreignSkill, "SKILL.md")); err != nil || string(body) != "foreign, must survive\n" {
		t.Fatalf("foreign skill touched: %q %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(configRoot, "unrelated", "notes.txt")); err != nil || string(body) != "unrelated\n" {
		t.Fatalf("unrelated file touched: %q %v", body, err)
	}
	settingsBody, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settingsBody), "foreign-untouched") || !strings.Contains(string(settingsBody), "sentinel") {
		t.Fatalf("foreign settings content lost: %s", settingsBody)
	}
	if strings.Contains(string(settingsBody), "proof-mcp") {
		t.Fatalf("managed MCP entry survived removal: %s", settingsBody)
	}
	t.Logf("windsurf exact removal inventory: removed=%d all plugin-owned, added=%d, changed=%d; managed MCP entry gone, foreign MCP entry, foreign skill and unrelated files verified byte-identical", len(removedPaths), len(addedPaths), len(changedPaths))
}

func windsurfFileInventory(t *testing.T, root string) map[string]string {
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

func windsurfInventoryDiff(before, after map[string]string) (removed, added, changed []string) {
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
