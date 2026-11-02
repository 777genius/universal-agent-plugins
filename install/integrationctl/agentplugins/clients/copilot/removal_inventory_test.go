package copilot

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

// TestCopilotRemoveIsExactManualRemoval pins the exact Copilot removal
// contract: without the external uninstall confirmation remove is refused
// fail-closed with the CLI-driven manual step surfaced, and a confirmed removal
// deletes exactly nothing on disk (Copilot registration lives in the Copilot
// CLI), proven by a full file-inventory diff -- the same exact-removal
// accounting as the Claude/Codex/OpenCode e2e suites.
func TestCopilotRemoveIsExactManualRemoval(t *testing.T) {
	root := t.TempDir()
	preparedFiles := map[string]string{
		filepath.Join(root, "prepared-package", ".agentplugins.json"): `{"name":"copilot-proof"}`,
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
	inventoryBefore := copilotFileInventory(t, root)

	adapter := New()
	blocked, err := adapter.Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientCopilot}, DeclaredName: "proof", Confirmed: true,
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
	if !strings.Contains(joined, "copilot plugin uninstall") || !strings.Contains(joined, "--external-uninstalled") {
		t.Fatalf("refusal did not surface the exact manual client step: %+v", blocked.UserActions)
	}

	confirmed, err := adapter.Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientCopilot}, DeclaredName: "proof", Confirmed: true, ExternalUninstalled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed.ExternalRemovalComplete {
		t.Fatalf("confirmed external uninstall was not accepted: %+v", confirmed)
	}
	removedPaths, addedPaths, changedPaths := copilotInventoryDiff(inventoryBefore, copilotFileInventory(t, root))
	if len(removedPaths) != 0 || len(addedPaths) != 0 || len(changedPaths) != 0 {
		t.Fatalf("Copilot removal must delete exactly nothing on disk (CLI-driven manual install): removed=%v added=%v changed=%v", removedPaths, addedPaths, changedPaths)
	}
	t.Logf("copilot exact removal inventory: refused without the external-uninstall confirmation (fail-closed manual step surfaced); with it, removed=%d on-disk paths (empty by design), added=%d, changed=%d", len(removedPaths), len(addedPaths), len(changedPaths))
}

func copilotFileInventory(t *testing.T, root string) map[string]string {
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

func copilotInventoryDiff(before, after map[string]string) (removed, added, changed []string) {
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
