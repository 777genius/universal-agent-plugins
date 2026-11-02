package vscode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"
)

// TestVSCodeLocalRemoveDeletesExactlyOwnedProfileEntry pins the exact VS Code
// Local removal contract with full file-inventory accounting, the same
// discipline as the Claude/Codex/OpenCode e2e suites: a confirmed removal
// deletes exactly the owned chat.pluginLocations entry from the selected
// profile settings and nothing else -- foreign settings keys, foreign
// pluginLocations and unrelated files survive byte-identical.
func TestVSCodeLocalRemoveDeletesExactlyOwnedProfileEntry(t *testing.T) {
	if runtime.GOOS == "darwin" && runtime.GOARCH != "arm64" {
		t.Skip("local profile mutation is qualified only on darwin/arm64, linux and windows")
	}
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	settings := filepath.Join(root, "settings.json")
	active := filepath.Join(root, "TEST-plugin")
	tuple := SourceQualifiedTESTTuple(runtime.GOOS)
	if runtime.GOOS == "darwin" {
		tuple = QualifiedDarwinTESTTuple()
	}
	for path, body := range map[string]string{
		settings:                              `{"foreign":17,"chat.pluginLocations":{"` + active + `":true,"/foreign/keep":true}}` + "\n",
		filepath.Join(active, "artifact.txt"): "managed artifact, removed by artifact removal not profile removal\n",
		filepath.Join(root, "unrelated", "notes.txt"): "unrelated\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	inventoryBefore := vscodeFileInventory(t, root)

	enabled := true
	facts := domain.LocalDeliveryFacts{
		ProfileRoot: root, SettingsPath: settings, ProfileIdentity: root, SettingsIdentity: settings,
		Tuple: tuple, Skills: []string{"notify"},
		CanonicalDigest: vscodeTestDigest([]byte("TEST canonical")), ProjectionDigest: vscodeTestDigest([]byte("TEST projected")),
		Registration: domain.OwnedProfileEntry{ObjectID: localObjectID(settings, active), Selector: active, DesiredValue: &enabled},
	}
	selected, err := domain.NewLocalDelivery(facts)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewLocal(LocalConfig{
		ProfileSettingsPath: settings, QualifiedTuple: tuple,
		TargetShell: vscodelocalhooks.Target{Shell: vscodelocalhooks.Shell(tuple.TargetShell)}, Skills: []string{"notify"},
	})
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := adapter.Deactivate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, domain.DeactivationRequest{
		Client:           domain.DetectedClient{ClientID: domain.ClientVSCode, ConfigRoot: root},
		SelectedDelivery: selected, ManagedArtifactPath: active,
		NativeObjects:    []domain.NativeObjectOwnership{facts.Registration.Ownership(settings)},
		RemoveOwnedEntry: true, Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.ExternalRemovalComplete {
		t.Fatalf("confirmed removal was not completed: %+v", outcome)
	}
	removedPaths, addedPaths, changedPaths := vscodeInventoryDiff(inventoryBefore, vscodeFileInventory(t, root))
	for _, path := range removedPaths {
		t.Fatalf("profile removal deleted a file outside the plugin's owned scope: %s (removed=%v)", path, removedPaths)
	}
	for _, path := range changedPaths {
		if path != settings {
			t.Fatalf("profile removal rewrote a file outside the selected settings: %s (changed=%v)", path, changedPaths)
		}
	}
	lockPaths := map[string]bool{}
	if paths, err := nativeconfig.WriterLockPaths(nativeconfig.Paths{JSON: settings}, nativeconfig.CodecMCPServers); err == nil {
		for _, path := range paths {
			lockPaths[path] = true
		}
	}
	for _, path := range addedPaths {
		if !lockPaths[path] {
			t.Fatalf("profile removal left a file behind outside the cooperating writer lock: %s (added=%v)", path, addedPaths)
		}
	}
	settingsBody, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(settingsBody), active) {
		t.Fatalf("owned chat.pluginLocations entry survived removal: %s", settingsBody)
	}
	if !strings.Contains(string(settingsBody), "/foreign/keep") || !strings.Contains(string(settingsBody), "foreign") {
		t.Fatalf("foreign profile settings lost: %s", settingsBody)
	}
	if body, err := os.ReadFile(filepath.Join(active, "artifact.txt")); err != nil || !strings.HasPrefix(string(body), "managed artifact") {
		t.Fatalf("managed artifact touched by profile removal: %q %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(root, "unrelated", "notes.txt")); err != nil || string(body) != "unrelated\n" {
		t.Fatalf("unrelated file touched: %q %v", body, err)
	}
	t.Logf("vscode local exact removal inventory: removed=%d all plugin-owned, added=%d, changed=%d; owned profile entry gone and foreign settings, pluginLocations and unrelated files verified byte-identical", len(removedPaths), len(addedPaths), len(changedPaths))
}

// TestVSCodeRemoveIsExactManualRemoval pins the exact removal contract of the
// Copilot-CLI-backed VS Code adapter: without the external uninstall
// confirmation remove is refused fail-closed with the manual client step
// surfaced, and a confirmed removal deletes exactly nothing on disk, proven by
// a full file-inventory diff -- the same discipline as the ChatGPT contract.
func TestVSCodeRemoveIsExactManualRemoval(t *testing.T) {
	root := t.TempDir()
	preparedFiles := map[string]string{
		filepath.Join(root, "prepared-package", ".agentplugins.json"): `{"name":"vscode-proof"}`,
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
	inventoryBefore := vscodeFileInventory(t, root)

	adapter := New()
	blocked, err := adapter.Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientVSCode}, DeclaredName: "proof", Confirmed: true,
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
	if !strings.Contains(joined, "remove the plugin in VS Code") || !strings.Contains(joined, "--external-uninstalled") {
		t.Fatalf("refusal did not surface the exact manual client step: %+v", blocked.UserActions)
	}

	confirmed, err := adapter.Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientVSCode}, DeclaredName: "proof", Confirmed: true, ExternalUninstalled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed.ExternalRemovalComplete {
		t.Fatalf("confirmed external uninstall was not accepted: %+v", confirmed)
	}
	removedPaths, addedPaths, changedPaths := vscodeInventoryDiff(inventoryBefore, vscodeFileInventory(t, root))
	if len(removedPaths) != 0 || len(addedPaths) != 0 || len(changedPaths) != 0 {
		t.Fatalf("VS Code CLI removal must delete exactly nothing on disk (manual install): removed=%v added=%v changed=%v", removedPaths, addedPaths, changedPaths)
	}
	t.Logf("vscode exact removal inventory (CLI manual path): refused without the external-uninstall confirmation (fail-closed manual step surfaced); with it, removed=%d on-disk paths (empty by design), added=%d, changed=%d", len(removedPaths), len(addedPaths), len(changedPaths))
}

func vscodeTestDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func vscodeFileInventory(t *testing.T, root string) map[string]string {
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

func vscodeInventoryDiff(before, after map[string]string) (removed, added, changed []string) {
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
