package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// dualNativeRawUserBindings checks the shared saved ledger without inferring
// ownership from plugin names or from which client happened to be queried.
func dualNativeRawUserBindings(t *testing.T, statePath string, expected int, codexProfile, claudeProfile string) map[string]domain.ClientBinding {
	t.Helper()
	body, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != expected {
		t.Fatalf("dual-client saved raw Skill has unexpected ownership: installations=%d receipts=%d", len(state.Installations), len(state.EnvironmentReceipts))
	}
	installation := state.Installations[0]
	if installation.Source.StableSourceKey != nativeRawUserSkillSourceKey ||
		installation.Source.ResolvedRevision != nativeRawUserSkillCommit {
		t.Fatalf("dual-client raw Skill lost immutable source identity: %+v", installation.Source)
	}
	out := make(map[string]domain.ClientBinding, expected)
	for _, receipt := range state.EnvironmentReceipts {
		binding, ok := installation.Clients[receipt.ClientBindingID]
		if !ok || receipt.InstallationID != installation.InstallationID ||
			binding.ClientBindingID != receipt.ClientBindingID || binding.ClientID != receipt.ClientID ||
			binding.Scope != "user" || binding.Verification != domain.VerificationInstalled ||
			binding.TargetLocator == "" || binding.PhysicalArtifact == "" {
			t.Fatalf("dual-client receipt disagrees with native binding: %+v %+v", receipt, binding)
		}
		switch binding.ClientID {
		case "codex":
			if binding.NativeProfileRoot != codexProfile {
				t.Fatalf("Codex user profile escaped selected authority: %+v", binding)
			}
		case "claude":
			if filepath.Dir(binding.TargetLocator) != filepath.Join(claudeProfile, "skills") ||
				(binding.NativeProfileRoot != "" && binding.NativeProfileRoot != claudeProfile) {
				t.Fatalf("Claude user profile escaped selected skills directory: %+v", binding)
			}
		default:
			t.Fatalf("unexpected native client in dual installation: %s", binding.ClientID)
		}
		if _, duplicate := out[binding.ClientID]; duplicate {
			t.Fatalf("duplicate dual-client ownership for %s", binding.ClientID)
		}
		out[binding.ClientID] = binding
	}
	if len(out) != expected || len(installation.Clients) != expected {
		t.Fatalf("dual-client bindings/receipts disagree: bindings=%d receipts=%d", len(installation.Clients), len(out))
	}
	return out
}

// dualNativeRawUserResult requires one completed public CLI action per named
// real client, not a generic top-level success that could hide a partial apply.
func dualNativeRawUserResult(t *testing.T, result map[string]any, runErr error, expectedStatus string) {
	t.Helper()
	rows, ok := result["items"].([]any)
	if runErr != nil || result["status"] != "succeeded" || !ok || len(rows) != 2 {
		t.Fatalf("dual native CLI did not complete two actions: %v %+v", runErr, result)
	}
	found := map[string]bool{}
	for _, raw := range rows {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("invalid dual native item: %+v", raw)
		}
		client, _ := item["targetId"].(string)
		if (client != "codex" && client != "claude") || found[client] || item["status"] != expectedStatus {
			t.Fatalf("inexact dual native outcome: %+v", item)
		}
		found[client] = true
	}
	if !found["codex"] || !found["claude"] {
		t.Fatalf("dual native items omitted a selected client: %+v", rows)
	}
}

// TestAgentpluginsCodexClaudeNativeDualUserSkillLifecycle proves both native
// clients discover one shared user/global raw Skill transaction, and that
// selected removal cannot damage the still-installed client's owned bytes.
func TestAgentpluginsCodexClaudeNativeDualUserSkillLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_DUAL_CODEX_CLAUDE_NATIVE_E2E") != "1" {
		t.Skip("opt-in genuine pinned dual-client raw user Skill E2E")
	}
	nativeRequireDisposable(t)
	codexBin := nativeBinary(t, "AGENTPLUGINS_CODEX_BIN")
	claudeBin := claudeNativeBinary(t, "AGENTPLUGINS_CLAUDE_BIN")
	installer := nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	if want := os.Getenv("AGENTPLUGINS_CODEX_SHA256"); len(want) != 64 || nativeSHA(t, codexBin) != want {
		t.Fatal("dual native Codex executable pin mismatch")
	}
	if want := os.Getenv("AGENTPLUGINS_CLAUDE_SHA256"); len(want) != 64 || claudeSHA256(t, claudeBin) != want {
		t.Fatal("dual native Claude executable pin mismatch")
	}
	f := newNativeFixture(t, true)
	f.ClientBinDir = filepath.Dir(codexBin)
	nativeProvisionScanner(t, f)
	// Match the pinned single-client Codex fixture: opt into on-disk auth
	// stores without introducing ambient credentials or native telemetry.
	nativeWrite(t, filepath.Join(f.CodexHome, "config.toml"), []byte("cli_auth_credentials_store = \"file\"\nmcp_oauth_credentials_store = \"file\"\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n"), 0600)
	claudeProfile := filepath.Join(f.Root, "claude-config")
	if err := os.MkdirAll(claudeProfile, 0700); err != nil {
		t.Fatal(err)
	}
	claudeFixture := &claudeNativeFixture{Root: f.Root, Home: f.Home, ConfigDir: claudeProfile,
		StateHome: filepath.Join(f.Root, "installer-state"), transcriptSHA256: map[string]string{}}
	codexVersion, err := nativeCommand(t, f, codexBin, "dual-codex-version", "--version")
	if err != nil || strings.TrimSpace(string(codexVersion)) != "codex-cli "+os.Getenv("AGENTPLUGINS_CODEX_VERSION") {
		t.Fatalf("dual native Codex version mismatch: %q %v", codexVersion, err)
	}
	if version := claudeVersionString(t, claudeFixture, claudeBin); version != os.Getenv("AGENTPLUGINS_CLAUDE_VERSION") {
		t.Fatalf("dual native Claude version mismatch: %q", version)
	}

	foreignCodex := filepath.Join(f.CodexHome, "skills", "foreign-dual", "SKILL.md")
	foreignClaude := filepath.Join(claudeProfile, "skills", "foreign-dual", "SKILL.md")
	nativeWrite(t, foreignCodex, []byte("---\nname: foreign-dual\ndescription: Foreign Codex user Skill\n---\nNever modify.\n"), 0600)
	nativeWrite(t, foreignClaude, []byte("---\nname: foreign-dual\ndescription: Foreign Claude user Skill\n---\nNever modify.\n"), 0600)
	codexForeignSHA, claudeForeignSHA := nativeSHA(t, foreignCodex), nativeSHA(t, foreignClaude)
	env := append(f.env(filepath.Dir(codexBin)), "CLAUDE_CONFIG_DIR="+claudeProfile,
		"DISABLE_AUTOUPDATER=1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
	statePath := filepath.Join(f.Root, "installer-state", "state-v2.json")
	add := []string{"skills", "add", nativeRawUserSkillSource, "--skill", nativeRawUserSkillName,
		"--target", "codex,claude", "--no-ui", "--yes"}
	result, runErr := commandsRunInstaller(t, f.Root, f.Project, env, installer, "dual-user-add", append(append([]string{}, add...), "--scope", "user")...)
	dualNativeRawUserResult(t, result, runErr, "succeeded")
	bindings := dualNativeRawUserBindings(t, statePath, 2, f.CodexHome, claudeProfile)
	for client, binding := range bindings {
		nativeRawUserFiles(t, binding, client)
	}
	// Both genuine clients must discover the exact projection, not just
	// appear in the installer's success envelope.
	declared := stateDeclaredNativeSkill(t, statePath)
	codexID := declared + "@" + shared.ManagedMarketplaceName(bindings["codex"].PhysicalArtifact)
	claudeID := declared + "@skills-dir"
	codexNativeRawUserInventory(t, f, codexBin, codexID, declared, bindings["codex"].TargetLocator, "dual-user-installed", true)
	verifyClaude := func(label string, want bool) {
		t.Helper()
		entries := claudePluginList(t, claudeFixture, claudeBin, label)
		count := 0
		for _, entry := range entries {
			if entry.ID == claudeID {
				if entry.InstallPath != bindings["claude"].TargetLocator || entry.Scope != "user" || !entry.Enabled {
					t.Fatalf("dual Claude native identity/path differs: %+v", entry)
				}
				count++
			}
		}
		if (count == 1) != want || count > 1 {
			t.Fatalf("dual Claude native plugin presence=%d want=%v", count, want)
		}
		if want {
			details := claudePluginDetails(t, claudeFixture, claudeBin, claudeID, label+"-details")
			if !strings.Contains(details, nativeRawUserSkillName) || !strings.Contains(details, "Skills (1)") {
				t.Fatalf("dual Claude did not discover installed raw Skill: %s", details)
			}
		}
	}
	verifyClaude("dual-user-installed", true)
	beforeState, beforeCodex, beforeClaude := nativeSHA(t, statePath),
		nativeTreeDigest(t, bindings["codex"].TargetLocator), nativeTreeDigest(t, bindings["claude"].TargetLocator)
	result, runErr = commandsRunInstaller(t, f.Root, f.Project, env, installer, "dual-user-default-replay", add...)
	dualNativeRawUserResult(t, result, runErr, "unchanged")
	if nativeSHA(t, statePath) != beforeState ||
		nativeTreeDigest(t, bindings["codex"].TargetLocator) != beforeCodex ||
		nativeTreeDigest(t, bindings["claude"].TargetLocator) != beforeClaude {
		t.Fatal("dual native default user replay altered saved or projected bytes")
	}
	codexNativeRawUserInventory(t, f, codexBin, codexID, declared, bindings["codex"].TargetLocator, "dual-user-repeated", true)
	verifyClaude("dual-user-repeated", true)

	result, runErr = commandsRunInstaller(t, f.Root, f.Project, env, installer, "dual-remove-codex",
		"remove", nativeRawUserSkillName, "--target", "codex", "--scope", "user", "--external-uninstalled")
	nativeRawUserSavedLifecycleResult(t, "codex", "remove", result, runErr)
	if _, err := os.Lstat(bindings["codex"].TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("selected Codex remove left owned raw Skill: %v", err)
	}
	remaining := dualNativeRawUserBindings(t, statePath, 1, f.CodexHome, claudeProfile)
	if !reflect.DeepEqual(remaining["claude"], bindings["claude"]) ||
		nativeTreeDigest(t, remaining["claude"].TargetLocator) != beforeClaude {
		t.Fatal("selected Codex native removal modified retained Claude binding or bytes")
	}
	codexNativeRawUserInventory(t, f, codexBin, codexID, declared, bindings["codex"].TargetLocator, "dual-user-codex-removed", false)
	verifyClaude("dual-user-codex-removed", true)
	result, runErr = commandsRunInstaller(t, f.Root, f.Project, env, installer, "dual-remove-claude",
		"remove", nativeRawUserSkillName, "--target", "claude", "--scope", "user")
	nativeRawUserSavedLifecycleResult(t, "claude", "remove", result, runErr)
	if _, err := os.Lstat(bindings["claude"].TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("selected Claude remove left owned raw Skill: %v", err)
	}
	if nativeSHA(t, foreignCodex) != codexForeignSHA || nativeSHA(t, foreignClaude) != claudeForeignSHA {
		t.Fatal("dual client lifecycle changed a foreign user Skill in either profile")
	}
	if body, err := os.ReadFile(statePath); err != nil {
		t.Fatal(err)
	} else {
		var final domain.StateFileV2
		if err := json.Unmarshal(body, &final); err != nil || len(final.EnvironmentReceipts) != 0 {
			t.Fatalf("dual native teardown retained ownership: %+v %v", final.EnvironmentReceipts, err)
		}
	}
	verifyClaude("dual-user-claude-removed", false)
}

// stateDeclaredNativeSkill reads the exact identity from the shared native
// ledger; the caller has already verified its immutable source authority.
func stateDeclaredNativeSkill(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(body, &state); err != nil || len(state.Installations) != 1 {
		t.Fatalf("dual native source state unavailable: %v", err)
	}
	return state.Installations[0].DeclaredName
}
