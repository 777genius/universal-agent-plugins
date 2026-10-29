//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var qoderCycleID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type qoderCycleDelivery struct {
	Active   string            `json:"active"`
	View     string            `json:"view"`
	Files    map[string]string `json:"files"`
	Manifest *qoderCycleBytes  `json:"manifest"`
	Skill    *qoderCycleBytes  `json:"skill"`
}

type qoderCycleData struct {
	Allocated bool                `json:"allocated"`
	Path      string              `json:"path"`
	Receipt   *domain.DataReceipt `json:"receipt"`
	Owner     *qoderCycleBytes    `json:"owner"`
	Files     map[string]string   `json:"files"`
}

func qoderCycleAbsent(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("Qoder absence path is noncanonical")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("Qoder exact owned absence required")
	}
	for parent, depth := filepath.Dir(path), 0; ; parent, depth = filepath.Dir(parent), depth+1 {
		if depth > 128 {
			return fmt.Errorf("Qoder absent ancestor bound")
		}
		info, err := os.Lstat(parent)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return fmt.Errorf("Qoder absent path has alias/non-directory parent")
		}
		if filepath.Dir(parent) == parent {
			return nil
		}
	}
}

func qoderCycleExpectedSource(i domain.Installation, r *qoderCycleRecipe) error {
	source := domain.SourceBinding{SourceBindingID: "src_" + qoderDeltaHash(r.Source + "\x00\x00")[:24], RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: r.Digest}
	pkg := domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: qoderCycleName, Version: "1.0.0", ManifestDigest: "sha256:" + qoderDeltaHash(string(r.Authored["plugin.json"])), Inventory: domain.ComponentInventory{Skills: []string{qoderCycleName}}}
	want := domain.Installation{InstallationID: i.InstallationID, DeclaredName: qoderCycleName, Source: source, Package: pkg, OriginMode: domain.OriginModeDirect, OperationGroupID: i.OperationGroupID, DataReceipts: i.DataReceipts, DataRetained: i.DataRetained, Clients: i.Clients, CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt}
	if !qoderCycleID.MatchString(i.InstallationID) || !qoderCycleOperation.MatchString(i.OperationGroupID) || !qoderCycleTimes(i.CreatedAt, i.UpdatedAt) || !reflect.DeepEqual(i, want) || r.Installation.CreatedAt != "" && i.CreatedAt != r.Installation.CreatedAt {
		return fmt.Errorf("Qoder exact independently authored installation/source/package differs")
	}
	return nil
}

func (c *qoderSkillsCycle) binding(t *testing.T, r *qoderCycleRecipe, i domain.Installation) {
	t.Helper()
	if len(i.Clients) != 1 || i.DataRetained || len(i.DataReceipts) != 0 {
		t.Fatal("Qoder skills-only exact installed client/no-data state required")
	}
	physical := qoderCycleName + "-" + qoderDeltaHash(i.InstallationID)[:12]
	base := filepath.Join(r.Project, ".uap", "managed", "clients", "qoder")
	active := filepath.Join(base, physical)
	id := "client_" + qoderDeltaHash(strings.Join([]string{i.InstallationID, "qoder", "project", active}, "\x00"))[:24]
	b, ok := i.Clients[id]
	revision := &domain.ClientPackageRevision{Version: "1.0.0", TreeDigest: r.Digest, ManifestDigest: "sha256:" + qoderDeltaHash(string(r.Authored["plugin.json"]))}
	want := domain.ClientBinding{ClientBindingID: id, ClientID: "qoder", Scope: "project", TargetLocator: active, PhysicalArtifact: physical, NativeProfileRoot: base, Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, AffectedSurfaces: []string{"qoder"}, PackageRevision: revision, NativeObjects: b.NativeObjects, Receipts: b.Receipts, UpdatedAt: b.UpdatedAt}
	if !ok || !reflect.DeepEqual(b, want) || !qoderCycleTimes(i.CreatedAt, b.UpdatedAt) || len(b.NativeObjects) != 1 {
		t.Fatal("Qoder derived Prepared binding differs")
	}
	if r.Binding.ClientBindingID != "" && (r.Binding.ClientBindingID != id || r.Binding.PhysicalArtifact != physical || r.Binding.TargetLocator != active) {
		t.Fatal("Qoder lifecycle changed stable binding/physical authority")
	}
	for key, other := range c.recipes {
		if key != r.Key && other.Installation.InstallationID != "" && (other.Installation.InstallationID == i.InstallationID || other.Binding.ClientBindingID == id || other.Binding.PhysicalArtifact == physical) {
			t.Fatal("Qoder same-name projects share logical/physical authority")
		}
	}
	r.Binding, r.Active, r.View = b, active, filepath.Join(active, ".agentplugins-qoder-native")
}

func (c *qoderSkillsCycle) proof(t *testing.T, r *qoderCycleRecipe, stage string) {
	t.Helper()
	raw := mustQoderCycleRead(t, c.statePath(r))
	state, err := clineCycleState(raw)
	if err != nil || qoderCycleRawState(raw) != nil || len(state.Installations) != 1 {
		t.Fatal("Qoder exact single current-schema state required", err)
	}
	i := state.Installations[0]
	if err := qoderCycleExpectedSource(i, r); err != nil {
		t.Fatal(err)
	}
	if r.Installation.InstallationID != "" && i.InstallationID != r.Installation.InstallationID {
		t.Fatal("Qoder installation identity changed")
	}
	prior := r.Binding
	if r.Removed {
		if len(i.Clients) != 0 || !i.DataRetained {
			t.Fatal("Qoder default removal did not retain data-only installation")
		}
	} else {
		c.binding(t, r, i)
	}
	if err := c.receipts(r, i, prior, state.TransactionReceipts, stage); err != nil {
		t.Fatal(err)
	}
	delivery := c.delivery(t, r)
	data := c.data(t, r, i)
	r.Installation = i
	c.append(t, qoderCycleAggregates[2], stage, map[string]any{"scope": r.Key, "revision": r.Revision, "removed": r.Removed, "source_digest": r.Digest, "state": qoderCycleStream(t, raw), "binding": r.Binding, "delivery": delivery, "data": data})
	c.f.protected[c.statePath(r)] = qoderDeltaHash(string(raw))
	c.check(t)
}

func qoderCycleNativeManifest() []byte {
	raw, _ := json.MarshalIndent(map[string]any{"name": qoderCycleName, "version": "1.0.0", "description": qoderCycleDescription, "skills": "./skills/"}, "", "  ")
	return append(raw, '\n')
}

func (c *qoderSkillsCycle) delivery(t *testing.T, r *qoderCycleRecipe) qoderCycleDelivery {
	t.Helper()
	out := qoderCycleDelivery{Active: r.Active, View: r.View}
	if r.Removed || r.Missing {
		if err := qoderCycleAbsent(r.Active); err != nil {
			t.Fatal(err)
		}
		return out
	}
	files := map[string][]byte{}
	for name, body := range r.Authored {
		files[name] = body
	}
	manifest := qoderCycleNativeManifest()
	skill := r.Authored["skills/"+qoderCycleName+"/SKILL.md"]
	files[".agentplugins-qoder-native/.qoder-plugin/plugin.json"] = manifest
	files[".agentplugins-qoder-native/skills/"+qoderCycleName+"/SKILL.md"] = skill
	want := map[string]string{".": "directory:20000000755"}
	for relative, body := range files {
		want[relative] = "file:644:" + qoderDeltaHash(string(body))
		for parent := filepath.Dir(relative); parent != "."; parent = filepath.Dir(parent) {
			want[parent] = "directory:20000000755"
		}
		actual := mustQoderCycleRead(t, filepath.Join(r.Active, relative))
		if !bytes.Equal(actual, body) {
			t.Fatal("Qoder exact authored/projected package leaf differs")
		}
	}
	actual, err := clineAttemptTree(r.Active)
	if err != nil || !reflect.DeepEqual(actual, want) {
		t.Fatal("Qoder full ordinary selected payload membership/mode differs", err)
	}
	digest, err := projectNativeCLIPackageDigest(c.f.ctx, r.Active)
	if err != nil {
		t.Fatal(err)
	}
	expected := domain.NativeObjectOwnership{ObjectID: "package:qoder:" + r.Binding.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: qoderCycleName, Path: r.Active, ManagedDigest: digest, ProtectionClass: "managed"}
	if !reflect.DeepEqual(r.Binding.NativeObjects, []domain.NativeObjectOwnership{expected}) {
		t.Fatal("Qoder sole package receipt does not join exact projected text payload")
	}
	out.Files = actual
	manifestStream, skillStream := qoderCycleStream(t, manifest), qoderCycleStream(t, skill)
	out.Manifest, out.Skill = &manifestStream, &skillStream
	c.trees[r.Active] = actual
	return out
}

func (c *qoderSkillsCycle) data(t *testing.T, r *qoderCycleRecipe, i domain.Installation) qoderCycleData {
	t.Helper()
	physical := qoderCycleName + "-" + qoderDeltaHash(i.InstallationID)[:12]
	path := filepath.Join(r.Project, ".uap", "plugin-data", physical)
	out := qoderCycleData{Path: path}
	if !r.Removed {
		if len(i.DataReceipts) != 0 || r.Binding.DataReceiptID != "" || qoderCycleAbsent(path) != nil {
			t.Fatal("Qoder skills-only installed package acquired unproved data")
		}
		return out
	}
	id := "data_" + qoderDeltaHash(strings.Join([]string{i.InstallationID, physical, "project"}, "\x00"))[:24]
	receipt := i.DataReceipts[id]
	if len(i.DataReceipts) != 1 || receipt.DataReceiptID != id || receipt.PhysicalBackend != physical || receipt.Scope != "project" || receipt.Locator != path || receipt.State != domain.DataReceiptOwned || !qoderCycleTimes(receipt.CreatedAt, receipt.UpdatedAt) || receipt.OwnershipDigest != "sha256:"+qoderDeltaHash("agentplugins-plugin-data-v1\x00"+id+"\x00"+path) {
		t.Fatal("Qoder actual default retained data authority differs")
	}
	if len(r.Installation.DataReceipts) != 0 && !reflect.DeepEqual(i.DataReceipts, r.Installation.DataReceipts) {
		t.Fatal("Qoder retained receipt changed")
	}
	ownerPath := filepath.Join(path, ".agentplugins-data-owner.json")
	owner := mustQoderCycleRead(t, ownerPath)
	if _, err := projectNativeRuntimeJSON(owner); err != nil {
		t.Fatal(err)
	}
	var actual domain.DataReceipt
	decoder := json.NewDecoder(bytes.NewReader(owner))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&actual); err != nil || !reflect.DeepEqual(actual, receipt) {
		t.Fatal("Qoder actual owner marker differs from retained receipt", err)
	}
	want := map[string]string{".": "directory:20000000700", ".agentplugins-data-owner.json": "file:600:" + qoderDeltaHash(string(owner))}
	inventory, err := clineAttemptTree(path)
	if err != nil || !reflect.DeepEqual(inventory, want) {
		t.Fatal("Qoder retained data contains unexpected membership/mode/bytes", err)
	}
	stream := qoderCycleStream(t, owner)
	out.Allocated, out.Receipt, out.Owner, out.Files = true, &receipt, &stream, inventory
	c.trees[path] = inventory
	c.f.protected[ownerPath] = qoderDeltaHash(string(owner))
	for _, held := range []string{path, ownerPath} {
		if _, exists := c.original[held]; !exists {
			c.hold(t, held)
		}
	}
	return out
}
