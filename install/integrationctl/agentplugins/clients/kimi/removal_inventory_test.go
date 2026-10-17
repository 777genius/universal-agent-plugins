package kimi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestKimiRemoveChangesExactlyTheRegistry pins the exact Kimi removal contract
// with full file-inventory accounting, the same discipline as the
// Claude/Codex/OpenCode e2e suites: a confirmed removal rewrites exactly the
// registry file to drop the managed record -- the managed artifact directory,
// foreign plugin records and unrelated files survive byte-identical.
func TestKimiRemoveChangesExactlyTheRegistry(t *testing.T) {
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	configRoot := filepath.Join(root, ".kimi-code")
	managedArtifact := filepath.Join(configRoot, "plugins", "managed", "proof")
	foreignArtifact := filepath.Join(configRoot, "plugins", "managed", "foreign-plugin")
	registryPath := filepath.Join(configRoot, "plugins", "installed.json")
	for path, body := range map[string]string{
		filepath.Join(managedArtifact, ".kimi-plugin", "plugin.json"): `{"name":"proof","skills":"./skills/"}` + "\n",
		filepath.Join(managedArtifact, "skills", "proof", "SKILL.md"): "managed proof\n",
		filepath.Join(foreignArtifact, ".kimi-plugin", "plugin.json"): `{"name":"foreign-plugin"}` + "\n",
		filepath.Join(configRoot, "unrelated", "notes.txt"):           "unrelated\n",
		registryPath: `{"version":1,"plugins":[` +
			`{"id":"proof","root":` + jsonString(managedArtifact) + `,"source":"local-path","enabled":true,"installedAt":"2026-01-01T00:00:00Z"},` +
			`{"id":"foreign-plugin","root":` + jsonString(foreignArtifact) + `,"source":"github","enabled":false,"github":{"ref":"xyz"},"future":[1,"two"]}` +
			`],"sentinel":{"keep":true}}` + "\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	inventoryBefore := kimiFileInventory(t, root)

	outcome, err := New().Deactivate(context.Background(), clients.Env{},
		domain.DeactivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientKimi, ConfigRoot: configRoot}, DeclaredName: "proof", ManagedArtifactPath: managedArtifact, Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.ExternalRemovalComplete {
		t.Fatalf("confirmed removal was not completed: %+v", outcome)
	}
	removedPaths, addedPaths, changedPaths := kimiInventoryDiff(inventoryBefore, kimiFileInventory(t, root))
	for _, path := range removedPaths {
		if path != registryPath && !strings.HasPrefix(path, managedArtifact) {
			t.Fatalf("remove deleted a file outside the plugin's owned scope: %s (removed=%v)", path, removedPaths)
		}
	}
	for _, path := range changedPaths {
		if path != registryPath {
			t.Fatalf("remove rewrote a file outside the Kimi registry: %s (changed=%v)", path, changedPaths)
		}
	}
	if len(addedPaths) != 0 {
		t.Fatalf("remove left files behind: added=%v", addedPaths)
	}
	if body, err := os.ReadFile(filepath.Join(managedArtifact, "skills", "proof", "SKILL.md")); err != nil || string(body) != "managed proof\n" {
		t.Fatalf("managed artifact touched by registry removal: %q %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(foreignArtifact, ".kimi-plugin", "plugin.json")); err != nil || string(body) != "{\"name\":\"foreign-plugin\"}\n" {
		t.Fatalf("foreign plugin artifact touched: %q %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(configRoot, "unrelated", "notes.txt")); err != nil || string(body) != "unrelated\n" {
		t.Fatalf("unrelated file touched: %q %v", body, err)
	}
	var document map[string]any
	body, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatalf("registry unreadable after removal: %v", err)
	}
	if _, ok := document["sentinel"]; !ok {
		t.Fatalf("foreign registry metadata lost: %s", body)
	}
	records, _ := document["plugins"].([]any)
	var ids []string
	for _, raw := range records {
		record, _ := raw.(map[string]any)
		ids = append(ids, record["id"].(string))
	}
	if strings.Contains(strings.Join(ids, ","), "proof") {
		t.Fatalf("managed registry record survived removal: %s", body)
	}
	if len(records) != 1 || ids[0] != "foreign-plugin" {
		t.Fatalf("foreign registry record lost: ids=%v body=%s", ids, body)
	}
	t.Logf("kimi exact removal inventory: removed=%d all plugin-owned, added=%d, changed=%d; only the registry record changed and the managed/foreign plugin artifacts plus unrelated files are byte-identical", len(removedPaths), len(addedPaths), len(changedPaths))
}

func jsonString(value string) string {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(body)
}

func kimiFileInventory(t *testing.T, root string) map[string]string {
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

func kimiInventoryDiff(before, after map[string]string) (removed, added, changed []string) {
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
