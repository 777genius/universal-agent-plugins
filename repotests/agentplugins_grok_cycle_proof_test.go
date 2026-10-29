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

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func grokCycleSource(i domain.Installation, r *grokCycleRecipe) error {
	source := domain.SourceBinding{SourceBindingID: "src_" + clineAttemptSHA([]byte(r.Source + "\x00\x00"))[:24], RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: r.Digest}
	pack := domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: r.Name, Version: "1.0.0", ManifestDigest: "sha256:" + clineAttemptSHA(r.Authored["plugin.json"]), Inventory: domain.ComponentInventory{Skills: []string{r.Name}}}
	if i.InstallationID == "" || i.DeclaredName != r.Name || !reflect.DeepEqual(i.Source, source) || !reflect.DeepEqual(i.Package, pack) {
		return fmt.Errorf("Grok cycle source/package differs from independent authored recipe")
	}
	return nil
}

func grokCycleProjected(r *grokCycleRecipe) map[string][]byte {
	manifest, _ := json.MarshalIndent(map[string]any{"name": r.Name, "version": "1.0.0", "description": "Disposable Grok skills cycle", "skills": "./skills/"}, "", "  ")
	return map[string][]byte{"plugin.json": r.Authored["plugin.json"], "skills/" + r.Name + "/SKILL.md": r.Authored["skills/"+r.Name+"/SKILL.md"], ".grok-plugin/plugin.json": append(manifest, '\n')}
}

func grokCyclePayload(snapshot map[string]grokCycleFile, expected map[string][]byte) error {
	directories := map[string]bool{".": true}
	for path := range expected {
		for dir := filepath.ToSlash(filepath.Dir(path)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			directories[dir] = true
		}
	}
	count := len(expected) + len(directories)
	if len(snapshot) != count {
		return fmt.Errorf("Grok cycle complete package membership differs")
	}
	for path, entry := range snapshot {
		if directories[path] {
			if entry.Kind != "directory" || entry.Mode != 0755 || entry.Content != nil {
				return fmt.Errorf("Grok cycle package directory mode differs")
			}
			continue
		}
		body, ok := expected[path]
		if !ok || entry.Kind != "file" || entry.Mode != 0644 || entry.Content == nil || !bytes.Equal(entry.Content.Body, body) {
			return fmt.Errorf("Grok cycle package bytes or file mode differ")
		}
	}
	return nil
}

func (c *grokProjectCycle) bindingProof(t *testing.T, r *grokCycleRecipe, i domain.Installation, stage string) map[string]grokCycleFile {
	t.Helper()
	if len(i.Clients) != 1 || len(i.DataReceipts) != 0 || i.DataRetained {
		t.Fatal("Grok skill-only installation must have one binding and zero data")
	}
	var b domain.ClientBinding
	for _, value := range i.Clients {
		b = value
	}
	physical := r.Name + "-" + clineAttemptSHA([]byte(i.InstallationID))[:12]
	active := filepath.Join(r.Project, ".grok", "plugins", physical)
	id := "client_" + clineAttemptSHA([]byte(strings.Join([]string{i.InstallationID, "grok", "project", active}, "\x00")))[:24]
	revision := &domain.ClientPackageRevision{Version: "1.0.0", TreeDigest: r.Digest, ManifestDigest: "sha256:" + clineAttemptSHA(r.Authored["plugin.json"])}
	if b.ClientID != "grok" || b.Scope != "project" || b.TargetLocator != active || b.PhysicalArtifact != physical || b.ClientBindingID != id || b.NativeProfileRoot != filepath.Join(r.Project, ".grok") || b.Materialization != domain.MaterializationMaterialized || b.Activation != domain.ActivationPrepared || b.Verification != domain.VerificationPackageValid || b.Authentication != domain.AuthenticationNotChecked || b.Policy != domain.PolicyAllowed || !reflect.DeepEqual(b.PackageRevision, revision) || b.DataReceiptID != "" || b.NativeActivationAttempt != "" || b.PendingNativeIntent != nil || !b.SelectedDelivery.IsZero() || b.InstallIntent != "" || b.ProfileNamespace != "" || b.ProfileAuthority != nil || b.LocalEntryObservation != nil || !reflect.DeepEqual(b.AffectedSurfaces, []string{"grok"}) {
		t.Fatal("Grok exact Prepared project binding differs")
	}
	if r.Binding.ClientBindingID != "" && (r.Binding.ClientBindingID != id || r.Binding.PhysicalArtifact != physical || r.Installation.InstallationID != i.InstallationID) {
		t.Fatal("Grok cycle update/repair changed logical binding authority")
	}
	for scope, other := range c.recipes {
		if scope != r.Scope && other.Installation.InstallationID != "" && (other.Installation.InstallationID == i.InstallationID || other.Binding.ClientBindingID == id || other.Binding.PhysicalArtifact == physical) {
			t.Fatal("Grok A/B installation authority overlaps")
		}
	}
	snapshot, err := grokCycleSnapshot(active)
	if err != nil || grokCyclePayload(snapshot, grokCycleProjected(r)) != nil {
		t.Fatal("Grok selected payload differs from independent projection", err)
	}
	entries := []packagedigest.CapturedEntry{}
	for path, entry := range snapshot {
		if path == "." {
			continue
		}
		captured := packagedigest.CapturedEntry{Path: path, Kind: entry.Kind}
		if entry.Content != nil {
			captured.Content = entry.Content.Body
		}
		entries = append(entries, captured)
	}
	digest, err := projectNativeRuntimeInstalledDigest(c.f.ctx, entries)
	want := []domain.NativeObjectOwnership{{ObjectID: "package:grok:" + physical, Kind: "managed_package_directory", LogicalName: r.Name, Path: active, ManagedDigest: digest, ProtectionClass: "managed"}}
	if err != nil || !reflect.DeepEqual(b.NativeObjects, want) {
		t.Fatal("Grok package ownership digest differs from held expected bytes", err)
	}
	if err := c.swapReceipts(r, b, stage); err != nil || len(b.Receipts) == 0 || i.OperationGroupID != b.Receipts[len(b.Receipts)-1].OperationGroupID {
		t.Fatal("Grok public grouped receipt history differs", err)
	}
	r.Binding, r.Active = b, active
	data := filepath.Join(r.Project, ".uap", "plugin-data", physical)
	if _, err := os.Lstat(data); !os.IsNotExist(err) {
		t.Fatal("Grok skill-only package adopted allocated data")
	}
	c.absent[data] = true
	c.guards[active] = snapshot
	return snapshot
}

func (c *grokProjectCycle) retainedProof(t *testing.T, r *grokCycleRecipe, i domain.Installation) map[string]grokCycleFile {
	t.Helper()
	physical := r.Name + "-" + clineAttemptSHA([]byte(i.InstallationID))[:12]
	id := "data_" + clineAttemptSHA([]byte(strings.Join([]string{i.InstallationID, physical, "project"}, "\x00")))[:24]
	path := filepath.Join(r.Project, ".uap", "plugin-data", physical)
	data := i.DataReceipts[id]
	wantDigest := "sha256:" + clineAttemptSHA([]byte("agentplugins-plugin-data-v1\x00"+id+"\x00"+path))
	if !i.DataRetained || len(i.Clients) != 0 || len(i.DataReceipts) != 1 || data.DataReceiptID != id || data.PhysicalBackend != physical || data.Scope != "project" || data.Locator != path || data.OwnershipDigest != wantDigest || data.State != domain.DataReceiptOwned || r.Binding.PhysicalArtifact != physical {
		t.Fatal("Grok actual default-removal retained ownership receipt differs")
	}
	if r.Data != "" && !reflect.DeepEqual(r.Installation.DataReceipts, i.DataReceipts) {
		t.Fatal("Grok retained receipt changed after removal")
	}
	snapshot, err := grokCycleSnapshot(path)
	marker := snapshot[".agentplugins-data-owner.json"]
	if err != nil || len(snapshot) != 2 || snapshot["."].Kind != "directory" || snapshot["."].Mode != 0700 || marker.Kind != "file" || marker.Mode != 0600 || marker.Content == nil {
		t.Fatal("Grok retained initial data membership differs", err)
	}
	value, err := projectNativeRuntimeJSON(marker.Content.Body)
	want, _ := json.Marshal(data)
	expected, _ := projectNativeRuntimeJSON(want)
	if err != nil || !reflect.DeepEqual(value, expected) {
		t.Fatal("Grok retained marker bytes differ from actual owned receipt")
	}
	if _, err := os.Lstat(r.Active); !os.IsNotExist(err) {
		t.Fatal("Grok removed managed package still exists")
	}
	c.absent[r.Active] = true
	r.Data = path
	c.guards[path] = snapshot
	return snapshot
}

func (c *grokProjectCycle) preserved(t *testing.T) map[string]map[string]grokCycleFile {
	t.Helper()
	result := map[string]map[string]grokCycleFile{}
	for path, expected := range c.fixed {
		actual, err := grokCycleSnapshot(path)
		if err != nil || !reflect.DeepEqual(actual, expected) {
			t.Fatal("Grok original fixed authority changed", err)
		}
		result[path] = actual
	}
	return result
}

func (c *grokProjectCycle) proof(t *testing.T, r *grokCycleRecipe, stage string) {
	t.Helper()
	path := filepath.Join(r.Project, ".uap", "state-v2.json")
	raw, err := projectNativeRuntimeRead(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	state, err := clineCycleState(raw)
	if err != nil || len(state.Installations) != 1 {
		t.Fatal("Grok exact single current installation missing", err)
	}
	i := state.Installations[0]
	if err := grokCycleSource(i, r); err != nil {
		t.Fatal(err)
	}
	if r.Installation.InstallationID != "" && i.InstallationID != r.Installation.InstallationID {
		t.Fatal("Grok stable installation identity changed")
	}
	var payload map[string]grokCycleFile
	if r.Removed {
		payload = c.retainedProof(t, r, i)
	} else {
		payload = c.bindingProof(t, r, i, stage)
	}
	if err := c.transactionReceipts(r, i, state.TransactionReceipts); err != nil {
		t.Fatal(err)
	}
	r.Installation = i
	c.freeze(t, path, false)
	c.append(t, "grok-cycle-proof.jsonl", stage, map[string]any{"scope": r.Scope, "revision": r.Revision, "removed": r.Removed, "state": c.stream(t, raw), "binding": r.Binding, "source_digest": r.Digest, "payload": payload, "preserved": c.preserved(t)})
	c.proofs = append(c.proofs, stage)
	c.check(t)
}
