//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (c *copilotProjectCycle) expectedMCP(r *copilotCycleRecipe, native bool) map[string]any {
	root, data := r.Active, r.Data
	entry := map[string]any{"type": "stdio", "command": filepath.Join(root, "io.github.777genius.agentplugins", "managed-stdio-v1", "agentplugins"),
		"args": []any{"--internal-stdio-v1", root, data, root, "plugin", "--", "./bin/probe", "argument with spaces", root + "/argument", "${UNKNOWN}"},
		"env": map[string]any{"PLUGIN_ROOT": root, "PLUGIN_DATA": data, "UAP_TEST_ROOT": c.f.base.Root, "UAP_TEST_EVENTS": c.f.base.Events,
			"UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision, "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": root + "/${UNKNOWN}"}}
	if native {
		entry["tools"] = []any{"*"}
	}
	return entry
}

func (c *copilotProjectCycle) nativeSnapshot(t *testing.T, r *copilotCycleRecipe) copilotCycleNativeSnapshot {
	t.Helper()
	out := copilotCycleNativeSnapshot{Active: r.Active, Objects: r.Binding.NativeObjects, SkillPath: r.SkillPath, ConfigPath: r.Config}
	if r.Removed {
		if copilotCycleAbsent(r.Active) != nil || copilotCycleAbsent(r.SkillPath) != nil {
			t.Fatal("Copilot exact removed package/skill remains")
		}
		c.absent[r.Active] = true
	} else {
		body, err := projectNativeRuntimeRead(r.SkillPath, 1<<20)
		if err != nil || !bytes.Equal(body, r.Authored["skills/"+r.Skill+"/SKILL.md"]) {
			t.Fatal("Copilot installed skill differs from authored revision", err)
		}
		stream := copilotCycleStream(t, body)
		out.Skill = &stream
		c.f.protected[r.SkillPath] = copilotProjectConfigHash(body)
		out.ManagedFiles = c.managedProof(t, r)
		c.trees[r.Active] = out.ManagedFiles
	}
	if r.Key == "user" {
		return out
	}
	if !r.Removed {
		metadata, err := projectNativeRuntimeRead(filepath.Join(r.Active, "io.github.777genius.agentplugins", "managed-stdio-v1", "metadata.json"), 4096)
		if err != nil {
			t.Fatal(err)
		}
		stream := copilotCycleStream(t, metadata)
		out.LauncherMetadata = &stream
	}
	raw, err := projectNativeRuntimeRead(r.Config, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	value, err := projectNativeRuntimeJSON(raw)
	servers := map[string]any{}
	if !r.Removed {
		servers[r.Skill] = c.expectedMCP(r, true)
	}
	if err != nil || !reflect.DeepEqual(value, map[string]any{"mcpServers": servers}) {
		t.Fatal("Copilot native owned MCP membership/projection differs", err)
	}
	stream := copilotCycleStream(t, raw)
	out.Config = &stream
	c.f.protected[r.Config] = copilotProjectConfigHash(raw)
	out.SkillFiles = c.skillInventory(t, r)
	return out
}

func (c *copilotProjectCycle) managedProof(t *testing.T, r *copilotCycleRecipe) map[string]string {
	t.Helper()
	generated := map[string][]byte{}
	if r.Key == "user" {
		market := "agentplugins-" + copilotProjectConfigHash([]byte(r.Binding.PhysicalArtifact))[:12]
		value := map[string]any{"name": market, "owner": map[string]any{"name": "Agent Plugins CLI"},
			"metadata": map[string]any{"description": "Managed local marketplace for native-proof", "version": r.Version},
			"plugins":  []any{map[string]any{"name": "native-proof", "description": "Disposable Copilot cycle", "version": r.Version, "source": "."}}}
		raw, _ := json.MarshalIndent(value, "", "  ")
		generated[".github/plugin/marketplace.json"] = append(raw, '\n')
	} else {
		raw, _ := json.MarshalIndent(map[string]any{"mcpServers": map[string]any{r.Skill: c.expectedMCP(r, false)}}, "", "  ")
		generated["mcp.json"] = append(raw, '\n')
	}
	for relative, want := range generated {
		body, err := projectNativeRuntimeRead(filepath.Join(r.Active, filepath.FromSlash(relative)), 1<<20)
		if err != nil || !bytes.Equal(body, want) {
			t.Fatal("Copilot generated owned package bytes differ", err)
		}
	}
	tree, err := clineAttemptTree(r.Active)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.managedTree(r, tree, generated); err != nil {
		t.Fatal(err)
	}
	entries, err := clineAttemptPayload(r.Active)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := projectNativeRuntimeInstalledDigest(c.f.ctx, entries)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]domain.NativeObjectOwnership{"managed_package_directory": {
		ObjectID: "package:copilot:" + r.Binding.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: "native-proof", Path: r.Active, ManagedDigest: digest, ProtectionClass: "managed"}}
	if r.Key != "user" {
		skill := r.Authored["skills/"+r.Skill+"/SKILL.md"]
		framed := append(append([]byte("f\x00SKILL.md\x00-\x00"), skill...), 0)
		expected["copilot_project_skill_directory"] = domain.NativeObjectOwnership{ObjectID: "copilot_project_skill_directory:" + r.Skill, Kind: "copilot_project_skill_directory",
			LogicalName: r.Skill, Path: filepath.Dir(r.SkillPath), SourceRelative: "skills/" + r.Skill, ManagedDigest: "sha256:" + copilotProjectConfigHash(framed), ProtectionClass: "managed"}
		entry, _ := json.Marshal(c.expectedMCP(r, true))
		entry = append([]byte("agentplugins-native-mcp-v1\x00copilot-mcpServers\x00"+r.Skill+"\x00"), entry...)
		expected["copilot_project_mcp_server"] = domain.NativeObjectOwnership{ObjectID: "copilot_project_mcp_server:" + r.Skill, Kind: "copilot_project_mcp_server",
			LogicalName: r.Skill, Path: r.Config, ManagedDigest: "sha256:" + copilotProjectConfigHash(entry), ProtectionClass: "managed"}
	}
	if err := clineAttemptOwnership(r.Binding.NativeObjects, expected); err != nil {
		t.Fatal("Copilot independently owned object set differs", err)
	}
	return tree
}

func (c *copilotProjectCycle) managedTree(r *copilotCycleRecipe, actual map[string]string, generated map[string][]byte) error {
	want := map[string]string{".": fmt.Sprintf("directory:%o", os.ModeDir|0755)}
	files := map[string][]byte{}
	for name, body := range r.Authored {
		files[name] = body
	}
	for name, body := range generated {
		files[name] = body
	}
	for name, body := range files {
		mode := 0644
		if name == "bin/probe" {
			mode = 0755
		}
		path := filepath.FromSlash(name)
		want[path] = fmt.Sprintf("file:%o:%s", mode, copilotProjectConfigHash(body))
		for parent := filepath.Dir(path); parent != "."; parent = filepath.Dir(parent) {
			want[parent] = fmt.Sprintf("directory:%o", os.ModeDir|0755)
		}
	}
	if r.Key != "user" {
		parent := "io.github.777genius.agentplugins"
		root := filepath.Join(parent, "managed-stdio-v1")
		want[parent], want[root] = fmt.Sprintf("directory:%o", os.ModeDir|0700), fmt.Sprintf("directory:%o", os.ModeDir|0700)
		want[filepath.Join(root, "agentplugins")] = "file:700:" + c.f.immutable[c.f.installer]
		metadata, err := projectNativeRuntimeRead(filepath.Join(r.Active, root, "metadata.json"), 4096)
		if err != nil {
			return err
		}
		value, err := projectNativeRuntimeJSON(metadata)
		row, shape := value.(map[string]any)
		if err != nil || !shape || projectNativeRuntimeKeys(row, "protocol sha256 cliVersion", "") != nil || row["protocol"] != json.Number("1") || row["sha256"] != c.f.immutable[c.f.installer] || !projectNativeRuntimeText(row["cliVersion"], 128) {
			return fmt.Errorf("Copilot managed launcher metadata does not join the held executable")
		}
		want[filepath.Join(root, "metadata.json")] = "file:600:" + copilotProjectConfigHash(metadata)
	}
	if !reflect.DeepEqual(actual, want) {
		return fmt.Errorf("Copilot exact installed package membership/mode/digest differs")
	}
	return nil
}

func (c *copilotProjectCycle) skillInventory(t *testing.T, r *copilotCycleRecipe) map[string]string {
	t.Helper()
	root := filepath.Dir(filepath.Dir(r.SkillPath))
	tree, err := clineAttemptTree(root)
	if err != nil {
		t.Fatal(err)
	}
	rootMode := os.ModeDir | 0700
	want := map[string]string{".": fmt.Sprintf("directory:%o", rootMode)}
	if r.Key == "b" {
		name := "uap-foreign-b"
		body := []byte("---\nname: " + name + "\ndescription: Original project B skill must remain unchanged\nuser-invocable: true\n---\nUAP_FOREIGN_B_" + c.f.base.Nonce + "\n")
		want[name] = fmt.Sprintf("directory:%o", os.ModeDir|0700)
		want[filepath.Join(name, "SKILL.md")] = "file:600:" + copilotProjectConfigHash(body)
	}
	if !r.Removed {
		want[r.Skill] = fmt.Sprintf("directory:%o", os.ModeDir|0755)
		want[filepath.Join(r.Skill, "SKILL.md")] = "file:644:" + copilotProjectConfigHash(r.Authored["skills/"+r.Skill+"/SKILL.md"])
	}
	if !reflect.DeepEqual(tree, want) {
		t.Fatal("Copilot complete selected/foreign native skill inventory differs")
	}
	c.trees[root] = tree
	return tree
}

func copilotCycleDataPath(c *copilotProjectCycle, r *copilotCycleRecipe) string {
	return filepath.Join(filepath.Dir(c.statePath(r)), "plugin-data", "native-proof-"+copilotProjectConfigHash([]byte(r.Installation.InstallationID))[:12])
}
