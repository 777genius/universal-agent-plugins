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
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

type cursorProjectIdentity struct {
	Client      string `json:"client"`
	Acquisition struct {
		SHA     string `json:"sha256"`
		Pin     bool   `json:"pin_verified"`
		Members []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
			SHA  string `json:"sha256"`
		} `json:"members"`
	} `json:"acquisition"`
	RuntimeFiles []struct {
		Path string `json:"path"`
		SHA  string `json:"sha256"`
	} `json:"runtime_files"`
}

func (f *cursorProjectAttempt) freezeAcquisition(t *testing.T) {
	t.Helper()
	body, err := projectNativeRuntimeRead(os.Getenv("AGENTPLUGINS_PROJECT_NATIVE_IDENTITY"), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectNativeRuntimeJSON(body); err != nil {
		t.Fatal(err)
	}
	var identity cursorProjectIdentity
	if json.Unmarshal(body, &identity) != nil || identity.Client != "cursor" || !identity.Acquisition.Pin || identity.Acquisition.SHA != cursorProjectArchive || len(identity.Acquisition.Members) != 565 || len(identity.RuntimeFiles) != 445 {
		t.Fatal("Cursor exact acquisition identity differs")
	}
	f.runtimeRoot = filepath.Dir(filepath.Dir(f.client))
	archiveMembers := map[string]string{}
	expected := map[string]string{}
	for _, member := range identity.Acquisition.Members {
		name := member.Name
		if name == "" || name == "." || name == ".." || filepath.IsAbs(name) || filepath.Clean(name) != name || strings.Contains(name, "\\") || strings.HasPrefix(name, "../") || (member.Kind != "file" && member.Kind != "directory") {
			t.Fatal("Cursor acquisition member differs")
		}
		path := filepath.Join(f.runtimeRoot, name)
		if archiveMembers[path] != "" {
			t.Fatal("Cursor duplicate acquisition member")
		}
		archiveMembers[path] = member.Kind
		if member.Kind == "file" {
			expected[path] = member.SHA
		}
	}
	f.runtimeMembers, err = cursorProjectMaterializedMembers(f.runtimeRoot, archiveMembers)
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) != 445 || expected[f.client] != "2ccc9a8e167797641448b5e5c936f006ba137a2555f117f38c5eb76a5238a233" || expected[filepath.Join(filepath.Dir(f.client), "node")] != "e32e2d16081cf644f88392934288086e7662529d0d43277c34f7aad2207cfafb" || expected[filepath.Join(filepath.Dir(f.client), "index.js")] != "7b4bb3badb36cff7b6cee7996d61bd3c32866cf354214374b77f5a5d33a0b3f2" {
		t.Fatal("Cursor launcher/bundled Node/index closure differs")
	}
	for _, file := range identity.RuntimeFiles {
		if expected[file.Path] == "" || expected[file.Path] != file.SHA || f.held[file.Path].info != nil {
			t.Fatal("Cursor runtime closure differs")
		}
		if err := f.hold(file.Path); err != nil || f.held[file.Path].digest != file.SHA {
			t.Fatal("Cursor runtime member digest differs")
		}
	}
	if err := f.runtimeClosure(); err != nil {
		t.Fatal(err)
	}
	f.evidence["acquisition"] = json.RawMessage(body)
}

// The existing materializer writes regular files and creates only their parent
// chains. Empty archive directories are inventoried, but are not materialized.
func cursorProjectMaterializedMembers(root string, archive map[string]string) (map[string]string, error) {
	result := map[string]string{}
	for path, kind := range archive {
		if !strings.HasPrefix(path, root+string(filepath.Separator)) {
			return nil, fmt.Errorf("Cursor archive member is outside materialized root")
		}
		if kind != "file" {
			continue
		}
		result[path] = "file"
		for parent := filepath.Dir(path); parent != root; parent = filepath.Dir(parent) {
			if archive[parent] != "directory" || parent == filepath.Dir(parent) {
				return nil, fmt.Errorf("Cursor materialized parent is absent from trusted archive directories")
			}
			result[parent] = "directory"
		}
	}
	return result, nil
}
func (f *cursorProjectAttempt) runtimeClosure() error {
	seen := 0
	err := filepath.WalkDir(f.runtimeRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == f.runtimeRoot {
			return nil
		}
		kind := f.runtimeMembers[path]
		if (kind == "directory" && !entry.IsDir()) || (kind == "file" && !entry.Type().IsRegular()) || kind == "" {
			return fmt.Errorf("Cursor unknown/aliased runtime member")
		}
		seen++
		return nil
	})
	if err != nil || seen != len(f.runtimeMembers) {
		return fmt.Errorf("Cursor complete runtime member set changed")
	}
	return nil
}
func (f *cursorProjectAttempt) packageSource(t *testing.T) {
	t.Helper()
	manifest := []byte("{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json\",\"name\":\"native-proof\",\"version\":\"1.0.0\"}\n")
	server := map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"cursor-component"}, "cwd": "${PLUGIN_ROOT}", "env": map[string]string{"UAP_TEST_ROOT": f.base.Root, "UAP_TEST_EVENTS": f.base.Events, "UAP_TEST_NONCE": f.base.Nonce, "UAP_TEST_REVISION": "cursor-a-one"}}
	config, err := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{"uapcursora": server}})
	if err != nil {
		t.Fatal(err)
	}
	probe, err := projectNativeRuntimeRead(f.base.Probe, 32<<20)
	if err != nil {
		t.Fatal(err)
	}
	authored := map[string][]byte{"plugin.json": manifest, "mcp.json": append(config, '\n'), "bin/probe": probe}
	for relative, body := range authored {
		path := filepath.Join(f.base.Package, relative)
		mode := os.FileMode(0600)
		if relative == "bin/probe" {
			mode = 0700
		}
		projectNativeRuntimeWrite(t, path, body, mode)
		if err := f.hold(path); err != nil {
			t.Fatal(err)
		}
	}
	f.evidence["fixture"] = map[string]any{"root": f.base.Root, "home": f.base.Home, "a": f.base.Project, "b": f.other, "outside": f.outside, "source": f.base.Package, "nonce": f.base.Nonce, "mcp_name": "uapcursora", "manifest": string(manifest), "mcp": string(config), "probe_sha256": f.held[f.base.Probe].digest}
}
func (f *cursorProjectAttempt) installed(t *testing.T, stdout []byte) {
	t.Helper()
	if _, err := projectNativeRuntimeJSON(stdout); err != nil {
		t.Fatal(err)
	}
	if err := nativeValidateInstallerJSON(stdout); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.base.Project, ".uap", "state-v2.json")
	body, err := projectNativeRuntimeRead(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if json.Unmarshal(body, &state) != nil || len(state.Installations) != 1 {
		t.Fatal("Cursor exact project installation missing")
	}
	installation := state.Installations[0]
	if installation.DeclaredName != "native-proof" || installation.Source.CanonicalSource != f.base.Package || installation.Source.SourceBindingID == "" || installation.Source.TreeDigest == "" || installation.Package.DeclaredName != "native-proof" || installation.Package.Version != "1.0.0" || installation.Package.ManifestDigest == "" || len(installation.Clients) != 1 {
		t.Fatal("Cursor selected source/package authority differs")
	}
	for _, binding := range installation.Clients {
		f.installedBinding(t, binding, installation.DataReceipts[binding.DataReceiptID])
	}
	if err := f.hold(path); err != nil {
		t.Fatal(err)
	}
	f.evidence["installed_state"] = json.RawMessage(body)
}
func (f *cursorProjectAttempt) installedBinding(t *testing.T, binding domain.ClientBinding, data domain.DataReceipt) {
	t.Helper()
	if binding.ClientID != "cursor" || binding.Scope != "project" || binding.NativeProfileRoot != filepath.Join(f.base.Project, ".cursor") || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || data.State != domain.DataReceiptOwned || data.Scope != "project" || data.DataReceiptID != binding.DataReceiptID || data.PhysicalBackend != binding.PhysicalArtifact || data.OwnershipDigest == "" {
		t.Fatal("Cursor Prepared binding/data receipt differs")
	}
	nativeRequireContained(t, filepath.Join(f.base.Project, ".uap", "managed"), binding.TargetLocator)
	nativeRequireContained(t, filepath.Join(f.base.Project, ".uap", "plugin-data"), data.Locator)
	configPath := filepath.Join(f.base.Project, ".cursor", "mcp.json")
	owned := 0
	for _, object := range binding.NativeObjects {
		if object.Kind == "cursor_project_mcp_server" {
			if object.Path != configPath || object.LogicalName != "uapcursora" || object.ManagedDigest == "" {
				t.Fatal("Cursor MCP ownership differs")
			}
			owned++
		}
	}
	if owned != 1 {
		t.Fatal("Cursor single owned MCP receipt missing")
	}
	config, err := projectNativeRuntimeRead(configPath, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	f.configBinding(t, config, binding.TargetLocator, data.Locator)
	for _, relative := range []string{"plugin.json", "bin/probe"} {
		target := filepath.Join(binding.TargetLocator, relative)
		body, err := projectNativeRuntimeRead(target, 32<<20)
		source, sourceErr := projectNativeRuntimeRead(filepath.Join(f.base.Package, relative), 32<<20)
		if err != nil || sourceErr != nil || !bytes.Equal(body, source) {
			t.Fatal("Cursor installed source bytes differ")
		}
		if err := f.hold(target); err != nil {
			t.Fatal(err)
		}
	}
	projected := filepath.Join(binding.TargetLocator, "mcp.json")
	projectedBody, err := projectNativeRuntimeRead(projected, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	f.configBinding(t, projectedBody, binding.TargetLocator, data.Locator)
	if err := f.hold(projected); err != nil {
		t.Fatal(err)
	}
	if err := f.hold(filepath.Join(data.Locator, ".agentplugins-data-owner.json")); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(binding.TargetLocator, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	if err := f.hold(launcher); err != nil {
		t.Fatal(err)
	}
	if f.held[launcher].digest != f.held[f.installer].digest {
		t.Fatal("Cursor managed launcher differs from held current installer")
	}
	if err := f.hold(configPath); err != nil {
		t.Fatal(err)
	}
	f.evidence["installed_config"] = string(config)
	f.evidence["binding"] = binding
	f.evidence["data_receipt"] = data
}
func (f *cursorProjectAttempt) configBinding(t *testing.T, body []byte, active, data string) {
	t.Helper()
	value, err := projectNativeRuntimeJSON(body)
	object, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(object, "mcpServers", "") != nil {
		t.Fatal("Cursor native config shape differs")
	}
	servers, ok := object["mcpServers"].(map[string]any)
	if !ok || len(servers) != 1 {
		t.Fatal("Cursor exact native MCP inventory differs")
	}
	server, ok := servers["uapcursora"].(map[string]any)
	if !ok || projectNativeRuntimeKeys(server, "command args env", "") != nil {
		t.Fatal("Cursor native MCP fields differ")
	}
	env := map[string]any{"UAP_TEST_ROOT": f.base.Root, "UAP_TEST_EVENTS": f.base.Events, "UAP_TEST_NONCE": f.base.Nonce, "UAP_TEST_REVISION": "cursor-a-one", "PLUGIN_ROOT": active, "PLUGIN_DATA": data}
	args := []any{managedstdio.Mode, active, data, active, "plugin", "--argv-base64-v1", "Li9iaW4vcHJvYmU=", "Y3Vyc29yLWNvbXBvbmVudA=="}
	if server["command"] != filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName) || !reflect.DeepEqual(server["args"], args) || !reflect.DeepEqual(server["env"], env) {
		t.Fatal("Cursor installed managed stdio recipe differs")
	}
}
