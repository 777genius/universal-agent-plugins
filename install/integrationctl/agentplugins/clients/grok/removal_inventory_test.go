package grok

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestGrokRemoveIsExactManualRemoval pins the exact Grok Build removal
// contract: without the external uninstall confirmation remove is refused
// fail-closed with the CLI-driven manual step surfaced, and a confirmed removal
// deletes exactly nothing on disk (Grok registration lives in the Grok Build
// CLI), proven by a full file-inventory diff -- the same exact-removal
// accounting as the Claude/Codex/OpenCode e2e suites.
func TestGrokRemoveIsExactManualRemoval(t *testing.T) {
	root := t.TempDir()
	preparedFiles := map[string]string{
		filepath.Join(root, "prepared-package", ".agentplugins.json"): `{"name":"grok-proof"}`,
		filepath.Join(root, "skills", "proof", "SKILL.md"):            "proof\n",
		filepath.Join(root, "unrelated", "notes.txt"):                 "unrelated\n",
	}
	for path, body := range preparedFiles {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	inventoryBefore := grokFileInventory(t, root)

	adapter := New()
	blocked, err := adapter.Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientGrok}, DeclaredName: "proof", BackendExecutable: "/test/bin/grok", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if blocked.ExternalRemovalComplete {
		t.Fatalf("remove without external uninstall confirmation was accepted: %+v", blocked)
	}
	if blocked.ArtifactRemovalAllowed {
		t.Fatalf("fail-closed removal allowed artifact removal while the client registration may dangle: %+v", blocked)
	}
	joined := strings.Join(blocked.UserActions, "\n")
	if !strings.Contains(joined, "uninstall the managed plugin in Grok Build") || !strings.Contains(joined, "--external-uninstalled") {
		t.Fatalf("refusal did not surface the exact manual client step: %+v", blocked.UserActions)
	}

	confirmed, err := adapter.Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientGrok}, DeclaredName: "proof", BackendExecutable: "/test/bin/grok", Confirmed: true, ExternalUninstalled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed.ExternalRemovalComplete {
		t.Fatalf("confirmed external uninstall was not accepted: %+v", confirmed)
	}
	removedPaths, addedPaths, changedPaths := grokInventoryDiff(inventoryBefore, grokFileInventory(t, root))
	if len(removedPaths) != 0 || len(addedPaths) != 0 || len(changedPaths) != 0 {
		t.Fatalf("Grok removal must delete exactly nothing on disk (CLI-driven manual install): removed=%v added=%v changed=%v", removedPaths, addedPaths, changedPaths)
	}
	t.Logf("grok exact removal inventory: refused without the external-uninstall confirmation (fail-closed manual step surfaced); with it, removed=%d on-disk paths (empty by design), added=%d, changed=%d", len(removedPaths), len(addedPaths), len(changedPaths))
}

// TestGrokRemoveRefusesForeignRegisteredSource pins the ownership refusal: a
// plugin registered from a different source must never be uninstalled, and the
// refusal must leave disk exactly unchanged.
func TestGrokRemoveRefusesForeignRegisteredSource(t *testing.T) {
	root := t.TempDir()
	managedArtifact := filepath.Join(root, "plugins", "managed", "proof")
	preparedFiles := map[string]string{
		filepath.Join(managedArtifact, ".agentplugins.json"): `{"name":"grok-proof"}`,
		filepath.Join(root, "unrelated", "notes.txt"):        "unrelated\n",
	}
	for path, body := range preparedFiles {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	inventoryBefore := grokFileInventory(t, root)

	fake := &fakeGrok{entries: []pluginEntry{{Name: "proof", Status: "installed", Source: filepath.Join(root, "foreign", "elsewhere"), Version: "1.0.0"}}}
	refused, err := New().Deactivate(context.Background(), clients.Env{Runner: fake}, domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientGrok}, DeclaredName: "proof", BackendExecutable: "/test/bin/grok", ManagedArtifactPath: managedArtifact, Confirmed: true,
	})
	if err == nil || !strings.Contains(err.Error(), "registered from a different source; refusing to uninstall it") {
		t.Fatalf("foreign registered source was not refused: %+v %v", refused, err)
	}
	if refused.ExternalRemovalComplete {
		t.Fatalf("refusal still reported completed removal: %+v", refused)
	}
	for _, command := range fake.commands {
		if len(command) > 2 && (command[2] == "uninstall" || command[2] == "install") {
			t.Fatalf("refusal mutated Grok Build: %v", fake.commands)
		}
	}
	removedPaths, addedPaths, changedPaths := grokInventoryDiff(inventoryBefore, grokFileInventory(t, root))
	if len(removedPaths) != 0 || len(addedPaths) != 0 || len(changedPaths) != 0 {
		t.Fatalf("ownership refusal must leave disk exactly unchanged: removed=%v added=%v changed=%v", removedPaths, addedPaths, changedPaths)
	}
	t.Logf("grok exact removal inventory (foreign-source refusal): removed=%d on-disk paths, added=%d, changed=%d; refusal left disk byte-identical", len(removedPaths), len(addedPaths), len(changedPaths))
}

func grokFileInventory(t *testing.T, root string) map[string]string {
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

func grokInventoryDiff(before, after map[string]string) (removed, added, changed []string) {
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
