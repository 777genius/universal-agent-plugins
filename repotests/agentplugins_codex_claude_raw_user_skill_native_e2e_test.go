package pluginkitairepo_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This immutable Agent Skills 1.0 fixture has no commands or credentials.
// The native suite fetches it through the actual public CLI, rather than
// injecting a Git acquisition adapter or asserting only prepared files.
const nativeRawUserSkillCommit = "7155140441b93db3a7b36dcd7e00291cafe608f9"
const nativeRawUserSkillSource = "777genius/universal-agent-plugins@" + nativeRawUserSkillCommit + "//tests/fixtures/uap_apm/skills/benchmark-note"
const nativeRawUserSkillName = "benchmark-note"
const nativeRawUserSkillSourceKey = "github-skill:777genius/universal-agent-plugins:benchmark-note:tests/fixtures/uap_apm/skills/benchmark-note"

// nativeRawUserInstall Runs the public skills-add CLI against one native user target and requires an exact outcome.
func nativeRawUserInstall(t *testing.T, root, dir string, env []string, installer, target, label string, explicit bool, repeated bool) {
	t.Helper()
	args := []string{"skills", "add", nativeRawUserSkillSource, "--skill", nativeRawUserSkillName, "--target", target, "--no-ui", "--yes"}
	if explicit {
		args = append(args, "--scope", "user")
	}
	result, err := commandsRunInstaller(t, root, dir, env, installer, label, args...)
	items, _ := result["items"].([]any)
	if err != nil || result["status"] != "succeeded" || len(items) != 1 {
		t.Fatalf("%s: native raw Skill CLI failed: %v; %+v", label, err, result)
	}
	item, _ := items[0].(map[string]any)
	status, _ := item["status"].(string)
	if !repeated && status != "succeeded" || repeated && status != "unchanged" {
		t.Fatalf("%s: unexpected per-target raw Skill outcome: %v", label, result)
	}
}

// nativeRawUserState Validates the persisted immutable Skill, ownership receipt and selected user profile.
func nativeRawUserState(t *testing.T, statePath, target, profile string, requiredVerification domain.VerificationState) (domain.StateFileV2, domain.ClientBinding) {
	t.Helper()
	body, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 1 {
		t.Fatalf("%s: expected exactly one raw Skill and receipt: %+v", target, state)
	}
	installed := state.Installations[0]
	// Raw Skills normalize into an immutable synthetic plugin identity keyed
	// by the exact source. It must never equal or shadow a user's Skill name.
	sum := sha256.Sum256([]byte(installed.Source.StableSourceKey))
	expectedName := "skill-" + hex.EncodeToString(sum[:])[:56]
	receipt := state.EnvironmentReceipts[0]
	binding, ok := installed.Clients[receipt.ClientBindingID]
	if !ok || len(installed.Clients) != 1 ||
		installed.Source.StableSourceKey != nativeRawUserSkillSourceKey ||
		installed.DeclaredName != expectedName ||
		installed.Source.ResolvedRevision != nativeRawUserSkillCommit ||
		receipt.InstallationID != installed.InstallationID ||
		receipt.ClientID != target || receipt.ClientBindingID != binding.ClientBindingID ||
		binding.ClientID != target || binding.Scope != "user" ||
		binding.TargetLocator == "" || binding.PhysicalArtifact == "" ||
		binding.ClientBindingID == "" ||
		((target == "codex" || binding.NativeProfileRoot != "") && binding.NativeProfileRoot != profile) ||
		binding.Verification != requiredVerification {
		t.Fatalf("%s: raw Skill lost immutable user/native identity: %+v; %+v", target, installed, binding)
	}
	// Claude's projected delivery binds the selected native profile through
	// its exact skills-directory path and authoritative CLI inventory; the
	// optional NativeProfileRoot field need not be populated for this adapter.
	if target == "claude" && filepath.Dir(binding.TargetLocator) != filepath.Join(profile, "skills") {
		t.Fatalf("Claude raw Skill escaped selected user profile: %s", binding.TargetLocator)
	}
	return state, binding
}

// nativeRawUserFiles Compares the projected Skill and reference bytes with the pinned Git source.
func nativeRawUserFiles(t *testing.T, binding domain.ClientBinding, client string) {
	t.Helper()
	root := binding.TargetLocator
	if client == "claude" {
		root = filepath.Join(root, ".agentplugins-runtime")
	}
	for relative, digest := range map[string]string{
		"skills/benchmark-note/SKILL.md":               "53e39c4f1dcaa4c2ed1196654678ede091b71fb837f77a1beb07ba556abc1198",
		"skills/benchmark-note/references/fixture.txt": "674bbf87f1469969103f15f60ea1389829fe431dfca5439da1fc57da211d1f2f",
	} {
		if actual := nativeSHA(t, filepath.Join(root, filepath.FromSlash(relative))); actual != digest {
			t.Fatalf("%s: normalized Skill changed pinned Agent Skills 1.0 bytes at %s: %s", client, relative, actual)
		}
	}
}

// nativeRawUserSavedLifecycleResult Requires a successful single-target saved repair/remove outcome.
// Saved dependency repair/removal uses the public outcome/results JSON
// contract, not the single-plugin result/data or skills-add status/items envelopes.
func nativeRawUserSavedLifecycleResult(t *testing.T, target, operation string, result map[string]any, runErr error) {
	t.Helper()
	entries, ok := result["results"].([]any)
	if runErr != nil || result["outcome"] != "completed" || !ok || len(entries) != 1 {
		t.Fatalf("%s %s did not complete: err=%v result=%+v", target, operation, runErr, result)
	}
	entry, ok := entries[0].(map[string]any)
	if !ok || entry["client_id"] != target || entry["operation"] != operation || entry["outcome"] != "completed" {
		t.Fatalf("%s %s result is not a completed exact-target mutation: %+v", target, operation, result)
	}
}

// nativeRawUserForeignDriftRefusal proves that a raw Skill's recorded ownership
// cannot authorize deleting someone else's files, even inside an owned path.
// Both attempts run the public CLI with genuine detected native client state.
func nativeRawUserForeignDriftRefusal(t *testing.T, root, dir string, env []string, installer, target, statePath string, binding domain.ClientBinding) {
	t.Helper()
	nativeRequireContained(t, root, binding.TargetLocator)
	beforeState := nativeSHA(t, statePath)
	beforeTree := nativeTreeDigest(t, binding.TargetLocator)
	foreign := filepath.Join(binding.TargetLocator, "foreign-injected.txt")
	nativeWrite(t, foreign, []byte("foreign user data: do not adopt or delete"), 0600)
	foreignSHA := nativeSHA(t, foreign)
	damagedTree := nativeTreeDigest(t, binding.TargetLocator)
	for _, operation := range []string{"repair", "remove"} {
		args := []string{operation, nativeRawUserSkillName, "--target", target, "--scope", "user"}
		if operation == "remove" && target == "codex" {
			args = append(args, "--external-uninstalled")
		}
		label := "raw-user-foreign-" + target + "-" + operation
		result, runErr := commandsRunInstaller(t, root, dir, env, installer, label, args...)
		stderr, err := os.ReadFile(filepath.Join(root, label+"-installer-stderr.log"))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(result)
		diagnostic := string(stderr) + "\n" + string(body)
		if runErr == nil || (!strings.Contains(diagnostic, "artifact digest mismatch") &&
			!strings.Contains(diagnostic, "native identity ownership is indeterminate")) {
			t.Fatalf("%s %s did not refuse modified raw Skill ownership: err=%v diagnostic=%s", target, operation, runErr, diagnostic)
		}
		if nativeSHA(t, foreign) != foreignSHA || nativeSHA(t, statePath) != beforeState ||
			nativeTreeDigest(t, binding.TargetLocator) != damagedTree {
			t.Fatalf("%s %s altered foreign bytes, the saved ledger, or the installed raw Skill after refusal", target, operation)
		}
	}
	if err := os.Remove(foreign); err != nil {
		t.Fatal(err)
	}
	if nativeTreeDigest(t, binding.TargetLocator) != beforeTree ||
		nativeSHA(t, statePath) != beforeState {
		t.Fatalf("%s failed to restore exact original owned raw Skill after foreign-file refusal", target)
	}
}

// nativeRawUserRepair Deletes only a disposable owned artifact, then proves the saved repair restores it.
func nativeRawUserRepair(t *testing.T, root, dir string, env []string, installer, target, statePath, profile string, original domain.ClientBinding) {
	t.Helper()
	// This test deliberately deletes a managed artifact to reproduce absent
	// repair. Never allow a bad persisted path to escape the disposable host.
	nativeRequireContained(t, root, original.TargetLocator)
	if err := os.RemoveAll(original.TargetLocator); err != nil {
		t.Fatal(err)
	}
	result, err := commandsRunInstaller(t, root, dir, env, installer, "raw-user-repair", "repair", nativeRawUserSkillName, "--target", target, "--scope", "user")
	nativeRawUserSavedLifecycleResult(t, target, "repair", result, err)
	// Saved Codex repair restores the owned package conservatively as
	// package_validated. The caller independently requires real Codex native
	// plugin and skills/list discovery before accepting the repaired installation.
	// Claude repair already records installation_verified.
	requiredVerification := domain.VerificationInstalled
	if target == "codex" {
		requiredVerification = domain.VerificationPackageValid
	}
	_, restored := nativeRawUserState(t, statePath, target, profile, requiredVerification)
	if restored.ClientBindingID != original.ClientBindingID || restored.PhysicalArtifact != original.PhysicalArtifact ||
		restored.TargetLocator != original.TargetLocator {
		t.Fatalf("%s: repair replaced immutable Skill identity: before=%+v after=%+v", target, original, restored)
	}
	nativeRawUserFiles(t, restored, target)
}

// nativeRawUserRemove Requires the selected owned artifact and receipt to disappear after removal.
func nativeRawUserRemove(t *testing.T, root, dir string, env []string, installer, target, statePath, active string, external bool) {
	t.Helper()
	args := []string{"remove", nativeRawUserSkillName, "--target", target, "--scope", "user"}
	if external {
		args = append(args, "--external-uninstalled")
	}
	result, err := commandsRunInstaller(t, root, dir, env, installer, "raw-user-remove", args...)
	nativeRawUserSavedLifecycleResult(t, target, "remove", result, err)
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("%s: managed raw Skill survived removal: %v", target, err)
	}
	body, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("%s: removal left a live environment receipt: %+v", target, state.EnvironmentReceipts)
	}
	for _, installation := range state.Installations {
		for key, binding := range installation.Clients {
			if binding.ClientID == target {
				t.Fatalf("%s: removed native binding is still registered under %s", target, key)
			}
		}
	}
}

// codexNativeRawUserInventory Cross-checks a native Codex plugin listing and app-server skills/list against the target.
func codexNativeRawUserInventory(t *testing.T, f *nativeFixture, client, pluginID, declaredName, active, label string, want bool) {
	t.Helper()
	listing, err := nativeCommand(t, f, client, label+"-plugins", "plugin", "list", "--json")
	if err != nil {
		t.Fatalf("Codex native plugin list: %v; %s", err, listing)
	}
	var inventory struct {
		Installed *[]struct {
			ID        string `json:"pluginId"`
			Installed bool   `json:"installed"`
			Enabled   bool   `json:"enabled"`
			Source    struct {
				Path string `json:"path"`
			} `json:"source"`
		} `json:"installed"`
	}
	if err := json.Unmarshal(listing, &inventory); err != nil {
		t.Fatal(err)
	}
	// A missing/null "installed" field is an unknown native protocol, not
	// evidence of absence after remove. Require the production adapter's
	// entire fail-closed contract, including unique identities and valid
	// schema for unrelated entries, before checking the exact source path.
	marketplace, ok := strings.CutPrefix(pluginID, declaredName+"@")
	if !ok || marketplace == "" || inventory.Installed == nil {
		t.Fatalf("Codex native listing has no valid installed identity: %s", listing)
	}
	status := codex.PluginStatusFromList(listing, declaredName, marketplace)
	if status == codex.StatusUnknown ||
		(want && status != codex.StatusInstalled) ||
		(!want && status != codex.StatusAbsent) {
		t.Fatalf("Codex native listing was invalid or had wrong status %v, want installed=%v: %s", status, want, listing)
	}
	present := 0
	for _, plugin := range *inventory.Installed {
		if plugin.ID == pluginID {
			if !plugin.Installed || !plugin.Enabled || plugin.Source.Path != active {
				t.Fatalf("Codex raw Skill native registration is not exact: %+v", plugin)
			}
			present++
		}
	}
	if (present == 1) != want || present > 1 {
		t.Fatalf("Codex native plugin presence=%d want=%v", present, want)
	}

	r := startNativeRPC(t, f, client, label)
	defer r.close()
	r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_raw_user_skill", "version": "1.0.0"}, "capabilities": map[string]any{"experimentalApi": true}})
	if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	var skills struct {
		Data []struct {
			CWD    string            `json:"cwd"`
			Errors []json.RawMessage `json:"errors"`
			Skills []struct {
				Name     string `json:"name"`
				Path     string `json:"path"`
				PluginID string `json:"pluginId"`
				Enabled  bool   `json:"enabled"`
			} `json:"skills"`
		} `json:"data"`
	}
	if err := json.Unmarshal(r.must(t, "skills/list", map[string]any{"cwds": []string{f.Project}, "forceReload": true}), &skills); err != nil {
		t.Fatal(err)
	}
	if len(skills.Data) != 1 || !openCodeSamePath(skills.Data[0].CWD, f.Project) ||
		len(skills.Data[0].Errors) != 0 {
		t.Fatalf("Codex user Skills inventory missing exact cwd or contains errors: %+v", skills.Data)
	}
	found := 0
	for _, group := range skills.Data {
		for _, skill := range group.Skills {
			if skill.PluginID != pluginID {
				continue
			}
			if skill.Name != declaredName+":"+nativeRawUserSkillName || !skill.Enabled ||
				!strings.HasSuffix(filepath.ToSlash(skill.Path), "/skills/benchmark-note/SKILL.md") ||
				!strings.Contains(filepath.ToSlash(skill.Path), "/plugins/cache/") {
				t.Fatalf("Codex native raw Skill identity mismatch: %+v", skill)
			}
			nativeRequireContained(t, f.Root, skill.Path)
			if got := nativeSHA(t, skill.Path); got != "53e39c4f1dcaa4c2ed1196654678ede091b71fb837f77a1beb07ba556abc1198" {
				t.Fatalf("Codex native Skill body differs from pinned Git source: %s", got)
			}
			found++
		}
	}
	if (found == 1) != want || found > 1 {
		t.Fatalf("Codex native skills/list found=%d want=%v", found, want)
	}
}

// TestAgentpluginsCodexNativeRawUserSkillLifecycle Proves real pinned Codex user Skill add, repeat, repair, discovery and removal.
func TestAgentpluginsCodexNativeRawUserSkillLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CODEX_NATIVE_E2E") != "1" {
		t.Skip("opt-in genuine Codex native user Skill E2E")
	}
	nativeRequireDisposable(t)
	client, installer := nativeBinary(t, "AGENTPLUGINS_CODEX_BIN"), nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	if expected := os.Getenv("AGENTPLUGINS_CODEX_SHA256"); len(expected) != 64 || nativeSHA(t, client) != expected {
		t.Fatal("genuine Codex executable SHA-256 pin mismatch")
	}
	f := newNativeFixture(t, true)
	f.ClientBinDir = filepath.Dir(client)
	nativeProvisionScanner(t, f)
	version, err := nativeCommand(t, f, client, "raw-user-version", "--version")
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli "+os.Getenv("AGENTPLUGINS_CODEX_VERSION") {
		t.Fatalf("wrong pinned Codex version: %q; %v", version, err)
	}
	nativeWrite(t, filepath.Join(f.CodexHome, "config.toml"), []byte("cli_auth_credentials_store = \"file\"\nmcp_oauth_credentials_store = \"file\"\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n"), 0600)
	foreign := filepath.Join(f.CodexHome, "skills", "foreign-raw-user", "SKILL.md")
	nativeWrite(t, foreign, []byte("---\nname: foreign-raw-user\ndescription: Unowned user Skill\n---\nPreserve me.\n"), 0600)
	foreignSHA := nativeSHA(t, foreign)
	env := f.env(filepath.Dir(client))
	statePath := filepath.Join(f.Root, "installer-state", "state-v2.json")
	nativeRawUserInstall(t, f.Root, f.Project, env, installer, "codex", "raw-user-add", true, false)
	state, binding := nativeRawUserState(t, statePath, "codex", f.CodexHome, domain.VerificationInstalled)
	nativeRawUserFiles(t, binding, "codex")
	declaredName := state.Installations[0].DeclaredName
	id := declaredName + "@" + shared.ManagedMarketplaceName(binding.PhysicalArtifact)
	codexNativeRawUserInventory(t, f, client, id, declaredName, binding.TargetLocator, "raw-user-installed", true)
	beforeState, beforeTree := nativeSHA(t, statePath), nativeTreeDigest(t, binding.TargetLocator)
	nativeRawUserInstall(t, f.Root, f.Project, env, installer, "codex", "raw-user-repeat-default", false, true)
	if nativeSHA(t, statePath) != beforeState || nativeTreeDigest(t, binding.TargetLocator) != beforeTree {
		t.Fatal("Codex implicit user replay changed managed bytes or receipts")
	}
	codexNativeRawUserInventory(t, f, client, id, declaredName, binding.TargetLocator, "raw-user-repeated", true)
	nativeRawUserForeignDriftRefusal(t, f.Root, f.Project, env, installer, "codex", statePath, binding)
	codexNativeRawUserInventory(t, f, client, id, declaredName, binding.TargetLocator, "raw-user-foreign-refused", true)
	nativeRawUserRepair(t, f.Root, f.Project, env, installer, "codex", statePath, f.CodexHome, binding)
	codexNativeRawUserInventory(t, f, client, id, declaredName, binding.TargetLocator, "raw-user-repaired", true)
	nativeRawUserRemove(t, f.Root, f.Project, env, installer, "codex", statePath, binding.TargetLocator, true)
	codexNativeRawUserInventory(t, f, client, id, declaredName, binding.TargetLocator, "raw-user-removed", false)
	if nativeSHA(t, foreign) != foreignSHA {
		t.Fatal("Codex raw user lifecycle changed a foreign user Skill")
	}
}

// TestAgentpluginsClaudeNativeRawUserSkillLifecycle Proves real pinned Claude user Skill add, repeat, repair, discovery and removal.
func TestAgentpluginsClaudeNativeRawUserSkillLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CLAUDE_NATIVE_E2E") != "1" {
		t.Skip("opt-in genuine Claude Code native user Skill E2E")
	}
	nativeRequireDisposable(t)
	client, installer := claudeNativeBinary(t, "AGENTPLUGINS_CLAUDE_BIN"), claudeNativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	if expected := os.Getenv("AGENTPLUGINS_CLAUDE_SHA256"); len(expected) != 64 || claudeSHA256(t, client) != expected {
		t.Fatal("genuine Claude Code executable SHA-256 pin mismatch")
	}
	f := newClaudeNativeFixture(t)
	clientDir := filepath.Dir(client)
	if got := claudeVersionString(t, f, client); got != os.Getenv("AGENTPLUGINS_CLAUDE_VERSION") {
		t.Fatalf("wrong pinned Claude Code version %q", got)
	}
	nativeProvisionScanner(t, &nativeFixture{Root: f.Root})
	foreign := filepath.Join(f.ConfigDir, "skills", "foreign-raw-user", "SKILL.md")
	nativeWrite(t, foreign, []byte("---\nname: foreign-raw-user\ndescription: Unowned user Skill\n---\nPreserve me.\n"), 0600)
	foreignSHA := nativeSHA(t, foreign)
	env := f.env(clientDir)
	statePath := filepath.Join(f.StateHome, "state-v2.json")
	nativeRawUserInstall(t, f.Root, f.Root, env, installer, "claude", "raw-user-add", true, false)
	state, binding := nativeRawUserState(t, statePath, "claude", f.ConfigDir, domain.VerificationInstalled)
	nativeRawUserFiles(t, binding, "claude")
	pluginID := state.Installations[0].DeclaredName + "@skills-dir"
	verify := func(label string, want bool) {
		t.Helper()
		entries := claudePluginList(t, f, client, label)
		found := 0
		for _, entry := range entries {
			if entry.ID != pluginID {
				continue
			}
			if !entry.Enabled || entry.InstallPath != binding.TargetLocator || entry.Scope != "user" {
				t.Fatalf("Claude native raw Skill registration mismatch: %+v", entry)
			}
			found++
		}
		if (found == 1) != want || found > 1 {
			t.Fatalf("Claude plugin list contains %d raw user entries, want=%v", found, want)
		}
	}
	verify("raw-user-installed", true)
	// Listing the native plugin is insufficient evidence that its raw Skill
	// remains discoverable after replay or repair: query the real CLI details
	// for every accepted state, including the restored installation.
	verifySkill := func(label string) {
		t.Helper()
		details := claudePluginDetails(t, f, client, pluginID, label)
		if !strings.Contains(details, nativeRawUserSkillName) || !strings.Contains(details, "Skills (1)") {
			t.Fatalf("Claude plugin details did not report exactly one installed raw Skill: %s", details)
		}
	}
	verifySkill("raw-user-installed-details")
	beforeState, beforeTree := claudeSHA256(t, statePath), nativeTreeDigest(t, binding.TargetLocator)
	nativeRawUserInstall(t, f.Root, f.Root, env, installer, "claude", "raw-user-repeat-default", false, true)
	if claudeSHA256(t, statePath) != beforeState || nativeTreeDigest(t, binding.TargetLocator) != beforeTree {
		t.Fatal("Claude implicit user replay changed managed bytes or receipts")
	}
	verify("raw-user-repeated", true)
	verifySkill("raw-user-repeated-details")
	nativeRawUserForeignDriftRefusal(t, f.Root, f.Root, env, installer, "claude", statePath, binding)
	verify("raw-user-foreign-refused", true)
	verifySkill("raw-user-foreign-refused-details")
	nativeRawUserRepair(t, f.Root, f.Root, env, installer, "claude", statePath, f.ConfigDir, binding)
	verify("raw-user-repaired", true)
	verifySkill("raw-user-repaired-details")
	nativeRawUserRemove(t, f.Root, f.Root, env, installer, "claude", statePath, binding.TargetLocator, false)
	verify("raw-user-removed", false)
	if nativeSHA(t, foreign) != foreignSHA {
		t.Fatal("Claude raw user lifecycle changed a foreign user Skill")
	}
}
