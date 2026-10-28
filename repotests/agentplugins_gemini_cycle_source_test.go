//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (c *geminiProjectCycle) author(t *testing.T, r *geminiCycleRecipe, replace bool) {
	t.Helper()
	probe, err := projectNativeRuntimeRead(c.f.base.Probe, 32<<20)
	if err != nil {
		t.Fatal(err)
	}
	manifest, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": geminiCycleName, "version": "1.0.0", "description": "Disposable Gemini lifecycle proof"})
	skill := []byte(fmt.Sprintf("---\nname: %s\ndescription: %s\n---\nUAP_GEMINI_%s_%s\n", r.Name, r.Revision, r.Revision, r.Nonce))
	env := map[string]string{"UAP_TEST_ROOT": c.f.base.Root, "UAP_TEST_EVENTS": c.f.base.Events, "UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision, "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": "${PLUGIN_ROOT}/${UNKNOWN}"}
	mcp, _ := json.Marshal(map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{r.Name: map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"argument with spaces", "${PLUGIN_ROOT}/argument", "${UNKNOWN}"}, "cwd": "${PLUGIN_ROOT}", "env": env}}})
	authored := map[string][]byte{"plugin.json": manifest, "skills/" + r.Name + "/SKILL.md": skill, "mcp.json": mcp, "bin/probe": probe}
	entries := make([]packagedigest.CapturedEntry, 0, len(authored)*3)
	dirs := map[string]bool{}
	names := make([]string, 0, len(authored))
	for name := range authored {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		body := authored[name]
		path := filepath.Join(r.Source, filepath.FromSlash(name))
		mode := os.FileMode(0600)
		if name == "bin/probe" {
			mode = 0700
		}
		if replace {
			if _, err := projectNativeRuntimeRead(path, 64<<20); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, mode); err != nil {
				t.Fatal(err)
			}
		} else {
			projectNativeRuntimeWrite(t, path, body, mode)
		}
		entries = append(entries, packagedigest.CapturedEntry{Path: name, Kind: "file", Executable: mode&0111 != 0, Content: bytes.Clone(body)})
		for dir := filepath.ToSlash(filepath.Dir(name)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			dirs[dir] = true
		}
	}
	for dir := range dirs {
		entries = append(entries, packagedigest.CapturedEntry{Path: dir, Kind: "directory"})
	}
	digest, err := packagedigest.DigestCaptured(c.f.ctx, entries)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := geminiCycleTree(c.f.ctx, r.Source)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != len(authored)+len(dirs)+1 {
		t.Fatal("Gemini independent source inventory")
	}
	for name, record := range tree {
		if name == "." || dirs[name] {
			if record.Mode&0170000 != 0040000 {
				t.Fatal("Gemini source directory")
			}
			continue
		}
		body, ok := authored[name]
		if !ok || record.SHA256 != geminiCycleSHA(body) || record.Mode&0170000 != 0100000 || (record.Mode&0111 != 0) != (name == "bin/probe") {
			t.Fatal("Gemini source recipe byte/mode drift")
		}
	}
	r.Authored, r.Digest = authored, digest
	c.frozen[r.Source] = tree
	inventory := map[string]any{}
	for name, body := range authored {
		inventory[name] = map[string]any{"bytes": len(body), "sha256": geminiCycleSHA(body), "executable": name == "bin/probe"}
	}
	c.append(t, "cycle-proof.jsonl", "source-"+r.Scope+"-"+r.Revision, map[string]any{"scope": r.Scope, "source": r.Source, "name": r.Name, "revision": r.Revision, "nonce": r.Nonce, "digest": digest, "files": inventory, "skill": c.stream(t, skill), "manifest": c.stream(t, manifest), "mcp": c.stream(t, mcp)})
}
func geminiCycleSource(i domain.Installation, r *geminiCycleRecipe) error {
	wantSource := domain.SourceBinding{SourceBindingID: domain.ComputeSourceBindingID(domain.SourceIdentity{RequestedSource: r.Source, CanonicalSource: r.Source}), RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: r.Digest}
	wantPackage := domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: geminiCycleName, Version: "1.0.0", ManifestDigest: "sha256:" + geminiCycleSHA(r.Authored["plugin.json"]), Inventory: domain.ComponentInventory{Skills: []string{r.Name}, MCPPresent: true, MCPEnabled: true, MCPServers: []string{r.Name}}}
	if i.InstallationID == "" || i.DeclaredName != geminiCycleName || i.OriginMode != domain.OriginModeDirect || i.NeedsRebind || i.Directory != nil || i.LocalChatGPTMapping != nil || len(i.InstallPreferences) != 0 || !reflect.DeepEqual(i.Source, wantSource) || !reflect.DeepEqual(i.Package, wantPackage) {
		return fmt.Errorf("Gemini cycle exact independent source/package")
	}
	return nil
}
func (c *geminiProjectCycle) reviseA(t *testing.T) {
	t.Helper()
	c.stage = "source-revision-a"
	r := c.recipes["a"]
	c.check(t)
	c.proof(t, r, "before-source-revision-a")
	delete(c.frozen, r.Source)
	r.Revision = "A-two"
	c.author(t, r, true)
	c.check(t)
}
