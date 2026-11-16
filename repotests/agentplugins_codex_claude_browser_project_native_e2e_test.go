package pluginkitairepo_test

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// This is an additional project case. The required user/global browser
// lifecycle remains separate and keeps its original source and assertions.
func TestAgentpluginsCodexClaudeNativeBrowserProjectSkillsLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_NATIVE_BROWSER_LIVE") != "1" {
		t.Skip("opt-in genuine project browser proof in the Linux Codex lane")
	}
	if runtime.GOOS != "linux" || os.Getenv("AGENTPLUGINS_DUAL_CODEX_CLAUDE_NATIVE_E2E") != "1" {
		t.Fatal("project browser proof requires both qualified native clients on Linux")
	}
	nativeRequireDisposable(t)
	f := newBrowserProjectFixture(t)
	base := f.codex.base
	evidence := map[string]any{"source": nativeAnthropicSource, "scope": "project", "model_request": false,
		"source_identity": nativeBrowserProjectIdentity(t), "installer_sha256": nativeSHA(t, f.codex.installer),
		"codex_sha256": nativeSHA(t, f.codex.client), "claude_sha256": nativeSHA(t, f.claude.client),
		"codex_version": os.Getenv("AGENTPLUGINS_CODEX_VERSION"), "claude_version": os.Getenv("AGENTPLUGINS_CLAUDE_VERSION"),
		"status": "failed"}
	defer func() { nativeJSON(t, filepath.Join(base.Root, "native-project-example-evidence.json"), evidence) }()
	f.trust(t, false)
	f.codexInventory(t, "example-before", map[string]bool{})
	nativeBrowserSkillsScenario(t, base, f.codex.installer, f.env, "project-anthropic", nativeAnthropicSource)
	wanted := map[string]bool{"docx/codex": true, "docx/claude": true, "frontend-design/codex": true, "frontend-design/claude": true}
	original := f.state(t, wanted)
	f.trust(t, true)
	f.codexInventory(t, "example-installed", wanted)
	f.claudeInventory(t, "example-installed", original, true)
	evidence["installed_bindings"] = 4

	before := nativeSHA(t, filepath.Join(base.Project, ".uap", "state-v2.json"))
	owned := browserProjectTrees(t, original)
	result, err := commandsRunInstaller(t, base.Root, base.Project, f.env, f.codex.installer, "example-repeat",
		"skills", "add", nativeAnthropicSource, "--skill", "docx", "--skill", "frontend-design", "--target", "codex,claude",
		"--scope", "project", "--project-root", base.Project, "--no-ui", "--yes")
	browserProjectReadyResult(t, result, err)
	repeated := f.state(t, wanted)
	if nativeSHA(t, filepath.Join(base.Project, ".uap", "state-v2.json")) != before ||
		!reflect.DeepEqual(original, repeated) || !maps.Equal(owned, browserProjectTrees(t, repeated)) {
		t.Fatal("unchanged four-binding replay altered saved ownership or full native payloads")
	}
	f.trust(t, true)
	f.codexInventory(t, "example-repeated", wanted)
	f.claudeInventory(t, "example-repeated", original, true)
	evidence["repeat_readiness"] = "ready"

	// Remove just one target of one Skill first. The other three exact bindings
	// and the selected Claude trust must survive this ordinary public command.
	f.remove(t, "example-selective-remove", original["docx/codex"])
	delete(wanted, "docx/codex")
	remaining := f.state(t, wanted)
	for key, item := range remaining {
		if !reflect.DeepEqual(item, original[key]) {
			t.Fatalf("selective removal changed sibling %s", key)
		}
	}
	for path, digest := range browserProjectTrees(t, remaining) {
		if owned[path] != digest {
			t.Fatal("selective removal changed a remaining owned tree")
		}
	}
	f.codexInventory(t, "example-selective-removed", wanted)
	f.claudeInventory(t, "example-selective-removed", original, true)
	evidence["selective_remaining_bindings"] = 3
	for _, key := range []string{"frontend-design/codex", "docx/claude", "frontend-design/claude"} {
		f.remove(t, "example-final-remove-"+strings.ReplaceAll(key, "/", "-"), original[key])
	}
	f.state(t, map[string]bool{})
	f.codexInventory(t, "example-removed", map[string]bool{})
	f.claudeInventory(t, "example-removed", original, false)
	f.trust(t, true)
	evidence["status"], evidence["final_bindings"], evidence["foreign_preserved"] = "passed", 0, true
}

func newBrowserProjectFixture(t *testing.T) *browserProjectFixture {
	t.Helper()
	codex := newCodexProjectNativeFixture(t)
	base := codex.base
	nativeProvisionScanner(t, base)
	claudeBin := claudeNativeBinary(t, "AGENTPLUGINS_CLAUDE_BIN")
	if want := os.Getenv("AGENTPLUGINS_CLAUDE_SHA256"); len(want) != 64 || nativeSHA(t, claudeBin) != want {
		t.Fatal("project browser Claude executable pin mismatch")
	}
	profile := filepath.Join(base.Root, "claude-config")
	claudeBase := &claudeNativeFixture{Root: base.Root, Home: base.Home, ConfigDir: profile,
		StateHome: filepath.Join(base.Root, "installer-state"), transcriptSHA256: map[string]string{}}
	claude := &claudeProjectNativeFixture{base: claudeBase, client: claudeBin, installer: codex.installer,
		project: base.Project, otherProject: codex.otherProject, foreign: map[string]string{}}
	for _, root := range []string{profile, filepath.Join(base.Project, ".claude"), filepath.Join(codex.otherProject, ".claude")} {
		claude.keep(t, filepath.Join(root, "settings.json"), `{"enabledPlugins":{"foreign-project-fixture@skills-dir":false}}`)
		claude.keep(t, filepath.Join(root, "skills", "foreign-project-fixture", "SKILL.md"), "---\nname: foreign-project-fixture\ndescription: Inert foreign Skill\n---\nPreserve these bytes.\n")
	}
	foreign := map[string]any{"hasTrustDialogAccepted": true, "allowedTools": []any{}}
	nativeJSON(t, filepath.Join(profile, ".claude.json"), map[string]any{"projects": map[string]any{filepath.ToSlash(codex.otherProject): foreign}})
	// Use a genuinely registered foreign plugin: Claude may garbage-collect
	// orphan cache leaves independently of any installer operation.
	claudeProjectSeedForeignCache(t, claude)
	f := &browserProjectFixture{codex: codex, claude: claude, pins: nativeAnthropicPins(t), foreignTrust: foreign}
	f.env = append(base.env(filepath.Dir(codex.client)), "CLAUDE_CONFIG_DIR="+profile, "DISABLE_AUTOUPDATER=1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
	version, err := nativeCommand(t, base, codex.client, "example-codex-version", "--version")
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli "+os.Getenv("AGENTPLUGINS_CODEX_VERSION") ||
		claudeVersionString(t, claudeBase, claudeBin) != os.Getenv("AGENTPLUGINS_CLAUDE_VERSION") {
		t.Fatal("project browser native version does not match the acquired official pin", err)
	}
	// Native SDK initialization can create its own profile metadata. Prime it
	// before UAP and capture the baseline once; no post-install rebaselining.
	claudeProjectInitializedSkills(t, claude, "example-before", nil, false)
	body, err := os.ReadFile(filepath.Join(profile, ".claude.json"))
	var document struct{ Projects map[string]map[string]any }
	if err != nil || json.Unmarshal(body, &document) != nil {
		t.Fatal("read initialized native profile", err)
	}
	f.foreignTrust = document.Projects[filepath.ToSlash(codex.otherProject)]
	if f.foreignTrust["hasTrustDialogAccepted"] != true {
		t.Fatal("native baseline lost unrelated project trust")
	}
	config, err := os.ReadFile(filepath.Join(base.CodexHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	nativeJSON(t, filepath.Join(base.Root, "native-project-baseline.json"), map[string]any{
		"other_project": codex.otherProject, "foreign_trust": f.foreignTrust, "codex_config": string(config)})
	// Only the selected project's enabledPlugins entries may be added by UAP;
	// requireForeign continues to check its pre-existing disabled foreign key.
	delete(claude.foreign, filepath.Join(base.Project, ".claude", "settings.json"))
	f.trust(t, false)
	return f
}

func nativeBrowserProjectIdentity(t *testing.T) any {
	t.Helper()
	identity, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func browserProjectTrees(t *testing.T, bindings map[string]browserProjectBinding) map[string]string {
	t.Helper()
	trees := map[string]string{}
	for _, item := range bindings {
		for _, path := range []string{item.binding.TargetLocator, item.payload, item.cache} {
			if path != "" {
				trees[path] = nativeTreeDigest(t, path)
			}
		}
	}
	return trees
}

func (f *browserProjectFixture) remove(t *testing.T, label string, item browserProjectBinding) {
	t.Helper()
	base := f.codex.base
	result, err := commandsRunInstaller(t, base.Root, base.Project, f.env, f.codex.installer, label,
		"remove", item.alias, "--target", item.binding.ClientID, "--scope", "project", "--project-root", base.Project)
	nativeRawUserSavedLifecycleResult(t, item.binding.ClientID, "remove", result, err)
	// These are the exact prior receipt-owned paths. Shared store packages and
	// unrelated native cache versions are intentionally outside this assertion.
	for _, path := range []string{item.binding.TargetLocator, item.payload, item.cache} {
		if path == "" {
			continue
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("selected owned path survived removal: %s: %v", path, err)
		}
	}
	f.trust(t, true)
}
