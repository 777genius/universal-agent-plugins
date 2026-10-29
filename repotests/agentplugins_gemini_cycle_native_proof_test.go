//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (c *geminiProjectCycle) expectedEntry(r *geminiCycleRecipe) map[string]any {
	env := map[string]any{"PLUGIN_ROOT": r.Active, "PLUGIN_DATA": r.Data, "UAP_TEST_ROOT": c.f.base.Root, "UAP_TEST_EVENTS": c.f.base.Events, "UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision, "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": r.Active + "/${UNKNOWN}"}
	return map[string]any{"command": filepath.Join(r.Active, "bin", "probe"), "cwd": r.Active, "args": []any{"argument with spaces", r.Active + "/argument", "${UNKNOWN}"}, "env": env}
}
func (c *geminiProjectCycle) configProof(r *geminiCycleRecipe, installed bool) (map[string]any, error) {
	raw, err := projectNativeRuntimeRead(r.Config, 1<<20)
	if err != nil {
		return nil, err
	}
	value, err := projectNativeRuntimeJSON(raw)
	if err != nil {
		return nil, err
	}
	// Clone only the independently authored object; observed keys are never used
	// to construct the expected original settings or its owned member.
	baseline, _ := json.Marshal(c.settings[r.Scope])
	expected, err := projectNativeRuntimeJSON(baseline)
	if err != nil {
		return nil, err
	}
	doc := expected.(map[string]any)
	servers := doc["mcpServers"].(map[string]any)
	var entry map[string]any
	if installed {
		entry = c.expectedEntry(r)
		servers[r.Name] = entry
	}
	if !reflect.DeepEqual(value, doc) {
		return nil, fmt.Errorf("Gemini complete original settings/foreign/owned entry differs")
	}
	return entry, nil
}
func (c *geminiProjectCycle) skillProof(r *geminiCycleRecipe, installed bool) error {
	tree, err := geminiCycleTree(c.f.ctx, filepath.Join(r.Profile, "skills"))
	if err != nil {
		return err
	}
	foreign := "foreign-" + r.Scope
	files := map[string][]byte{foreign + "/SKILL.md": geminiCycleForeignSkill(r.Scope)}
	dirs := map[string]uint32{".": 0700, foreign: 0700}
	if installed {
		files[r.Name+"/SKILL.md"] = r.Authored["skills/"+r.Name+"/SKILL.md"]
		dirs[r.Name] = 0755
	}
	if len(tree) != len(files)+len(dirs) {
		return fmt.Errorf("Gemini complete native skill membership")
	}
	for name, record := range tree {
		if mode, ok := dirs[name]; ok {
			if record.Mode != 0040000|mode {
				return fmt.Errorf("Gemini native skill directory mode")
			}
			continue
		}
		body, ok := files[name]
		mode := uint32(0600)
		if strings.HasPrefix(name, r.Name+"/") {
			mode = 0644
		}
		if !ok || record.Mode != 0100000|mode || record.SHA256 != geminiCycleSHA(body) {
			return fmt.Errorf("Gemini native skill independent body/mode")
		}
	}
	return nil
}
func (c *geminiProjectCycle) payloadProof(r *geminiCycleRecipe) (string, error) {
	tree, err := geminiCycleTree(c.f.ctx, r.Active)
	if err != nil {
		return "", err
	}
	files := map[string][]byte{}
	for name, body := range r.Authored {
		files[name] = body
	}
	files[".agentplugins-gemini.json"], _ = json.Marshal(map[string]any{"data_root": r.Data})
	dirs := map[string]bool{".": true}
	for name := range files {
		for dir := filepath.ToSlash(filepath.Dir(name)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			dirs[dir] = true
		}
	}
	if len(tree) != len(files)+len(dirs) {
		return "", fmt.Errorf("Gemini complete installed payload membership")
	}
	entries := make([]packagedigest.CapturedEntry, 0, len(tree)-1)
	for name, record := range tree {
		if dirs[name] {
			if record.Mode&0170000 != 0040000 {
				return "", fmt.Errorf("Gemini payload directory")
			}
			if name != "." {
				entries = append(entries, packagedigest.CapturedEntry{Path: name, Kind: "directory"})
			}
			continue
		}
		want, ok := files[name]
		executable := name == "bin/probe"
		if !ok || record.Mode&0170000 != 0100000 || (record.Mode&0111 != 0) != executable {
			return "", fmt.Errorf("Gemini payload exact file/execute")
		}
		body, err := projectNativeRuntimeRead(filepath.Join(r.Active, filepath.FromSlash(name)), 64<<20)
		if err != nil || record.SHA256 != geminiCycleSHA(body) {
			return "", fmt.Errorf("Gemini payload held body")
		}
		if name == "mcp.json" || name == ".agentplugins-gemini.json" {
			a, e := projectNativeRuntimeJSON(body)
			b, f := projectNativeRuntimeJSON(want)
			if e != nil || f != nil || !reflect.DeepEqual(a, b) {
				return "", fmt.Errorf("Gemini payload exact independent JSON")
			}
		} else if !bytes.Equal(body, want) {
			return "", fmt.Errorf("Gemini payload exact authored bytes")
		}
		entries = append(entries, packagedigest.CapturedEntry{Path: name, Kind: "file", Executable: executable, Content: body})
	}
	after, err := geminiCycleTree(c.f.ctx, r.Active)
	if err != nil || !reflect.DeepEqual(tree, after) {
		return "", fmt.Errorf("Gemini payload changed during independent proof")
	}
	return geminiCycleInstalledDigest(c.f.ctx, entries)
}
func geminiCycleObjects(r *geminiCycleRecipe, b domain.ClientBinding, payload string, entry map[string]any) error {
	canonical, _ := json.Marshal(entry)
	mcpDigest := "sha256:" + geminiCycleSHA(append([]byte("agentplugins-native-mcp-v1\x00gemini-mcpServers\x00"+r.Name+"\x00"), canonical...))
	skill := r.Authored["skills/"+r.Name+"/SKILL.md"]
	skillDigest := "sha256:" + geminiCycleSHA(append(append([]byte("f\x00SKILL.md\x00-\x00"), skill...), 0))
	want := map[string]domain.NativeObjectOwnership{
		"managed_package_directory":     {ObjectID: "package:gemini:" + b.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: geminiCycleName, Path: r.Active, ManagedDigest: payload, ProtectionClass: "managed"},
		"gemini_global_mcp_server":      {ObjectID: "gemini-mcp:" + r.Name, Kind: "gemini_global_mcp_server", LogicalName: r.Name, Path: r.Config, ManagedDigest: mcpDigest, ProtectionClass: "managed"},
		"gemini_global_skill_directory": {ObjectID: "gemini-skill:" + r.Name, Kind: "gemini_global_skill_directory", LogicalName: r.Name, Path: filepath.Dir(r.SkillPath), SourceRelative: "skills/" + r.Name, ManagedDigest: skillDigest, ProtectionClass: "managed"},
	}
	if len(b.NativeObjects) != len(want) {
		return fmt.Errorf("Gemini exact native ownership count")
	}
	seen := map[string]bool{}
	for _, object := range b.NativeObjects {
		expected, ok := want[object.Kind]
		if !ok || seen[object.Kind] || !reflect.DeepEqual(object, expected) {
			return fmt.Errorf("Gemini exact native ownership identity")
		}
		seen[object.Kind] = true
	}
	return nil
}
func (c *geminiProjectCycle) dataProof(r *geminiCycleRecipe, i domain.Installation, id, physical string) (domain.DataReceipt, error) {
	path := filepath.Join(r.StateRoot, "plugin-data", physical)
	if id != "data_"+geminiCycleSHA([]byte(strings.Join([]string{i.InstallationID, physical, r.scopeName()}, "\x00")))[:24] || len(i.DataReceipts) != 1 {
		return domain.DataReceipt{}, fmt.Errorf("Gemini exact owned data identity/count")
	}
	d, ok := i.DataReceipts[id]
	if !ok || d.DataReceiptID != id || d.PhysicalBackend != physical || d.Scope != r.scopeName() || d.Locator != path || d.OwnershipDigest != "sha256:"+geminiCycleSHA([]byte("agentplugins-plugin-data-v1\x00"+id+"\x00"+path)) || d.State != domain.DataReceiptOwned {
		return d, fmt.Errorf("Gemini exact owned data receipt")
	}
	raw, err := projectNativeRuntimeRead(filepath.Join(path, ".agentplugins-data-owner.json"), 64<<10)
	if err != nil {
		return d, err
	}
	value, err := projectNativeRuntimeJSON(raw)
	expected, _ := json.Marshal(d)
	want, e := projectNativeRuntimeJSON(expected)
	if err != nil || e != nil || !reflect.DeepEqual(value, want) {
		return d, fmt.Errorf("Gemini exact owned marker/receipt")
	}
	if r.Installation.InstallationID != "" && !reflect.DeepEqual(i.DataReceipts, r.Installation.DataReceipts) {
		return d, fmt.Errorf("Gemini retained data receipt changed")
	}
	tree, err := geminiCycleTree(c.f.ctx, path)
	if err != nil {
		return d, err
	}
	if tree["."].Mode != 0040000|0700 || tree[".agentplugins-data-owner.json"].Mode != 0100000|0600 {
		return d, fmt.Errorf("Gemini exact owned data modes")
	}
	if r.Installation.InstallationID == "" && len(tree) != 2 {
		return d, fmt.Errorf("Gemini first data contains unproved files")
	}
	if r.Installation.InstallationID != "" {
		for name := range tree {
			if name != "." && name != ".agentplugins-data-owner.json" && name != "native-marker.txt" {
				return d, fmt.Errorf("Gemini unowned data child")
			}
		}
	}
	return d, nil
}
func geminiCycleAbsent(path string) error {
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("Gemini exact owned absence required")
	}
	return nil
}
