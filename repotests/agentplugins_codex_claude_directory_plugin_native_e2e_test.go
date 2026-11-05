package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const directorySitePlugin = "context7"
const directorySiteDistribution = "upstash/context7"
const directorySiteRevision = "769c6cd22c3d95462d1f55d789e9532cabefa5a9"
const directorySiteSkill = "context7-mcp"

// directorySiteBindings checks actual saved Directory origin and two distinct
// installed native profile receipts. An arbitrary local bundle is not sufficient.
func directorySiteBindings(t *testing.T, path, codexHome, claudeHome string) map[string]domain.ClientBinding {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(data, &state); err != nil || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 2 {
		t.Fatalf("Directory site install lost saved ownership: %v, %+v", err, state)
	}
	install := state.Installations[0]
	if install.DeclaredName != directorySitePlugin || install.Directory == nil ||
		install.OriginMode != domain.OriginModeDirectory ||
		install.Directory.ProductID != directorySitePlugin || install.Directory.DistributionID != directorySiteDistribution ||
		install.Directory.DesiredReleaseSequence == 0 ||
		install.Source.ResolvedRevision != directorySiteRevision || len(install.Clients) != 2 {
		t.Fatalf("site alias did not resolve signed Context7 distribution: %+v", install)
	}
	bindings := map[string]domain.ClientBinding{}
	for _, receipt := range state.EnvironmentReceipts {
		binding, ok := install.Clients[receipt.ClientBindingID]
		if !ok || receipt.InstallationID != install.InstallationID ||
			binding.ClientBindingID != receipt.ClientBindingID || binding.ClientID != receipt.ClientID || binding.Scope != "user" ||
			binding.Verification != domain.VerificationInstalled || binding.TargetLocator == "" || binding.PhysicalArtifact == "" {
			t.Fatalf("Directory receipt does not match installed native binding: %+v %+v", receipt, binding)
		}
		switch binding.ClientID {
		case "codex":
			if binding.NativeProfileRoot != codexHome {
				t.Fatalf("Codex Directory profile differs from selected HOME: %+v", binding)
			}
		case "claude":
			if filepath.Dir(binding.TargetLocator) != filepath.Join(claudeHome, "skills") ||
				(binding.NativeProfileRoot != "" && binding.NativeProfileRoot != claudeHome) {
				t.Fatalf("Claude Directory profile differs from selected HOME: %+v", binding)
			}
		default:
			t.Fatalf("unexpected Directory target %q", binding.ClientID)
		}
		if _, duplicate := bindings[binding.ClientID]; duplicate {
			t.Fatalf("duplicate Directory client binding %q", binding.ClientID)
		}
		bindings[binding.ClientID] = binding
	}
	if len(bindings) != 2 {
		t.Fatalf("missing Codex or Claude Directory binding: %+v", bindings)
	}
	return bindings
}

// directorySiteCodexInventory asks the genuine pinned Codex CLI and its
// app-server, never inferring native discovery from the installer's JSON.
func directorySiteCodexInventory(t *testing.T, f *nativeFixture, codexBin string, binding domain.ClientBinding, present bool, label string) {
	t.Helper()
	pluginID := directorySitePlugin + "@" + shared.ManagedMarketplaceName(binding.PhysicalArtifact)
	result, err := nativeCommand(t, f, codexBin, label+"-list", "plugin", "list", "--json")
	if err != nil {
		t.Fatalf("Codex plugin list: %v %s", err, result)
	}
	var listing struct {
		Installed []struct {
			ID        string `json:"pluginId"`
			Installed bool   `json:"installed"`
			Enabled   bool   `json:"enabled"`
			Source    struct {
				Path string `json:"path"`
			} `json:"source"`
		} `json:"installed"`
	}
	if err := json.Unmarshal(result, &listing); err != nil {
		t.Fatal(err)
	}
	marketplace, ok := strings.CutPrefix(pluginID, directorySitePlugin+"@")
	if !ok || marketplace == "" || listing.Installed == nil {
		t.Fatalf("Codex Directory native listing is not authoritative: %s", result)
	}
	status := codex.PluginStatusFromList(result, directorySitePlugin, marketplace)
	if status == codex.StatusUnknown ||
		(present && status != codex.StatusInstalled) || (!present && status != codex.StatusAbsent) {
		t.Fatalf("Codex Directory native listing status %v, want present=%v: %s", status, present, result)
	}
	found := 0
	for _, entry := range listing.Installed {
		if entry.ID == pluginID {
			if !entry.Enabled || !entry.Installed || entry.Source.Path != binding.TargetLocator {
				t.Fatalf("Codex Directory source/path mismatch: %+v", entry)
			}
			found++
		}
	}
	if (found == 1) != present || found > 1 {
		t.Fatalf("Codex plugin presence %d, want %t: %s", found, present, result)
	}
	r := startNativeRPC(t, f, codexBin, label)
	defer r.close()
	r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_directory_fixture", "version": "1.0.0"}, "capabilities": map[string]any{"experimentalApi": true}})
	if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	var skills struct {
		Data []struct {
			CWD    string            `json:"cwd"`
			Errors []json.RawMessage `json:"errors"`
			Skills []struct {
				Name     string `json:"name"`
				PluginID string `json:"pluginId"`
				Path     string `json:"path"`
				Enabled  bool   `json:"enabled"`
			} `json:"skills"`
		} `json:"data"`
		NextCursor *string `json:"nextCursor"`
	}
	if err := json.Unmarshal(r.must(t, "skills/list", map[string]any{"cwds": []string{f.Project}, "forceReload": true}), &skills); err != nil {
		t.Fatal(err)
	}
	if skills.NextCursor != nil || len(skills.Data) != 1 ||
		!openCodeSamePath(skills.Data[0].CWD, f.Project) || len(skills.Data[0].Errors) != 0 {
		t.Fatalf("Codex Directory skills/list missing selected cwd or contained errors: %+v", skills)
	}
	found = 0
	for _, group := range skills.Data {
		for _, skill := range group.Skills {
			if skill.PluginID != pluginID {
				continue
			}
			if skill.Name != directorySitePlugin+":"+directorySiteSkill || !skill.Enabled ||
				!strings.HasSuffix(filepath.ToSlash(skill.Path), "/skills/"+directorySiteSkill+"/SKILL.md") {
				t.Fatalf("Codex Directory Skill name/path mismatch: %+v", skill)
			}
			nativeRequireContained(t, f.Root, skill.Path)
			if _, err := os.Stat(skill.Path); err != nil {
				t.Fatal(err)
			}
			found++
		}
	}
	if (found == 1) != present || found > 1 {
		t.Fatalf("Codex Directory native skills count=%d, want present=%t", found, present)
	}
}

func TestAgentpluginsCodexClaudeNativeDirectoryPluginLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_DUAL_CODEX_CLAUDE_NATIVE_E2E") != "1" {
		t.Skip("opt-in real pinned Codex and Claude Directory plugin E2E")
	}
	nativeRequireDisposable(t)
	codexBin := nativeBinary(t, "AGENTPLUGINS_CODEX_BIN")
	claudeBin := claudeNativeBinary(t, "AGENTPLUGINS_CLAUDE_BIN")
	installer := nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	if want := os.Getenv("AGENTPLUGINS_CODEX_SHA256"); len(want) != 64 || nativeSHA(t, codexBin) != want {
		t.Fatal("Codex binary pin mismatch")
	}
	if want := os.Getenv("AGENTPLUGINS_CLAUDE_SHA256"); len(want) != 64 || claudeSHA256(t, claudeBin) != want {
		t.Fatal("Claude binary pin mismatch")
	}

	f := newNativeFixture(t, true)
	f.ClientBinDir = filepath.Dir(codexBin)
	nativeProvisionScanner(t, f)
	nativeWrite(t, filepath.Join(f.CodexHome, "config.toml"), []byte("cli_auth_credentials_store = \"file\"\nmcp_oauth_credentials_store = \"file\"\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n"), 0600)
	claudeHome := filepath.Join(f.Root, "claude-config")
	if err := os.MkdirAll(claudeHome, 0700); err != nil {
		t.Fatal(err)
	}
	claude := &claudeNativeFixture{Root: f.Root, Home: f.Home, ConfigDir: claudeHome,
		StateHome: filepath.Join(f.Root, "installer-state"), transcriptSHA256: map[string]string{}}
	env := append(f.env(filepath.Dir(codexBin)), "CLAUDE_CONFIG_DIR="+claudeHome,
		"DISABLE_AUTOUPDATER=1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
	statePath := filepath.Join(f.Root, "installer-state", "state-v2.json")
	foreignCodex := filepath.Join(f.CodexHome, "skills", "foreign-site-plugin", "SKILL.md")
	foreignClaude := filepath.Join(claudeHome, "skills", "foreign-site-plugin", "SKILL.md")
	nativeWrite(t, foreignCodex, []byte("---\nname: foreign-site-plugin\ndescription: Foreign Codex user Skill\n---\nMust not change.\n"), 0600)
	nativeWrite(t, foreignClaude, []byte("---\nname: foreign-site-plugin\ndescription: Foreign Claude user Skill\n---\nMust not change.\n"), 0600)
	codexForeignSHA, claudeForeignSHA := nativeSHA(t, foreignCodex), nativeSHA(t, foreignClaude)
	verifyForeign := func(stage string) {
		t.Helper()
		if nativeSHA(t, foreignCodex) != codexForeignSHA || nativeSHA(t, foreignClaude) != claudeForeignSHA {
			t.Fatalf("%s altered foreign user Skill bytes", stage)
		}
	}
	// This is the Directory card's actual command, with only non-interactive
	// consent and JSON output added for a disposable CI host.
	args := []string{"add", directorySitePlugin, "--target", "codex,claude", "--no-ui", "--yes"}
	result, err := commandsRunInstaller(t, f.Root, f.Project, env, installer, "site-add", args...)
	if err != nil || result["result"] != "success" {
		t.Fatalf("site Directory command failed: %v %+v", err, result)
	}
	bindings := directorySiteBindings(t, statePath, f.CodexHome, claudeHome)
	for client, binding := range bindings {
		nativeRequireContained(t, f.Root, binding.TargetLocator)
		if _, err := os.Stat(binding.TargetLocator); err != nil {
			t.Fatalf("%s managed directory absent: %v", client, err)
		}
	}
	codexBinding := bindings["codex"]
	claudeBinding := bindings["claude"]
	codexCheck := func(label string, present bool) {
		directorySiteCodexInventory(t, f, codexBin, codexBinding, present, label)
	}
	claudeCheck := func(label string, present bool) {
		entries := claudePluginList(t, claude, claudeBin, label)
		count := 0
		for _, entry := range entries {
			if entry.ID != directorySitePlugin+"@skills-dir" {
				continue
			}
			if !entry.Enabled || entry.InstallPath != claudeBinding.TargetLocator || entry.Scope != "user" {
				t.Fatalf("Claude Directory native entry mismatches actual installed location: %+v", entry)
			}
			count++
		}
		if (count == 1) != present || count > 1 {
			t.Fatalf("Claude plugin presence=%d, want=%t", count, present)
		}
		if present {
			details := claudePluginDetails(t, claude, claudeBin, directorySitePlugin+"@skills-dir", label+"-details")
			if !strings.Contains(details, directorySiteSkill) || !strings.Contains(details, "Skills (1)") ||
				!strings.Contains(details, "MCP servers (1)") {
				t.Fatalf("Claude Directory Skill/MCP not discovered: %s", details)
			}
		}
	}
	codexCheck("site-installed", true)
	claudeCheck("site-installed", true)
	verifyForeign("add")
	beforeState := nativeSHA(t, statePath)
	beforeCodex, beforeClaude := nativeTreeDigest(t, codexBinding.TargetLocator), nativeTreeDigest(t, claudeBinding.TargetLocator)
	result, err = commandsRunInstaller(t, f.Root, f.Project, env, installer, "site-replay", args...)
	if err != nil || result["result"] != "success" {
		t.Fatalf("site replay failed: %v %+v", err, result)
	}
	if nativeSHA(t, statePath) != beforeState || nativeTreeDigest(t, codexBinding.TargetLocator) != beforeCodex ||
		nativeTreeDigest(t, claudeBinding.TargetLocator) != beforeClaude {
		t.Fatal("site command replay changed saved/native content")
	}
	verifyForeign("replay")

	// The site exposes unfiltered update/remove of the saved Directory alias.
	result, err = commandsRunInstaller(t, f.Root, f.Project, env, installer, "site-update", "update", directorySitePlugin, "--no-ui", "--yes")
	if err != nil || result["result"] != "success" {
		t.Fatalf("site update failed: %v %+v", err, result)
	}
	codexCheck("site-updated", true)
	claudeCheck("site-updated", true)
	verifyForeign("update")
	result, err = commandsRunInstaller(t, f.Root, f.Project, env, installer, "site-remove", "remove", directorySitePlugin, "--no-ui", "--yes")
	if err != nil || result["result"] != "success" {
		t.Fatalf("site remove failed: %v %+v", err, result)
	}
	codexCheck("site-removed", false)
	claudeCheck("site-removed", false)
	for _, binding := range bindings {
		if _, err := os.Lstat(binding.TargetLocator); !os.IsNotExist(err) {
			t.Fatalf("removed Directory plugin still has managed artifact: %v", err)
		}
	}
	verifyForeign("remove")
}
