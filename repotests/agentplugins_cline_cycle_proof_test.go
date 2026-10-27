//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func clineCycleState(raw []byte) (domain.StateFileV2, error) {
	var result domain.StateFileV2
	if _, err := projectNativeRuntimeJSON(raw); err != nil {
		return result, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&result); err != nil {
		return result, err
	}
	if result.SchemaVersion != domain.StateSchemaVersion {
		return result, fmt.Errorf("current Cline cycle state schema required")
	}
	if _, err := d.Token(); err != io.EOF {
		return result, fmt.Errorf("Cline cycle state trailing value")
	}
	return result, nil
}
func (c *clineProjectCycle) proof(t *testing.T, r *clineCycleRecipe, stage string) {
	t.Helper()
	raw, err := projectNativeRuntimeRead(c.statePath(r), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	state, err := clineCycleState(raw)
	if err != nil || len(state.Installations) != 1 {
		t.Fatal("exact single current Cline cycle installation", err)
	}
	i := state.Installations[0]
	if err := clineCycleSource(i, r); err != nil {
		t.Fatal(err)
	}
	if r.Installation.InstallationID != "" && i.InstallationID != r.Installation.InstallationID {
		t.Fatal("Cline cycle installation identity changed")
	}
	if r.Removed {
		c.removedProof(t, r, i)
	} else {
		c.bindingProof(t, r, i)
	}
	c.foreign(t, r)
	c.skillInventory(t, r)
	r.Installation = i
	c.append(t, "cycle-proof.jsonl", stage, map[string]any{"scope": r.Scope, "revision": r.Revision, "removed": r.Removed, "state": c.stream(t, raw), "binding": r.Binding, "source_digest": r.Digest})
	c.f.protected[c.statePath(r)] = clineAttemptSHA(raw)
	c.f.protected[r.Config] = nativeSHA(t, r.Config)
	if !r.Removed {
		c.f.protected[r.SkillPath] = nativeSHA(t, r.SkillPath)
		c.f.trees[r.Active], err = clineAttemptTree(r.Active)
		if err != nil {
			t.Fatal(err)
		}
	}
	c.f.check(t)
}
func clineCycleSource(i domain.Installation, r *clineCycleRecipe) error {
	wantSource := domain.SourceBinding{SourceBindingID: "src_" + clineAttemptSHA([]byte(r.Source + "\x00\x00"))[:24], RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: r.Digest}
	inventory := domain.ComponentInventory{Skills: []string{r.Skill}}
	if r.Scope != "user" {
		inventory.MCPPresent = true
		inventory.MCPEnabled = true
		inventory.MCPServers = []string{"uapclinea"}
	}
	want := domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: clineCycleName, Version: "1.0.0", ManifestDigest: "sha256:" + clineAttemptSHA(r.Authored["plugin.json"]), Inventory: inventory}
	if i.InstallationID == "" || i.DeclaredName != clineCycleName || !reflect.DeepEqual(i.Source, wantSource) || !reflect.DeepEqual(i.Package, want) {
		return fmt.Errorf("Cline cycle source/package differs from prior independent recipe")
	}
	return nil
}
func (c *clineProjectCycle) bindingProof(t *testing.T, r *clineCycleRecipe, i domain.Installation) {
	t.Helper()
	if len(i.Clients) != 1 || i.DataRetained {
		t.Fatal("Cline cycle single materialized binding required")
	}
	var b domain.ClientBinding
	for _, candidate := range i.Clients {
		b = candidate
	}
	stateRoot := filepath.Dir(c.statePath(r))
	physical := clineCycleName + "-" + clineAttemptSHA([]byte(i.InstallationID))[:12]
	active := filepath.Join(stateRoot, "managed", "clients", "cline", physical)
	profile, activation := r.Project, domain.ActivationPrepared
	if r.Scope == "user" {
		// The pinned Cline user trait does not bind a NativeProfileRoot token;
		// exact user-native object paths still bind original HOME below.
		profile = ""
		activation = domain.ActivationActive
	}
	wantRevision := &domain.ClientPackageRevision{Version: "1.0.0", TreeDigest: r.Digest, ManifestDigest: "sha256:" + clineAttemptSHA(r.Authored["plugin.json"])}
	if b.ClientID != "cline" || b.Scope != scopeName(r) || b.TargetLocator != active || b.NativeProfileRoot != profile || b.PhysicalArtifact != physical || b.ClientBindingID != "client_"+clineAttemptSHA([]byte(strings.Join([]string{i.InstallationID, "cline", scopeName(r), active}, "\x00")))[:24] || b.Materialization != domain.MaterializationMaterialized || b.Activation != activation || b.Verification != domain.VerificationInstalled || !reflect.DeepEqual(b.PackageRevision, wantRevision) {
		t.Fatal("exact Cline cycle binding identity/activation/revision differs")
	}
	if r.Binding.ClientBindingID != "" && (b.ClientBindingID != r.Binding.ClientBindingID || b.PhysicalArtifact != r.Binding.PhysicalArtifact || b.DataReceiptID != r.Binding.DataReceiptID) {
		t.Fatal("Cline cycle update/repair replaced stable binding/data identity")
	}
	r.Active = active
	for scope, other := range c.recipes {
		if scope != r.Scope && other.Installation.InstallationID != "" && (other.Installation.InstallationID == i.InstallationID || other.Binding.PhysicalArtifact == physical || other.Binding.ClientBindingID == b.ClientBindingID) {
			t.Fatal("Cline same-name scopes shared installation/binding/physical authority")
		}
	}
	if r.Scope == "user" {
		path, err := c.userUnallocatedData(r, i, b)
		if err != nil {
			t.Fatal(err)
		}
		c.f.absent[path] = true
	} else {
		c.dataProof(t, r, i, b.DataReceiptID, physical)
	}
	entry := c.configProof(t, r, !r.Removed)
	if err := clineCycleInstalled(c.f, r, entry); err != nil {
		t.Fatal(err)
	}
	body, err := projectNativeRuntimeRead(r.SkillPath, 1<<20)
	if err != nil || !bytes.Equal(body, r.Authored["skills/"+r.Skill+"/SKILL.md"]) {
		t.Fatal("Cline cycle delivered skill differs")
	}
	entries, err := clineAttemptPayload(active)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := projectNativeRuntimeInstalledDigest(c.f.ctx, entries)
	if err != nil {
		t.Fatal(err)
	}
	skillDigest := "sha256:" + clineAttemptSHA(append(append([]byte("f\x00SKILL.md\x00-\x00"), body...), 0))
	skillKind, skillID := "cline_project_skill_directory", "cline_project_skill_directory:"+r.Skill
	if r.Scope == "user" {
		skillKind, skillID = "cline_global_skill_directory", "cline-skill:"+r.Skill
	}
	expected := map[string]domain.NativeObjectOwnership{
		"managed_package_directory": {ObjectID: "package:cline:" + physical, Kind: "managed_package_directory", LogicalName: clineCycleName, Path: active, ManagedDigest: digest, ProtectionClass: "managed"},
		skillKind:                   {ObjectID: skillID, Kind: skillKind, LogicalName: r.Skill, Path: filepath.Dir(r.SkillPath), SourceRelative: "skills/" + r.Skill, ManagedDigest: skillDigest, ProtectionClass: "managed"},
	}
	if r.Scope != "user" {
		canonical, _ := json.Marshal(entry)
		mcpDigest := "sha256:" + clineAttemptSHA(append([]byte("agentplugins-native-mcp-v1\x00cline-mcpServers\x00uapclinea\x00"), canonical...))
		expected["cline_project_mcp_server"] = domain.NativeObjectOwnership{ObjectID: "cline_project_mcp_server:uapclinea", Kind: "cline_project_mcp_server", LogicalName: "uapclinea", Path: r.Config, ManagedDigest: mcpDigest, ProtectionClass: "managed"}
	}
	if err := clineAttemptOwnership(b.NativeObjects, expected); err != nil {
		t.Fatal(err)
	}
	r.Binding = b
}
func (c *clineProjectCycle) dataProof(t *testing.T, r *clineCycleRecipe, i domain.Installation, id, physical string) {
	t.Helper()
	d, err := c.ownedDataReceipt(r, i, id, physical)
	if err != nil {
		t.Fatal(err)
	}
	locator := d.Locator
	marker := filepath.Join(locator, ".agentplugins-data-owner.json")
	raw, err := projectNativeRuntimeRead(marker, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectNativeRuntimeJSON(raw); err != nil {
		t.Fatal(err)
	}
	var actual domain.DataReceipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&actual) != nil || !reflect.DeepEqual(actual, d) {
		t.Fatal("Cline cycle held data marker differs from receipt")
	}
	if r.Installation.InstallationID != "" && len(r.Installation.DataReceipts) != 0 && !reflect.DeepEqual(r.Installation.DataReceipts, i.DataReceipts) {
		t.Fatal("Cline cycle retained data receipt changed")
	}
	tree, err := clineAttemptTree(locator)
	if err != nil {
		t.Fatal(err)
	}
	if r.Data == "" {
		want := map[string]string{".": fmt.Sprintf("directory:%o", os.ModeDir|0700), ".agentplugins-data-owner.json": "file:600:" + clineAttemptSHA(raw)}
		if !reflect.DeepEqual(tree, want) {
			t.Fatal("Cline first owned data contains unrecorded payload")
		}
	}
	r.Data = locator
	c.f.trees[locator] = tree
	c.f.protected[marker] = clineAttemptSHA(raw)
}
func (c *clineProjectCycle) configProof(t *testing.T, r *clineCycleRecipe, installed bool) map[string]any {
	t.Helper()
	raw, err := projectNativeRuntimeRead(r.Config, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	value, err := projectNativeRuntimeJSON(raw)
	doc, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(doc, "mcpServers foreignPreference", "") != nil || doc["foreignPreference"] != "retain-"+r.Scope {
		t.Fatal("Cline cycle native config root differs")
	}
	servers, ok := doc["mcpServers"].(map[string]any)
	foreign := map[string]any{"transport": map[string]any{"type": "streamableHttp", "url": "http://127.0.0.1:9/foreign-" + r.Scope}, "disabled": true}
	count := 1
	if installed && r.Scope != "user" {
		count = 2
	}
	if !ok || len(servers) != count || !reflect.DeepEqual(servers["foreign-"+r.Scope], foreign) {
		t.Fatal("Cline cycle foreign/server inventory differs")
	}
	if count == 1 {
		return nil
	}
	entry, ok := servers["uapclinea"].(map[string]any)
	env := map[string]any{"PLUGIN_ROOT": r.Active, "PLUGIN_DATA": r.Data, "UAP_TEST_ROOT": c.f.base.Root, "UAP_TEST_EVENTS": c.f.base.Events, "UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision}
	if !ok || clineAttemptServer(entry, filepath.Join(r.Active, "bin", "probe"), r.Active, []any{"Owned Cline " + r.Scope, r.Active + "/argument"}, env) != nil {
		t.Fatal("Cline cycle exact source-bound transport differs")
	}
	return entry
}
func (c *clineProjectCycle) foreign(t *testing.T, r *clineCycleRecipe) {
	t.Helper()
	c.configProof(t, r, r.Active != "" && !r.Removed)
	c.f.check(t)
}
func (c *clineProjectCycle) skillInventory(t *testing.T, r *clineCycleRecipe) {
	t.Helper()
	root := filepath.Dir(filepath.Dir(r.SkillPath))
	tree, err := clineAttemptTree(root)
	if err != nil {
		t.Fatal(err)
	}
	foreign := "foreign-" + r.Scope
	body := []byte("---\nname: " + foreign + "\ndescription: Foreign inert marker\n---\nFOREIGN_" + r.Scope + "\n")
	directory := fmt.Sprintf("directory:%o", os.ModeDir|0700)
	want := map[string]string{".": directory, foreign: directory, filepath.Join(foreign, "SKILL.md"): "file:600:" + clineAttemptSHA(body)}
	if !r.Removed && r.Active != "" {
		want[r.Skill] = fmt.Sprintf("directory:%o", os.ModeDir|0755)
		want[filepath.Join(r.Skill, "SKILL.md")] = "file:644:" + clineAttemptSHA(r.Authored["skills/"+r.Skill+"/SKILL.md"])
	}
	if !reflect.DeepEqual(tree, want) {
		t.Fatal("Cline complete native skill tree differs from finite source recipe")
	}
	c.f.trees[root] = tree
}
func (c *clineProjectCycle) removedProof(t *testing.T, r *clineCycleRecipe, i domain.Installation) {
	t.Helper()
	if len(i.Clients) != 0 || !i.DataRetained {
		t.Fatal("Cline cycle default removal did not retain data-only installation")
	}
	physical := r.Binding.PhysicalArtifact
	id := r.Binding.DataReceiptID
	if id == "" {
		id = "data_" + clineAttemptSHA([]byte(strings.Join([]string{i.InstallationID, physical, scopeName(r)}, "\x00")))[:24]
	}
	c.dataProof(t, r, i, id, physical)
	for _, path := range []string{r.Active, filepath.Dir(r.SkillPath)} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("removed Cline cycle owned object remains or is aliased")
		}
	}
	c.configProof(t, r, false)
}
func clineCycleInstalled(f *clineProjectAttempt, r *clineCycleRecipe, entry map[string]any) error {
	entries, err := clineAttemptPayload(r.Active)
	if err != nil {
		return err
	}
	want := map[string][]byte{}
	executable := map[string]bool{}
	for path, body := range r.Authored {
		want[path] = body
		executable[path] = path == "bin/probe"
	}
	if r.Scope != "user" {
		want[".agentplugins-cline-native.json"], _ = json.Marshal(map[string]any{"servers": map[string]any{"uapclinea": entry["transport"]}})
		want["io.github.777genius.agentplugins/managed-stdio-v1/agentplugins"] = nil
		executable["io.github.777genius.agentplugins/managed-stdio-v1/agentplugins"] = true
		want["io.github.777genius.agentplugins/managed-stdio-v1/metadata.json"], _ = json.Marshal(map[string]any{"protocol": 1, "sha256": f.immutable[f.installer], "cliVersion": "0.1.0-development"})
	}
	dirs := map[string]bool{}
	for path := range want {
		for dir := filepath.ToSlash(filepath.Dir(path)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			dirs[dir] = true
		}
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Path] {
			return fmt.Errorf("duplicate Cline installed member")
		}
		seen[e.Path] = true
		if e.Kind == "directory" {
			if !dirs[e.Path] {
				return fmt.Errorf("extra Cline installed directory")
			}
			continue
		}
		body, ok := want[e.Path]
		if !ok || e.Kind != "file" || e.Executable != executable[e.Path] {
			return fmt.Errorf("extra Cline installed member/mode")
		}
		switch e.Path {
		case "io.github.777genius.agentplugins/managed-stdio-v1/agentplugins":
			if clineAttemptSHA(e.Content) != f.immutable[f.installer] {
				return fmt.Errorf("Cline installed launcher identity")
			}
		case "mcp.json", ".agentplugins-cline-native.json", "io.github.777genius.agentplugins/managed-stdio-v1/metadata.json":
			actual, err := projectNativeRuntimeJSON(e.Content)
			expected, werr := projectNativeRuntimeJSON(body)
			if err != nil || werr != nil || !reflect.DeepEqual(actual, expected) {
				return fmt.Errorf("Cline installed exact JSON projection")
			}
		default:
			if !bytes.Equal(body, e.Content) {
				return fmt.Errorf("Cline installed bytes differ from recipe")
			}
		}
	}
	if len(seen) != len(want)+len(dirs) {
		return fmt.Errorf("Cline installed closed payload incomplete")
	}
	return nil
}
