//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

type kiroCycleFile struct {
	info os.FileInfo
	body []byte
}

type kiroCycleRole struct {
	key, project, name, data, marker string
	installation                    domain.Installation
	binding                         domain.ClientBinding
	removed                         bool
}

func (r *kiroCycleRole) statePath() string { return filepath.Join(r.project, ".uap", "state-v2.json") }
func (r *kiroCycleRole) configPath() string { return filepath.Join(r.project, ".kiro", "settings", "mcp.json") }
func (r *kiroCycleRole) skillPath() string { return filepath.Join(r.project, ".kiro", "skills", r.name, "SKILL.md") }
func (r *kiroCycleRole) markerPath() string { return filepath.Join(r.data, "native-marker.txt") }

func kiroCycleCapture(path string) (kiroCycleFile, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return kiroCycleFile{}, err
	}
	body, err := projectNativeRuntimeRead(path, 1<<20)
	if err != nil {
		return kiroCycleFile{}, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || !projectNativeRuntimeStableStat(before.Sys(), after.Sys()) {
		return kiroCycleFile{}, fmt.Errorf("Kiro held snapshot identity changed")
	}
	return kiroCycleFile{before, body}, nil
}

func (c *kiroNativeCycle) pin(t *testing.T, path string) {
	t.Helper()
	snapshot, err := kiroCycleCapture(path)
	if err != nil {
		t.Fatal(err)
	}
	c.pins[path] = snapshot
}

func kiroCycleAbsent(path string) error {
	original := path
	for count := 0; count <= 32; count++ {
		info, err := os.Lstat(path)
		if err == nil {
			if path == original || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("Kiro unselected catalog or expected-absent member exists")
			}
			return projectNativeRuntimeParents(filepath.Join(path, "leaf"))
		}
		if !os.IsNotExist(err) || filepath.Dir(path) == path {
			return fmt.Errorf("Kiro expected absence is unproved")
		}
		path = filepath.Dir(path)
	}
	return fmt.Errorf("Kiro absent ancestry bound")
}

func (c *kiroNativeCycle) start(t *testing.T) {
	t.Helper()
	for key, project := range map[string]string{"a": c.f.base.Project, "b": c.f.other} {
		c.roles[key] = &kiroCycleRole{key: key, project: project, name: c.f.name, marker: "kiro-" + key + "-" + c.f.base.Nonce}
	}
	for _, path := range []string{c.f.base.Root, c.f.base.Home, c.f.base.Project, c.f.other, c.f.outside} {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("Kiro original directory authority")
		}
		c.roots[path] = info
	}
	for _, path := range []string{filepath.Join(c.f.base.Home, "original-user-marker.txt"), filepath.Join(c.f.base.Package, "plugin.json"), filepath.Join(c.f.base.Package, "mcp.json"), filepath.Join(c.f.base.Package, "skills", c.f.name, "SKILL.md")} {
		c.pin(t, path)
	}
	c.f.immutable[filepath.Join(c.f.base.Package, "bin", "probe")] = nativeSHA(t, c.f.base.Probe)
	if err := kiroCycleAbsent(c.f.base.Events); err != nil {
		t.Fatal(err)
	}
	c.observe(t, "a", "add-a", false)
	c.check(t)
}

func (c *kiroNativeCycle) check(t *testing.T) {
	t.Helper()
	if err := c.f.ctx.Err(); err != nil {
		t.Fatal(err)
	}
	c.f.check(t)
	for path, original := range c.roots {
		info, err := os.Lstat(path)
		left, okLeft := original.Sys().(*syscall.Stat_t)
		right, okRight := kiroCycleInfoSys(info).(*syscall.Stat_t)
		if err != nil || !okLeft || !okRight || !os.SameFile(original, info) || original.Mode() != info.Mode() || left.Uid != right.Uid || left.Gid != right.Gid {
			t.Fatal("Kiro original root/owner/mode changed")
		}
	}
	for path, original := range c.pins {
		now, err := kiroCycleCapture(path)
		if err != nil || !os.SameFile(original.info, now.info) || !projectNativeRuntimeStableStat(original.info.Sys(), now.info.Sys()) || !bytes.Equal(original.body, now.body) {
			t.Fatal("Kiro original profile/state/agent/data member changed", path)
		}
	}
	c.catalogClosure(t)
}

func kiroCycleInfoSys(info os.FileInfo) any {
	if info == nil {
		return nil
	}
	return info.Sys()
}

func (c *kiroNativeCycle) catalogClosure(t *testing.T) {
	t.Helper()
	paths := []string{filepath.Join(c.f.base.Home, ".kiro", "settings", "mcp.json")}
	for _, key := range []string{"a", "b"} {
		paths = append(paths, filepath.Join(c.f.base.Home, ".kiro", "agents", kiroCycleAgentName(key)+".json"))
	}
	for parent, count := filepath.Dir(c.f.base.Project), 0; ; parent, count = filepath.Dir(parent), count+1 {
		if count > 32 {
			t.Fatal("Kiro ancestor catalog bound")
		}
		paths = append(paths, filepath.Join(parent, ".kiro"))
		if filepath.Dir(parent) == parent {
			break
		}
	}
	for _, path := range paths {
		if err := kiroCycleAbsent(path); err != nil {
			t.Fatal(err)
		}
	}
}

func kiroCycleState(raw []byte) (domain.StateFileV2, error) {
	var state domain.StateFileV2
	if _, err := projectNativeRuntimeJSON(raw); err != nil {
		return state, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return state, err
	}
	if state.SchemaVersion != domain.StateSchemaVersion || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
		return state, fmt.Errorf("Kiro singleton Project state differs")
	}
	return state, nil
}

func (c *kiroNativeCycle) observe(t *testing.T, key, stage string, removed bool) {
	t.Helper()
	r := c.roles[key]
	raw, err := projectNativeRuntimeRead(r.statePath(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	state, err := kiroCycleState(raw)
	if err != nil {
		t.Fatal(err)
	}
	current := state.Installations[0]
	if r.installation.InstallationID != "" && !kiroCycleContinues(r.installation, current, removed) {
		t.Fatal("Kiro operation changed original source/binding/DATA authority")
	}
	if removed {
		c.removed(t, r, current)
	} else {
		c.installed(t, r, current)
	}
	r.installation, r.removed = current, removed
	c.pin(t, r.statePath())
	config, err := projectNativeRuntimeRead(r.configPath(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	c.pin(t, r.configPath())
	record := map[string]any{"stage": stage, "role": key, "project": r.project, "state_json": string(raw), "state_sha256": kiroCycleSHA(raw), "config_json": string(config), "config_sha256": kiroCycleSHA(config), "data": r.data, "removed": removed, "binding": r.binding}
	c.proofs = append(c.proofs, record)
	c.check(t)
}

func kiroCycleContinues(original, current domain.Installation, removed bool) bool {
	expected := original
	expected.UpdatedAt, expected.OperationGroupID = current.UpdatedAt, current.OperationGroupID
	if removed {
		expected.Clients, expected.DataRetained = current.Clients, true
		return len(current.Clients) == 0 && reflect.DeepEqual(expected, current)
	}
	if len(original.Clients) != 1 || len(current.Clients) != 1 {
		return false
	}
	expected.Clients = make(map[string]domain.ClientBinding, 1)
	for id, before := range original.Clients {
		after, ok := current.Clients[id]
		if !ok || len(after.Receipts) < len(before.Receipts) || !reflect.DeepEqual(before.Receipts, after.Receipts[:len(before.Receipts)]) {
			return false
		}
		before.Receipts, before.UpdatedAt = after.Receipts, after.UpdatedAt
		expected.Clients[id] = before
	}
	return reflect.DeepEqual(expected, current)
}

func (c *kiroNativeCycle) installed(t *testing.T, r *kiroCycleRole, current domain.Installation) {
	t.Helper()
	if current.DeclaredName != c.f.name || current.Source.CanonicalSource != c.f.base.Package || current.Source.SourceBindingID == "" || current.Source.TreeDigest == "" || current.Package.DeclaredName != c.f.name || current.Package.Version != "1.0.0" || current.Package.ManifestDigest == "" || len(current.Clients) != 1 || len(current.DataReceipts) != 1 {
		t.Fatal("Kiro installed source/package singleton differs")
	}
	var binding domain.ClientBinding
	for _, b := range current.Clients {
		binding = b
	}
	active := filepath.Join(r.project, ".uap", "managed", "clients", "kiro", binding.PhysicalArtifact)
	if binding.ClientID != "kiro" || binding.Scope != "project" || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || binding.PhysicalArtifact == "" || binding.TargetLocator != active || binding.NativeProfileRoot != filepath.Join(r.project, ".kiro") {
		t.Fatal("Kiro receipt-selected package/profile differs")
	}
	data := current.DataReceipts[binding.DataReceiptID]
	if data.DataReceiptID != binding.DataReceiptID || data.State != domain.DataReceiptOwned || data.Scope != "project" || data.PhysicalBackend != binding.PhysicalArtifact || data.OwnershipDigest == "" {
		t.Fatal("Kiro exact owned Project DATA differs")
	}
	nativeRequireContained(t, filepath.Join(r.project, ".uap", "plugin-data"), data.Locator)
	r.data, r.binding = data.Locator, binding
	for key, other := range c.roles {
		if key != r.key && other.data != "" && (other.data == r.data || other.binding.TargetLocator == active) {
			t.Fatal("Kiro A/B physical DATA/package overlap")
		}
	}
	c.installedObjects(t, r)
	for _, path := range []string{filepath.Join(r.project, ".kiro"), filepath.Dir(r.configPath()), r.data} {
		if c.roots[path] == nil {
			info, err := os.Lstat(path)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				t.Fatal("Kiro original selected profile/DATA directory", err)
			}
			c.roots[path] = info
		}
	}
	c.pin(t, r.skillPath())
	for _, path := range []string{filepath.Join(active, "bin", "probe"), filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)} {
		digest, err := projectNativeRuntimeDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		c.f.immutable[path] = digest
	}
}

func (c *kiroNativeCycle) installedObjects(t *testing.T, r *kiroCycleRole) {
	t.Helper()
	raw, err := projectNativeRuntimeRead(r.configPath(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := kiroCycleConfig(raw, r.name, false)
	if err != nil {
		t.Fatal(err)
	}
	active := r.binding.TargetLocator
	args := []any{"--internal-stdio-v1", active, r.data, active, "plugin", "--", "./bin/probe", "Owned kiro", active + "/argument"}
	env := map[string]any{"PLUGIN_ROOT": active, "PLUGIN_DATA": r.data, "UAP_TEST_ROOT": c.f.base.Root, "UAP_TEST_EVENTS": c.f.base.Events, "UAP_TEST_NONCE": c.f.base.Nonce, "UAP_TEST_REVISION": "a-one"}
	if err := projectNativeCLIMixedEntry(entry, filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName), args, env); err != nil {
		t.Fatal(err)
	}
	skill, err := projectNativeRuntimeRead(r.skillPath(), 1<<20)
	source := c.pins[filepath.Join(c.f.base.Package, "skills", r.name, "SKILL.md")].body
	if err != nil || !bytes.Equal(skill, source) {
		t.Fatal("Kiro exact source Skill projection differs")
	}
	digest, err := projectNativeCLIPackageDigest(c.f.ctx, active)
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := json.Marshal(entry)
	skillDigest := kiroCycleSHA(append(append([]byte("f\x00SKILL.md\x00-\x00"), skill...), 0))
	expected := map[string]domain.NativeObjectOwnership{
		"managed_package_directory": {ObjectID: "package:kiro:" + r.binding.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: r.name, Path: active, ManagedDigest: digest},
		"kiro_global_skill_directory": {ObjectID: "kiro-skill:" + r.name, Kind: "kiro_global_skill_directory", LogicalName: r.name, Path: filepath.Dir(r.skillPath()), ManagedDigest: "sha256:" + skillDigest},
		"kiro_global_mcp_server": {ObjectID: "kiro-mcp:" + r.name, Kind: "kiro_global_mcp_server", LogicalName: r.name, Path: r.configPath(), ManagedDigest: "sha256:" + kiroCycleSHA(canonical)},
	}
	if err := projectNativeCLIOwnership(r.binding.NativeObjects, expected); err != nil {
		t.Fatal(err)
	}
}

func kiroCycleConfig(raw []byte, name string, removed bool) (map[string]any, error) {
	value, err := projectNativeRuntimeJSON(raw)
	document, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(document, "mcpServers", "") != nil {
		return nil, fmt.Errorf("Kiro closed native MCP document differs")
	}
	entries, ok := document["mcpServers"].(map[string]any)
	if !ok || removed && len(entries) != 0 || !removed && len(entries) != 1 {
		return nil, fmt.Errorf("Kiro selected native MCP membership differs")
	}
	if removed {
		return nil, nil
	}
	entry, ok := entries[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Kiro receipt-selected native MCP entry is missing")
	}
	return entry, nil
}

func (c *kiroNativeCycle) removed(t *testing.T, r *kiroCycleRole, current domain.Installation) {
	t.Helper()
	if len(current.Clients) != 0 || !current.DataRetained || !reflect.DeepEqual(current.DataReceipts, r.installation.DataReceipts) {
		t.Fatal("Kiro removal lost DATA or retained a binding")
	}
	for _, path := range []string{r.binding.TargetLocator, filepath.Dir(r.skillPath())} {
		if err := kiroCycleAbsent(path); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := projectNativeRuntimeRead(r.configPath(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kiroCycleConfig(raw, r.name, true); err != nil {
		t.Fatal(err)
	}
}

func (c *kiroNativeCycle) missingSkill(t *testing.T) {
	t.Helper()
	c.check(t)
	r := c.roles["a"]
	original := c.pins[r.skillPath()]
	if original.info == nil || original.info.Mode().Perm() != 0600 {
		t.Fatal("Kiro exact owned Skill fault authority")
	}
	delete(c.pins, r.skillPath())
	delete(c.f.protected, r.skillPath())
	if err := os.Remove(r.skillPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(r.skillPath())); err != nil {
		t.Fatal(err)
	}
	if err := kiroCycleAbsent(filepath.Dir(r.skillPath())); err != nil {
		t.Fatal(err)
	}
}

func kiroCycleSHA(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func TestKiroCycleStateContinuityRetainsIndependentData(t *testing.T) {
	original := domain.Installation{
		InstallationID: "original", DeclaredName: "fixture", UpdatedAt: "before",
		Clients: map[string]domain.ClientBinding{"kiro": {ClientID: "kiro", DataReceiptID: "data", UpdatedAt: "before"}},
		DataReceipts: map[string]domain.DataReceipt{"data": {DataReceiptID: "data", Locator: "/original/data", State: domain.DataReceiptOwned}},
	}
	clone := func() domain.Installation {
		raw, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		var value domain.Installation
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	repaired := clone()
	repaired.UpdatedAt = "after"
	binding := repaired.Clients["kiro"]
	binding.UpdatedAt = "after"
	repaired.Clients["kiro"] = binding
	if !kiroCycleContinues(original, repaired, false) {
		t.Fatal("timestamp-only repaired binding refused")
	}
	removed := clone()
	removed.Clients, removed.DataRetained = map[string]domain.ClientBinding{}, true
	if !kiroCycleContinues(original, removed, true) {
		t.Fatal("owned removal with retained DATA refused")
	}
	for _, field := range []string{"data", "binding", "installation"} {
		changed := clone()
		switch field {
		case "data":
			row := changed.DataReceipts["data"]
			row.Locator = "/foreign/data"
			changed.DataReceipts["data"] = row
		case "binding":
			row := changed.Clients["kiro"]
			row.DataReceiptID = "foreign"
			changed.Clients["kiro"] = row
		case "installation":
			changed.InstallationID = "foreign"
		}
		if kiroCycleContinues(original, changed, false) {
			t.Fatal("unrelated authority drift admitted", field)
		}
	}
}
