package usecase

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestCodexRemoveGroupPreservesForeignSiblingAndRefusesRepeatRemove exercises
// Codex's real removal path (usecase/remove_group.go plus the real
// providers.Activator.Deactivate branch for domain.ClientCodex) end to end
// with unmocked production code. No fake CLI runner is required: Codex's
// Deactivate only invokes its CLI when a managed marketplace entry is found
// registered in config.toml, and a disposable test ConfigRoot never has one
// (providers/codex_marketplace_cleanup.go's managedCodexMarketplaceRegistered
// returns false, nil when config.toml is absent -- confirmed by reading that
// exact function). This is real, non-mocked proof at the usecase/provider
// level -- explicitly NOT a live-native-Codex-binary proof. A live proof now
// exists separately (repotests/agentplugins_codex_native_e2e_test.go, run on
// a genuine non-emulated arm64 Linux Codex binary in a native linux/arm64
// container -- the earlier x86_64-emulation pidfd_open blocker on this
// machine does not apply to a real arm64 binary on this machine's native
// arm64 Docker Desktop) and found a real bug this usecase-level test cannot
// reach: after a successful remove, a freshly started codex app-server
// silently re-materialized the plugin from its stale config.toml
// registration, because config.toml's separate per-plugin `[plugins."id"]`
// entry was never cleaned up (fixed in providers/activator.go's
// removeCodexPlugin). This file's fake-CLI-free scenario cannot exercise
// that path at all, since it never has a registered marketplace entry to
// clean up in the first place -- see the live e2e test for that coverage.
//
// TestRemovePlanExposesExternalUninstallAndPreservesCodexArtifactUntilAcknowledged
// and TestRemoveCleansNativeCodexMarketplaceBeforeManagedArtifactDeletion
// (service_test.go) already cover Codex remove, the --external-uninstalled
// gate, and foreign-preservation at the native-config level (a real
// config.toml with a foreign marketplace entry that survives) through the
// single-target Service.Remove path -- this file does not repeat that. What
// is new here: the RemoveGroup path specifically (its refusal contract
// differs from single Remove -- RemoveGroup returns a hard error without the
// flag, Remove returns err == nil with Mutated:false), a foreign *sibling
// directory* surviving on the filesystem (not just a foreign native-config
// entry), repeat-remove refusal, and an exact assertion on the
// --external-uninstalled flag text in the returned guidance.
func TestCodexRemoveGroupPreservesForeignSiblingAndRefusesRepeatRemove(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	install := addInput(t, codex, "https://example.com/codex-remove-lifecycle")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{install}, OperationGroupID: "codex-remove-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	managedPath := added.Targets[0].Plan.ActivePath
	originalPluginJSON, err := os.ReadFile(filepath.Join(managedPath, "plugin.json"))
	if err != nil {
		t.Fatalf("read original plugin.json: %v", err)
	}
	baselineState, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	baselineClient := onlyBinding(baselineState.Installations[0])

	// Plant a completely unrelated, foreign sibling package under the same
	// managed root, to prove owned-only removal (collision/foreign-content
	// preservation, per task 3's requirement).
	foreignSibling := filepath.Join(filepath.Dir(managedPath), "foreign-sibling-plugin")
	if err := os.MkdirAll(foreignSibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreignSibling, "marker.txt"), []byte("not ours"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Tamper the managed content itself before attempting removal. The
	// refusal below comes from the generic ownership-digest preflight shared
	// by every client (remove_group.go), not anything Codex-specific --
	// Deactivate is never reached on this path. It must refuse rather than
	// silently remove drifted bytes, preserving both the (now-changed)
	// managed directory and the foreign sibling exactly as-is.
	if err := os.WriteFile(filepath.Join(managedPath, "plugin.json"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	removeInput := RemoveGroupInput{
		Selector:         added.InstallationID,
		Targets:          []RemoveInput{{Client: codex, Scope: domain.ScopeUser, ExternalUninstalled: true}},
		OperationGroupID: "codex-remove-tampered", Confirmed: true,
	}
	if _, err := service.RemoveGroup(context.Background(), removeInput); err == nil || !strings.Contains(err.Error(), "ownership verification failed") {
		t.Fatalf("tampered managed content removal error = %v, want an ownership verification failure", err)
	}
	if _, statErr := os.Stat(managedPath); statErr != nil {
		t.Fatalf("refused removal deleted the managed directory: %v", statErr)
	}
	if _, statErr := os.Stat(foreignSibling); statErr != nil {
		t.Fatalf("foreign sibling did not survive a refused removal: %v", statErr)
	}
	afterTamperState, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	afterTamperClient := onlyBinding(afterTamperState.Installations[0])
	if afterTamperClient.Materialization != baselineClient.Materialization || len(afterTamperClient.Receipts) != len(baselineClient.Receipts) {
		t.Fatalf("refused removal changed persisted client state: before=%+v after=%+v", baselineClient, afterTamperClient)
	}

	// Restore the managed content to its originally recorded bytes (an
	// operator undoing the tamper) and perform a genuine removal.
	if err := os.WriteFile(filepath.Join(managedPath, "plugin.json"), originalPluginJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	removeInput.OperationGroupID = "codex-remove-real"
	removed, err := service.RemoveGroup(context.Background(), removeInput)
	if err != nil {
		t.Fatalf("real removal failed: %v", err)
	}
	if !removed.Mutated || removed.Phase != GroupPhaseCompleted {
		t.Fatalf("real removal result = %+v, want mutated/completed", removed)
	}
	if _, statErr := os.Stat(managedPath); !os.IsNotExist(statErr) {
		t.Fatalf("managed directory survived a completed removal: %v", statErr)
	}
	if _, statErr := os.Stat(foreignSibling); statErr != nil {
		t.Fatalf("foreign sibling did not survive real removal: %v", statErr)
	}
	if body, readErr := os.ReadFile(filepath.Join(foreignSibling, "marker.txt")); readErr != nil || string(body) != "not ours" {
		t.Fatalf("foreign sibling content changed: body=%q err=%v", body, readErr)
	}
	afterRemoveState, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRemoveState.Installations) != 1 || len(afterRemoveState.Installations[0].Clients) != 0 || !afterRemoveState.Installations[0].DataRetained {
		t.Fatalf("real removal did not leave minimal data-retained state: %+v", afterRemoveState.Installations)
	}

	// Repeat-remove must be safely refused, not silently accepted.
	_, err = service.RemoveGroup(context.Background(), RemoveGroupInput{
		Selector:         added.InstallationID,
		Targets:          []RemoveInput{{Client: codex, Scope: domain.ScopeUser, ExternalUninstalled: true}},
		OperationGroupID: "codex-remove-repeat", Confirmed: true,
	})
	if err == nil || !strings.Contains(err.Error(), "not materialized") {
		t.Fatalf("repeat-remove error = %v, want a not-materialized refusal", err)
	}
	if _, statErr := os.Stat(foreignSibling); statErr != nil {
		t.Fatalf("foreign sibling did not survive a repeat-remove attempt: %v", statErr)
	}
	if body, readErr := os.ReadFile(filepath.Join(foreignSibling, "marker.txt")); readErr != nil || string(body) != "not ours" {
		t.Fatalf("foreign sibling content changed after repeat-remove attempt: body=%q err=%v", body, readErr)
	}
}

// TestCodexRemoveGroupRequiresExternalUninstalledFlag confirms Codex's
// documented remove contract: UAP has no supported Codex CLI verb to
// silently uninstall a plugin on the user's behalf, so remove without
// --external-uninstalled must be refused with actionable guidance, leaving
// the managed directory untouched.
func TestCodexRemoveGroupRequiresExternalUninstalledFlag(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	install := addInput(t, codex, "https://example.com/codex-remove-requires-flag")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{install}, OperationGroupID: "codex-remove-flag-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	managedPath := added.Targets[0].Plan.ActivePath
	beforePluginJSON, err := os.ReadFile(filepath.Join(managedPath, "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin.json before refused remove: %v", err)
	}
	beforeState, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RemoveGroup(context.Background(), RemoveGroupInput{
		Selector:         added.InstallationID,
		Targets:          []RemoveInput{{Client: codex, Scope: domain.ScopeUser}},
		OperationGroupID: "codex-remove-no-flag", Confirmed: true,
	})
	if err == nil || !strings.Contains(err.Error(), "did not authorize managed artifact removal") {
		t.Fatalf("remove without the flag error = %v, want a client-did-not-authorize refusal", err)
	}
	if result.Mutated || result.Phase != GroupPhaseManagedUnchanged {
		t.Fatalf("refused remove result = %+v, want unmutated/managed_unchanged", result)
	}
	if len(result.Targets) != 1 {
		t.Fatalf("expected exactly one target result: %+v", result.Targets)
	}
	actions := strings.Join(result.Targets[0].Deactivation.UserActions, " | ")
	if !strings.Contains(actions, "--external-uninstalled") {
		t.Fatalf("actionable guidance missing --external-uninstalled: %q", actions)
	}
	if _, statErr := os.Stat(managedPath); statErr != nil {
		t.Fatalf("refused remove touched the managed directory: %v", statErr)
	}
	afterPluginJSON, err := os.ReadFile(filepath.Join(managedPath, "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin.json after refused remove: %v", err)
	}
	if !bytes.Equal(beforePluginJSON, afterPluginJSON) {
		t.Fatalf("refused remove changed managed directory content: before=%q after=%q", beforePluginJSON, afterPluginJSON)
	}
	afterState, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(afterState.Installations) != 1 || len(afterState.Installations[0].Clients) != 1 {
		t.Fatalf("refused remove changed persisted state shape: before=%+v after=%+v", beforeState.Installations, afterState.Installations)
	}
	beforeClient := onlyBinding(beforeState.Installations[0])
	afterClient := onlyBinding(afterState.Installations[0])
	if beforeClient.Materialization != afterClient.Materialization || len(beforeClient.Receipts) != len(afterClient.Receipts) {
		t.Fatalf("refused remove changed the client binding: before=%+v after=%+v", beforeClient, afterClient)
	}
}
