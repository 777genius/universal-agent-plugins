package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

const nativeAnthropicCommit = "ca45fc496a3436251f191354091d557da31c15b5"
const nativeAnthropicSource = "https://github.com/anthropics/skills/tree/" + nativeAnthropicCommit + "/skills"

type browserProjectFixture struct {
	codex        *codexProjectNativeFixture
	claude       *claudeProjectNativeFixture
	env          []string
	pins         map[string]string
	foreignTrust map[string]any
}

type browserProjectBinding struct {
	alias, declared, installation, payload, cache string
	binding                                       domain.ClientBinding
}

func nativeAnthropicPins(t *testing.T) map[string]string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(os.Getenv("AGENTPLUGINS_NATIVE_BROWSER_SOURCE"), "tests", "live-install", "fixtures.json"))
	var fixtures struct {
		Skills []struct {
			Name, Repo, Ref, Folder, Source string
			Blob                            string `json:"skill_md_git_blob_sha1"`
		}
	}
	if err != nil || json.Unmarshal(body, &fixtures) != nil {
		t.Fatal("read the existing immutable live Skill fixture pins", err)
	}
	pins := map[string]string{}
	for _, skill := range fixtures.Skills {
		if skill.Name != "docx" && skill.Name != "frontend-design" {
			continue
		}
		if skill.Repo != "anthropics/skills" || skill.Ref != nativeAnthropicCommit || skill.Folder != skill.Name ||
			skill.Source != nativeAnthropicSource+"/"+skill.Name || len(skill.Blob) != 40 || pins[skill.Name] != "" {
			t.Fatal("the selected example no longer has its exact immutable source pin")
		}
		pins[skill.Name] = skill.Blob
	}
	if len(pins) != 2 {
		t.Fatal("the example must contain exactly docx and frontend-design")
	}
	return pins
}

func (f *browserProjectFixture) state(t *testing.T, wanted map[string]bool) map[string]browserProjectBinding {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.codex.base.Project, ".uap", "state-v2.json"))
	var state domain.StateFileV2
	if err != nil || json.Unmarshal(body, &state) != nil || len(state.EnvironmentReceipts) != len(wanted) {
		t.Fatalf("unexpected selected project receipt set: %v, %s", err, body)
	}
	installations := map[string]domain.Installation{}
	count := 0
	for _, installation := range state.Installations {
		installations[installation.InstallationID] = installation
		count += len(installation.Clients)
	}
	if count != len(wanted) {
		t.Fatal("project has bindings outside the selected receipt set")
	}
	bindings := map[string]browserProjectBinding{}
	for _, receipt := range state.EnvironmentReceipts {
		key := receipt.Alias + "/" + receipt.ClientID
		installation := installations[receipt.InstallationID]
		binding, ok := installation.Clients[receipt.ClientBindingID]
		if !wanted[key] || bindings[key].installation != "" || !ok || binding.ClientBindingID != receipt.ClientBindingID ||
			binding.ClientID != receipt.ClientID || binding.Scope != "project" || binding.Activation != domain.ActivationActive ||
			binding.Authentication != domain.AuthenticationNotRequired || binding.Policy != domain.PolicyAllowed ||
			binding.Materialization != domain.MaterializationMaterialized || binding.Verification != domain.VerificationInstalled {
			t.Fatalf("selected binding is not uniquely owned and ready: %+v %+v", receipt, binding)
		}
		sourceKey := "github-skill:anthropics/skills:" + receipt.Alias + ":skills/" + receipt.Alias
		if f.pins[receipt.Alias] == "" || receipt.SourceKey != sourceKey || installation.Source.StableSourceKey != sourceKey ||
			installation.Source.ResolvedRevision != nativeAnthropicCommit || installation.Source.Repository != "anthropics/skills" ||
			installation.Source.TreeDigest != receipt.PackageDigest {
			t.Fatal("project receipt is not bound to the selected immutable Anthropic source")
		}
		item := browserProjectBinding{alias: receipt.Alias, declared: installation.DeclaredName,
			installation: installation.InstallationID, binding: binding}
		f.verifyPayload(t, &item, receipt.PackageDigest)
		bindings[key] = item
	}
	return bindings
}

func (f *browserProjectFixture) verifyPayload(t *testing.T, item *browserProjectBinding, digest string) {
	t.Helper()
	base := f.codex.base
	store := environmentstore.Store{Root: filepath.Join(base.Root, "installer-state", "store")}
	pkg, err := store.Get(t.Context(), domain.TreeDigestAlgorithm, digest)
	if err != nil {
		t.Fatal("verify the actual acquired immutable source", err)
	}
	source := pkg.Metadata.Source
	if source.Repository != "anthropics/skills" || source.ResolvedRevision != nativeAnthropicCommit ||
		source.Skill == nil || source.Skill.Name != item.alias || source.Skill.ResolvedPath != "skills/"+item.alias || source.Skill.RawTreeDigest == "" {
		t.Fatal("acquired package source is not the selected pinned raw Skill")
	}
	canonical := filepath.Join(pkg.Root, "skills", item.alias)
	want, err := shared.DigestSkillDirectory(canonical)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(canonical, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Git's existing object identifier is an additional immutable-byte check;
	// package ownership and full-tree verification continue to use SHA-256.
	blob := sha1.Sum(append([]byte(fmt.Sprintf("blob %d\x00", len(body))), body...))
	if hex.EncodeToString(blob[:]) != f.pins[item.alias] {
		t.Fatal("acquired SKILL.md does not match its independently pinned Git blob")
	}
	nativeRequireContained(t, base.Project, item.binding.TargetLocator)
	if item.binding.ClientID == "codex" {
		item.payload = filepath.Join(base.Project, ".agents", "skills", item.alias)
	} else {
		item.payload = filepath.Join(item.binding.TargetLocator, ".agentplugins-plugin", ".agentplugins-runtime", "skills", item.alias)
		marketplace := claude.ProjectMarketplaceName(item.binding.PhysicalArtifact, base.Project)
		item.cache = filepath.Join(f.claude.base.ConfigDir, "plugins", "cache", marketplace, item.declared, "0.0.0")
		cached := filepath.Join(item.cache, ".agentplugins-runtime", "skills", item.alias)
		if got, err := shared.DigestSkillDirectory(cached); err != nil || got != want {
			t.Fatalf("native Claude payload differs from the full acquired Skill: %v", err)
		}
	}
	if got, err := shared.DigestSkillDirectory(item.payload); err != nil || got != want {
		t.Fatalf("project payload differs from the full acquired Skill: %v", err)
	}
	f.verifyOwnedDigests(t, *item, want)
}

func (f *browserProjectFixture) verifyOwnedDigests(t *testing.T, item browserProjectBinding, payloadDigest string) {
	t.Helper()
	managed, native := 0, 0
	for _, object := range item.binding.NativeObjects {
		if object.UserModified || object.ManagedDigest == "" {
			t.Fatal("selected project receipt has no exact unchanged ownership")
		}
		switch object.Kind {
		case "managed_package_directory":
			managed++
			got, err := (packagesnapshot.Builder{}).Digest(t.Context(), item.binding.TargetLocator)
			if err != nil || object.Path != item.binding.TargetLocator || got != object.ManagedDigest {
				t.Fatal("managed project package digest differs from its ownership receipt", err)
			}
		case "codex_project_skill_directory":
			native++
			if object.Path != item.payload || object.LogicalName != item.alias || object.ManagedDigest != payloadDigest {
				t.Fatal("Codex native Skill path/digest differs from its receipt")
			}
		case "claude_project_plugin_cache":
			native++
			bundle := filepath.Join(item.binding.TargetLocator, ".agentplugins-plugin")
			if object.Path != bundle || filepath.Join(f.claude.base.ConfigDir, filepath.FromSlash(object.SourceRelative)) != item.cache {
				t.Fatal("Claude copied cache lacks exact owned source/path authority")
			}
			for _, path := range []string{bundle, item.cache} {
				got, err := (packagesnapshot.Builder{}).Digest(t.Context(), path)
				if err != nil || got != object.ManagedDigest {
					t.Fatal("Claude exact copied bundle differs from its ownership receipt", err)
				}
			}
		default:
			t.Fatalf("unexpected non-Skill native component: %s", object.Kind)
		}
	}
	if managed != 1 || native != 1 {
		t.Fatal("selected project binding lacks unique package and native ownership")
	}
}

func (f *browserProjectFixture) codexInventory(t *testing.T, label string, active map[string]bool) {
	t.Helper()
	for _, project := range []string{f.codex.base.Project, f.codex.otherProject} {
		selected := *f.codex.base
		selected.Project = project
		rpc := startNativeRPC(t, &selected, f.codex.client, label+"-"+filepath.Base(project))
		func() {
			defer rpc.close()
			rpc.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_example_check", "version": "1"}, "capabilities": map[string]bool{"experimentalApi": true}})
			if err := json.NewEncoder(rpc.in).Encode(map[string]any{"method": "initialized"}); err != nil {
				t.Fatal(err)
			}
			var result struct {
				Data []struct {
					CWD    string
					Errors []any
					Skills []struct {
						Name, Path, Scope, PluginID string
						Enabled                     bool
					}
				}
			}
			if err := json.Unmarshal(rpc.must(t, "skills/list", map[string]any{"cwds": []string{project}, "forceReload": true}), &result); err != nil ||
				len(result.Data) != 1 || result.Data[0].CWD != project || len(result.Data[0].Errors) != 0 {
				t.Fatal("native Codex inventory has an inexact project or discovery errors", err)
			}
			found := map[string]bool{}
			for _, skill := range result.Data[0].Skills {
				if f.pins[skill.Name] == "" {
					continue
				}
				if project != f.codex.base.Project || !active[skill.Name+"/codex"] || found[skill.Name] || !skill.Enabled ||
					skill.Scope != "repo" || skill.PluginID != "" || skill.Path != filepath.Join(project, ".agents", "skills", skill.Name, "SKILL.md") {
					t.Fatalf("unexpected native project Skill: %+v", skill)
				}
				found[skill.Name] = true
			}
			for name := range f.pins {
				if found[name] != (project == f.codex.base.Project && active[name+"/codex"]) {
					t.Fatalf("Codex native presence differs for %s in %s", name, project)
				}
			}
		}()
	}
}

func (f *browserProjectFixture) claudeInventory(t *testing.T, label string, original map[string]browserProjectBinding, present bool) {
	t.Helper()
	expected := []string{}
	for _, item := range original {
		if item.binding.ClientID == "claude" {
			expected = append(expected, item.declared+":"+item.alias)
		}
	}
	for _, project := range []string{f.claude.project, f.claude.otherProject} {
		body := claudeProjectCapture(t, f.claude, label+"-plugins-"+filepath.Base(project), project, 0, "plugin", "list", "--json")
		value, err := shared.DecodeUniqueJSONValue(json.NewDecoder(bytes.NewReader(body)))
		rows, ok := value.([]any)
		if err != nil || !ok || !json.Valid(body) {
			t.Fatal("native Claude listing is not one exact array", err)
		}
		for _, item := range original {
			if item.binding.ClientID != "claude" {
				continue
			}
			id := item.declared + "@" + claude.ProjectMarketplaceName(item.binding.PhysicalArtifact, f.claude.project)
			count := 0
			for _, raw := range rows {
				row, ok := raw.(map[string]any)
				if !ok {
					t.Fatal("malformed native Claude row")
				}
				if row["id"] != id {
					continue
				}
				count++
				if !present || row["scope"] != "project" || row["enabled"] != (project == f.claude.project) ||
					row["projectPath"] != f.claude.project || row["installPath"] != item.cache || row["version"] != "0.0.0" {
					t.Fatal("native Claude selected plugin has incorrect scope, enabled state, or owned path")
				}
			}
			if (count == 1) != present || count > 1 {
				t.Fatal("native Claude selected plugin is missing, duplicated, or still active after removal")
			}
		}
	}
	claudeProjectInitializedSkills(t, f.claude, label, expected, present)
}

func (f *browserProjectFixture) trust(t *testing.T, expected bool) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.claude.base.ConfigDir, ".claude.json"))
	var profile struct{ Projects map[string]map[string]any }
	if err != nil || json.Unmarshal(body, &profile) != nil {
		t.Fatal("read Claude native project trust", err)
	}
	trusted := profile.Projects[filepath.ToSlash(f.claude.project)]["hasTrustDialogAccepted"] == true
	if trusted != expected || !reflect.DeepEqual(profile.Projects[filepath.ToSlash(f.claude.otherProject)], f.foreignTrust) {
		t.Fatalf("unexpected selected or foreign native trust: %+v", profile.Projects)
	}
	f.codex.requireForeign(t)
	f.claude.requireForeign(t)
}

func browserProjectReadyResult(t *testing.T, result map[string]any, runErr error) {
	t.Helper()
	items, ok := result["items"].([]any)
	if runErr != nil || result["status"] != "succeeded" || result["readiness"] != "ready" || !ok || len(items) != 4 {
		t.Fatalf("same four-binding raw Skill batch is not ready: %v %+v", runErr, result)
	}
	seen := map[string]bool{}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		name, _ := item["alias"].(string)
		client, _ := item["targetId"].(string)
		key := name + "/" + client
		if !ok || seen[key] || (name != "docx" && name != "frontend-design") || (client != "codex" && client != "claude") || item["status"] != "unchanged" {
			t.Fatalf("unexpected repeated Skill item: %+v", item)
		}
		seen[key] = true
	}
}
