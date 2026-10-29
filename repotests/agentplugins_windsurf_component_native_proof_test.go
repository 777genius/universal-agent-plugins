//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

func (w *windComponentFixture) expectedEntry(r *windComponentRecipe) map[string]any {
	args := []any{"--internal-stdio-v1", r.Active, r.Data, r.Active, "plugin", "--", "./bin/probe", "Owned Windsurf " + r.Scope, r.Active + "/argument"}
	env := map[string]any{"PLUGIN_ROOT": r.Active, "PLUGIN_DATA": r.Data, "UAP_TEST_ROOT": w.f.base.Root, "UAP_TEST_EVENTS": w.f.base.Events, "UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision}
	return map[string]any{"command": filepath.Join(r.Active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName), "args": args, "env": env}
}
func (w *windComponentFixture) nativeProof(r *windComponentRecipe) (map[string]any, error) {
	raw, err := projectNativeRuntimeRead(r.Config, 1<<20)
	if err != nil {
		return nil, err
	}
	value, err := projectNativeRuntimeJSON(raw)
	doc, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(doc, "mcpServers", "") != nil {
		return nil, fmt.Errorf("component complete native object root")
	}
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok || len(servers) != 2 || !reflect.DeepEqual(servers["foreign-"+r.Scope], windForeign(r.Scope)) {
		return nil, fmt.Errorf("component complete foreign/server membership")
	}
	entry, ok := servers[windComponentName].(map[string]any)
	if !ok || !reflect.DeepEqual(entry, w.expectedEntry(r)) {
		return nil, fmt.Errorf("component exact native launcher/args/env")
	}
	profileTree, err := windOwnedTree(w.f.ctx, r.Profile)
	if err != nil || windNativeProfileClosure(w.trees[r.Profile], profileTree, r.Scope, r.Authored["skills/"+windComponentName+"/SKILL.md"]) != nil {
		return nil, fmt.Errorf("component public native closure")
	}
	tree, err := windOwnedTree(w.f.ctx, filepath.Join(r.Profile, "skills"))
	if err != nil {
		return nil, err
	}
	foreign := "foreign-" + r.Scope
	expected := map[string]bool{".": true, foreign: true, foreign + "/SKILL.md": true}
	if r.Scope != "user" {
		expected[windComponentName] = true
		expected[windComponentName+"/SKILL.md"] = true
	}
	if len(tree) != len(expected) {
		return nil, fmt.Errorf("component complete activated skill membership")
	}
	for key, record := range tree {
		if !expected[key] {
			return nil, fmt.Errorf("component unexpected native skill")
		}
		if filepath.Base(key) == "SKILL.md" && key == windComponentName+"/SKILL.md" {
			if record.Kind != "file" || record.SHA256 != windsurfConfigDigest(r.Authored["skills/"+windComponentName+"/SKILL.md"]) {
				return nil, fmt.Errorf("component native skill bytes")
			}
		}
	}
	return entry, nil
}
func windComponentObjects(r *windComponentRecipe, b domain.ClientBinding, payload string, entry map[string]any) error {
	kind, id, codec, protection, relative := "windsurf_mcp_entry", "windsurf:mcp:", "windsurf-mcpServers", "managed_entry", "mcp.json"
	if r.Scope != "user" {
		kind = "devin_project_mcp_server"
		id = kind + ":"
		codec = "devin-mcpServers"
		protection = "managed"
		relative = ""
	}
	canonical, _ := json.Marshal(entry)
	digest := "sha256:" + windsurfConfigDigest(append([]byte("agentplugins-native-mcp-v1\x00"+codec+"\x00"+windComponentName+"\x00"), canonical...))
	expected := map[string]domain.NativeObjectOwnership{
		"managed_package_directory": {ObjectID: "package:windsurf:" + b.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: windComponentName, Path: r.Active, ManagedDigest: payload, ProtectionClass: "managed"},
		kind:                        {ObjectID: id + windComponentName, Kind: kind, LogicalName: windComponentName, Path: r.Config, SourceRelative: relative, ManagedDigest: digest, ProtectionClass: protection},
	}
	if r.Scope != "user" {
		skill := r.Authored["skills/"+windComponentName+"/SKILL.md"]
		skillDigest := "sha256:" + windsurfConfigDigest(append(append([]byte("f\x00SKILL.md\x00-\x00"), skill...), 0))
		expected["devin_project_skill_directory"] = domain.NativeObjectOwnership{ObjectID: "devin_project_skill_directory:" + windComponentName, Kind: "devin_project_skill_directory", LogicalName: windComponentName, Path: filepath.Join(r.Profile, "skills", windComponentName), SourceRelative: "skills/" + windComponentName, ManagedDigest: skillDigest, ProtectionClass: "managed"}
	}
	if len(b.NativeObjects) != len(expected) {
		return fmt.Errorf("component native ownership count")
	}
	seen := map[string]bool{}
	for _, object := range b.NativeObjects {
		want, ok := expected[object.Kind]
		if !ok || seen[object.Kind] || !reflect.DeepEqual(object, want) {
			return fmt.Errorf("component exact native ownership")
		}
		seen[object.Kind] = true
	}
	return nil
}
func (w *windComponentFixture) payloadProof(r *windComponentRecipe, entry map[string]any) (string, error) {
	tree, err := windOwnedTree(w.f.ctx, r.Active)
	if err != nil {
		return "", err
	}
	expected := map[string][]byte{}
	executable := map[string]bool{}
	for name, body := range r.Authored {
		expected[name] = body
		executable[name] = name == "bin/probe"
	}
	server := map[string]any{"type": "stdio", "command": entry["command"], "args": entry["args"], "env": entry["env"]}
	expected["mcp.json"], _ = json.Marshal(map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{windComponentName: server}})
	launcher := filepath.ToSlash(filepath.Join(managedstdio.RelativeDirectory, managedstdio.ExecutableName))
	expected[launcher] = nil
	executable[launcher] = true
	expected[filepath.ToSlash(filepath.Join(managedstdio.RelativeDirectory, "metadata.json"))], _ = json.Marshal(map[string]any{"protocol": 1, "sha256": w.f.immutable[w.f.installer], "cliVersion": "0.1.0-development"})
	dirs := map[string]bool{".": true}
	for name := range expected {
		for dir := filepath.ToSlash(filepath.Dir(name)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			dirs[dir] = true
		}
	}
	if len(tree) != len(expected)+len(dirs) {
		return "", fmt.Errorf("component complete installed payload membership")
	}
	captured := []packagedigest.CapturedEntry{}
	for name, record := range tree {
		if dirs[name] {
			if record.Kind != "directory" {
				return "", fmt.Errorf("component installed directory type")
			}
			if name != "." {
				captured = append(captured, packagedigest.CapturedEntry{Path: name, Kind: "directory"})
			}
			continue
		}
		want, ok := expected[name]
		if !ok || record.Kind != "file" || (record.Mode&0111 != 0) != executable[name] {
			return "", fmt.Errorf("component installed file/type/execute")
		}
		body, err := projectNativeRuntimeRead(filepath.Join(r.Active, filepath.FromSlash(name)), 64<<20)
		if err != nil || windsurfConfigDigest(body) != record.SHA256 {
			return "", fmt.Errorf("component installed body changed")
		}
		switch {
		case name == launcher:
			if windsurfConfigDigest(body) != w.f.immutable[w.f.installer] {
				return "", fmt.Errorf("component launcher is not held installer")
			}
		case name == "mcp.json" || filepath.Base(name) == "metadata.json":
			actual, aerr := projectNativeRuntimeJSON(body)
			desired, werr := projectNativeRuntimeJSON(want)
			if aerr != nil || werr != nil || !reflect.DeepEqual(actual, desired) {
				return "", fmt.Errorf("component exact installed JSON")
			}
		default:
			if !bytes.Equal(body, want) {
				return "", fmt.Errorf("component independent installed bytes")
			}
		}
		captured = append(captured, packagedigest.CapturedEntry{Path: name, Kind: "file", Executable: executable[name], Content: body})
	}
	return projectNativeRuntimeInstalledDigest(w.f.ctx, captured)
}
