//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGrokCycleInspectRequiresExactNativeDotSkillSource(t *testing.T) {
	want := []grokCycleNativePlugin{{Name: "user", Scope: "user", Path: "/owned/home/plugin", Description: "Original marker", Skills: 1, Loaded: true}}
	for _, path := range []string{"/owned/home/plugin/./skills/user/SKILL.md", "/owned/home/plugin/skills/user/SKILL.md", "/owned/home/plugin/../plugin/skills/user/SKILL.md", "/owned/home/plugin/././skills/user/SKILL.md", "/foreign/./skills/user/SKILL.md"} {
		row := map[string]any{"grokVersion": "1.0.70", "cwd": "/owned/A", "projectRoot": "/owned/A/", "projectTrusted": true, "mcpServers": []any{},
			"plugins": []any{map[string]any{"name": "user", "scope": "user", "path": want[0].Path, "enabled": true, "provides": map[string]any{"skills": 1, "agents": 0, "hooks": false, "mcpServers": 0}}},
			"skills":  []any{map[string]any{"name": "user", "description": "Original marker", "userInvocable": true, "source": map[string]any{"type": "plugin", "plugin_name": "user", "path": path}}}}
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		if err := grokCycleInspect(raw, "/owned/A", true, want); (err == nil) != (path == "/owned/home/plugin/./skills/user/SKILL.md") {
			t.Fatal("native literal skill source comparison differs", path, err)
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
