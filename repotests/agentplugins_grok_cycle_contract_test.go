//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGrokCycleInspectRequiresExactProducerSkillSource(t *testing.T) {
	for _, want := range []grokCycleNativePlugin{
		{Name: "user", Scope: "user", Path: "/owned/home/plugin", SkillPath: "/owned/home/plugin/./skills/user/SKILL.md", Description: "Original marker", Skills: 1, Loaded: true},
		{Name: "project", Scope: "project", Path: "/owned/A/plugin", SkillPath: "/owned/A/plugin/skills/project/SKILL.md", Description: "Owned marker", Skills: 1, Loaded: true},
	} {
		for _, path := range []string{want.Path + "/./skills/" + want.Name + "/SKILL.md", want.Path + "/skills/" + want.Name + "/SKILL.md", want.Path + "/../plugin/skills/" + want.Name + "/SKILL.md", want.Path + "/././skills/" + want.Name + "/SKILL.md", "/foreign/skills/" + want.Name + "/SKILL.md"} {
			row := map[string]any{"grokVersion": "1.0.70", "cwd": "/owned/A", "projectRoot": "/owned/A/", "projectTrusted": true, "mcpServers": []any{},
				"plugins": []any{map[string]any{"name": want.Name, "scope": want.Scope, "path": want.Path, "enabled": true, "provides": map[string]any{"skills": 1, "agents": 0, "hooks": false, "mcpServers": 0}}},
				"skills":  []any{map[string]any{"name": want.Name, "description": want.Description, "userInvocable": true, "source": map[string]any{"type": "plugin", "plugin_name": want.Name, "path": path}}}}
			raw, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			if err := grokCycleInspect(raw, "/owned/A", true, []grokCycleNativePlugin{want}); (err == nil) != (path == want.SkillPath) {
				t.Fatal("native producer skill source comparison differs", want.Name, path, err)
			}
		}
	}
}

func grokCycleRepeatAddContract(t *testing.T) (map[string]any, *grokCycleRecipe) {
	t.Helper()
	const body = `{"schema_version":1,"command":"add","result":"success","data":{"plugin":"held-grok","version":"1.0.0","source":"direct local source","tree_digest":"sha256:held-tree","manifest_digest":"sha256:held-manifest","dry_run":false,"result":{"installation_id":"held-installation","plan":{"client_id":"grok","scope":"project","status":"ready","package_mode":"native","activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"package_validated","physical_artifact_id":"held-physical","components":[{"kind":"skill","name":"held-grok","support":"native"}]},"activation":{"activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"package_validated"},"requires_confirmation":false,"mutated":false}}}`
	value, err := projectNativeRuntimeJSON([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	r := &grokCycleRecipe{Scope: "a", Source: "/held/source", Name: "held-grok", Digest: "sha256:held-tree"}
	r.Installation.InstallationID = "held-installation"
	r.Installation.Source.CanonicalSource, r.Installation.Source.TreeDigest = r.Source, r.Digest
	r.Installation.Package.ManifestDigest = "sha256:held-manifest"
	r.Binding.PhysicalArtifact = "held-physical"
	return value.(map[string]any), r
}

func TestGrokCycleRepeatAddRequiresHeldDirectPublicResult(t *testing.T) {
	value, r := grokCycleRepeatAddContract(t)
	if err := grokCycleRepeatAdd("repeat-add-a", "a", false, value, r); err != nil {
		t.Fatal("held direct repeated add refused", err)
	}
	if _, err := commandsTargetOutcome(value); err == nil {
		t.Fatal("shared batch helper unexpectedly accepts direct result")
	}
	for _, stage := range []string{"add-a", "add-b", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a"} {
		if grokCycleRepeatAdd(stage, "a", false, value, r) == nil {
			t.Fatal("unrelated stage accepted singleton", stage)
		}
	}
	if grokCycleRepeatAdd("repeat-add-a", "b", false, value, r) == nil || grokCycleRepeatAdd("repeat-add-a", "a", true, value, r) == nil || grokCycleRepeatAdd("repeat-add-a", "a", false, value, nil) == nil {
		t.Fatal("wrong scope/mutation/missing held authority accepted")
	}
}

func TestGrokCycleRepeatAddRefusesChangedPublicAuthority(t *testing.T) {
	for _, path := range [][]string{{"schema_version"}, {"command"}, {"result"}, {"data"}, {"data", "plugin"}, {"data", "version"}, {"data", "source"}, {"data", "tree_digest"}, {"data", "manifest_digest"}, {"data", "dry_run"}, {"data", "result"}, {"data", "result", "installation_id"}, {"data", "result", "requires_confirmation"}, {"data", "result", "mutated"}, {"data", "result", "plan"}, {"data", "result", "activation"}, {"data", "result", "plan", "client_id"}, {"data", "result", "plan", "scope"}, {"data", "result", "plan", "status"}, {"data", "result", "plan", "package_mode"}, {"data", "result", "plan", "physical_artifact_id"}, {"data", "result", "plan", "components"}, {"data", "result", "plan", "activation"}, {"data", "result", "plan", "authentication"}, {"data", "result", "plan", "policy"}, {"data", "result", "plan", "verification"}, {"data", "result", "activation", "activation"}, {"data", "result", "activation", "authentication"}, {"data", "result", "activation", "policy"}, {"data", "result", "activation", "verification"}} {
		for _, replacement := range []any{nil, "foreign", true, json.Number("1")} {
			value, r := grokCycleRepeatAddContract(t)
			parent := value
			for _, key := range path[:len(path)-1] {
				parent = parent[key].(map[string]any)
			}
			if reflect.DeepEqual(parent[path[len(path)-1]], replacement) {
				continue
			}
			if replacement == nil {
				delete(parent, path[len(path)-1])
			} else {
				parent[path[len(path)-1]] = replacement
			}
			if grokCycleRepeatAdd("repeat-add-a", "a", false, value, r) == nil {
				t.Fatal("changed singleton authority accepted", path, replacement)
			}
		}
	}
	for _, field := range []string{"batch", "targets", "operation_id"} {
		value, r := grokCycleRepeatAddContract(t)
		value["data"].(map[string]any)[field] = "invented"
		if grokCycleRepeatAdd("repeat-add-a", "a", false, value, r) == nil {
			t.Fatal("singleton adopted grouped output", field)
		}
	}
}

func TestGrokCycleRawPrivacyDoesNotRequireProvider(t *testing.T) {
	for _, raw := range [][]byte{[]byte("uap-grok-private-inert"), []byte(`{"value":"\u0075ap-grok-private-inert"}`), []byte("{}\n{\"value\":\"\\u0075ap-grok-private-inert\"}\n")} {
		if _, err := grokCycleStream(raw); err == nil {
			t.Fatal("provider-free cycle encoded private raw input")
		}
	}
	value, err := grokCycleStream([]byte("ordinary fixture\n"))
	if err != nil || string(value.Body) != "ordinary fixture\n" || value.Bytes != len(value.Body) || value.SHA256 != clineAttemptSHA(value.Body) {
		t.Fatal("provider-free raw byte receipt differs", err)
	}
}

func TestGrokCycleSnapshotHoldsOriginalIdentityAndRefusesLinks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "original")
	projectNativeRuntimeWrite(t, path, []byte("original bytes\n"), 0600)
	before, err := grokCycleSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	after, err := grokCycleSnapshot(root)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("ordinary held original changed", err)
	}
	replacement := filepath.Join(t.TempDir(), "replacement")
	projectNativeRuntimeWrite(t, replacement, []byte("original bytes\n"), 0600)
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	after, err = grokCycleSnapshot(root)
	if err != nil || reflect.DeepEqual(before, after) {
		t.Fatal("same bytes adopted a replacement original inode", err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Link(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := grokCycleSnapshot(root); err == nil {
		t.Fatal("hard-linked original was adopted")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := grokCycleSnapshot(root); err == nil {
		t.Fatal("symlinked original was adopted")
	}
}
