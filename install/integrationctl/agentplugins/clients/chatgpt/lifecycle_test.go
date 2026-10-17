package chatgpt

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

// TestChatGPTRemoveIsExactManualRemoval pins the exact ChatGPT removal
// contract: without the external uninstall confirmation remove is refused
// with the manual client step surfaced, and a confirmed removal deletes
// exactly nothing on disk (ChatGPT is a remote manual install), proven by a
// full file-inventory diff -- the same exact-removal accounting as the
// Claude/Codex/OpenCode e2e suites.
func TestChatGPTRemoveIsExactManualRemoval(t *testing.T) {
	root := t.TempDir()
	preparedFiles := map[string]string{
		filepath.Join(root, "prepared-marketplace", ".app.json"):        `{"name":"chatgpt-proof"}`,
		filepath.Join(root, "prepared-marketplace", ".well-known.json"): `{"x":1}`,
		filepath.Join(root, "skills", "proof", "SKILL.md"):              "proof\n",
	}
	for path, body := range preparedFiles {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	inventoryBefore := chatgptFileInventory(t, root)

	adapter := &Adapter{}
	blocked, err := adapter.Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientChatGPT}, Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if blocked.ExternalRemovalComplete {
		t.Fatalf("remove without external uninstall confirmation was accepted: %+v", blocked)
	}
	joined := strings.Join(blocked.UserActions, "\n")
	if !strings.Contains(joined, "uninstall the plugin in ChatGPT Plugins") || !strings.Contains(joined, "--external-uninstalled") {
		t.Fatalf("refusal did not surface the exact manual client step: %+v", blocked.UserActions)
	}

	confirmed, err := adapter.Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientChatGPT}, Confirmed: true, ExternalUninstalled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed.ExternalRemovalComplete {
		t.Fatalf("confirmed external uninstall was not accepted: %+v", confirmed)
	}
	removedPaths, addedPaths, changedPaths := chatgptInventoryDiff(inventoryBefore, chatgptFileInventory(t, root))
	if len(removedPaths) != 0 || len(addedPaths) != 0 || len(changedPaths) != 0 {
		t.Fatalf("ChatGPT removal must delete exactly nothing on disk (remote manual install): removed=%v added=%v changed=%v", removedPaths, addedPaths, changedPaths)
	}
	t.Logf("chatgpt exact removal: refused without the external-uninstall confirmation (manual step surfaced verbatim); with it, removed=%d on-disk paths (empty by design), added=%d, changed=%d", len(removedPaths), len(addedPaths), len(changedPaths))
}

func chatgptFileInventory(t *testing.T, root string) map[string]string {
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

func chatgptInventoryDiff(before, after map[string]string) (removed, added, changed []string) {
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
