//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (c *cursorMCPProjectCycle) readState(t *testing.T, r *cursorCycleRecipe) (domain.StateFileV2, []byte) {
	t.Helper()
	raw, err := projectNativeRuntimeRead(c.statePath(r), 1<<20)
	var state domain.StateFileV2
	if err != nil || json.Unmarshal(raw, &state) != nil || cursorCycleDocument(raw, state) != nil || state.SchemaVersion != 5 || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatal("Cursor closed single-installation state differs", err)
	}
	i := state.Installations[0]
	if !copilotReadinessID(i.InstallationID) || i.DeclaredName != cursorCycleName || i.OriginMode != domain.OriginModeDirect || i.Directory != nil || i.NeedsRebind || i.LocalChatGPTMapping != nil || len(i.InstallPreferences) != 0 {
		t.Fatal("Cursor selected installation identity differs")
	}
	if r.Installation.InstallationID != "" && i.InstallationID != r.Installation.InstallationID {
		t.Fatal("Cursor installation identity changed")
	}
	for _, stamp := range []string{i.CreatedAt, i.UpdatedAt} {
		value, err := time.Parse(time.RFC3339Nano, stamp)
		if err != nil || value.Before(c.f.started) || value.After(time.Now()) {
			t.Fatal("Cursor installation time differs")
		}
	}
	source := domain.SourceBinding{SourceBindingID: "src_" + clineAttemptSHA([]byte(r.Source + "\x00\x00"))[:24], RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: r.Digest}
	pack := domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: cursorCycleName, Version: r.Version, ManifestDigest: "sha256:" + clineAttemptSHA(r.Authored["plugin.json"]), Inventory: domain.ComponentInventory{MCPPresent: true, MCPEnabled: true, MCPServers: []string{"uapcursora"}}}
	if !reflect.DeepEqual(i.Source, source) || !reflect.DeepEqual(i.Package, pack) {
		t.Fatal("Cursor independent source/package recipe differs")
	}
	return state, raw
}

func (c *cursorMCPProjectCycle) currentData(t *testing.T, r *cursorCycleRecipe, i domain.Installation) {
	t.Helper()
	physical := cursorCycleName + "-" + clineAttemptSHA([]byte(i.InstallationID))[:12]
	id := "data_" + clineAttemptSHA([]byte(strings.Join([]string{i.InstallationID, physical, "project"}, "\x00")))[:24]
	d := i.DataReceipts[id]
	path := filepath.Join(r.Project, ".uap", "plugin-data", physical)
	want := "sha256:" + clineAttemptSHA([]byte("agentplugins-plugin-data-v1\x00"+id+"\x00"+path))
	if len(i.DataReceipts) != 1 || d.DataReceiptID != id || d.PhysicalBackend != physical || d.Scope != "project" || d.Locator != path || d.OwnershipDigest != want || d.State != domain.DataReceiptOwned {
		t.Fatal("Cursor persistent data authority differs")
	}
	if r.Data.DataReceiptID != "" && !reflect.DeepEqual(r.Data, d) {
		t.Fatal("Cursor persistent data receipt changed")
	}
	for scope, other := range c.recipes {
		if scope != r.Scope && other.Installation.InstallationID != "" && (other.Installation.InstallationID == i.InstallationID || other.Data.DataReceiptID == id || other.Data.PhysicalBackend == physical) {
			t.Fatal("Cursor A/B physical/data authority overlaps")
		}
	}
	r.Data = d
	r.Active = filepath.Join(r.Project, ".uap", "managed", "clients", "cursor", physical)
}

func (c *cursorMCPProjectCycle) binding(t *testing.T, r *cursorCycleRecipe, i domain.Installation) domain.ClientBinding {
	t.Helper()
	id := "client_" + clineAttemptSHA([]byte(strings.Join([]string{i.InstallationID, "cursor", "project", r.Active}, "\x00")))[:24]
	b := i.Clients[id]
	if len(i.Clients) != 1 || i.DataRetained || b.ClientBindingID != id || b.ClientID != "cursor" || b.Scope != "project" || b.TargetLocator != r.Active || b.PhysicalArtifact != r.Data.PhysicalBackend || b.DataReceiptID != r.Data.DataReceiptID || b.NativeProfileRoot != filepath.Dir(r.Config) {
		t.Fatal("Cursor selected project binding differs")
	}
	if b.Materialization != domain.MaterializationMaterialized || b.Activation != domain.ActivationPrepared || b.Authentication != domain.AuthenticationNotChecked || b.Policy != domain.PolicyAllowed || b.Verification != domain.VerificationInstalled || !reflect.DeepEqual(b.AffectedSurfaces, []string{"cursor"}) || b.NativeActivationAttempt != "" || b.PendingNativeIntent != nil || !b.SelectedDelivery.IsZero() || b.InstallIntent != "" || b.ProfileAuthority != nil || b.ProfileNamespace != "" || b.LocalEntryObservation != nil {
		t.Fatal("Cursor conservative installed state was upgraded")
	}
	revision := &domain.ClientPackageRevision{Version: r.Version, TreeDigest: r.Digest, ManifestDigest: "sha256:" + clineAttemptSHA(r.Authored["plugin.json"])}
	server, _ := json.Marshal(c.server(r))
	nativeDigest := "sha256:" + clineAttemptSHA(append([]byte("agentplugins-native-mcp-v1\x00mcpServers\x00uapcursora\x00"), server...))
	objects := []domain.NativeObjectOwnership{{ObjectID: "package:cursor:" + r.Data.PhysicalBackend, Kind: "managed_package_directory", LogicalName: cursorCycleName, Path: r.Active, ManagedDigest: c.payloadDigest(t, c.projected(r)), ProtectionClass: "managed"}, {ObjectID: "cursor_project_mcp_server:uapcursora", Kind: "cursor_project_mcp_server", LogicalName: "uapcursora", Path: r.Config, ManagedDigest: nativeDigest, ProtectionClass: "managed"}}
	if !reflect.DeepEqual(b.PackageRevision, revision) || !reflect.DeepEqual(b.NativeObjects, objects) {
		t.Fatal("Cursor expected native ownership differs")
	}
	return b
}

func cursorCycleReceipt(r *cursorCycleRecipe, row domain.MutationReceipt, group string, sequence int, kind, before, after string) error {
	if len(group) != 35 || !strings.HasPrefix(group, "op-") {
		return fmt.Errorf("Cursor operation group differs")
	}
	for _, b := range group[3:] {
		if !strings.ContainsRune("0123456789abcdef", b) {
			return fmt.Errorf("Cursor operation group differs")
		}
	}
	op := group + "-001"
	base := filepath.Dir(r.Active)
	want := domain.MutationReceipt{OperationID: op, OperationGroupID: group, Sequence: sequence, MutationType: kind, ClientBindingID: r.Binding.ClientBindingID, ActivePath: r.Active, BackupPath: filepath.Join(base, ".agentplugins-backup-"+clineAttemptSHA([]byte(op))[:16]), BeforeDigest: before, AfterDigest: after, Phase: "committed"}
	if kind == "directory_swap" {
		want.StagingPath = filepath.Join(base, ".agentplugins-staging-"+clineAttemptSHA([]byte(op))[:16])
	}
	if !reflect.DeepEqual(row, want) {
		return fmt.Errorf("Cursor exact receipt history differs")
	}
	return nil
}

func (c *cursorMCPProjectCycle) swapProof(t *testing.T, r *cursorCycleRecipe, i domain.Installation, b domain.ClientBinding, mutated bool) {
	t.Helper()
	old := r.Binding
	if !mutated {
		if !reflect.DeepEqual(old, b) {
			t.Fatal("Cursor no-op changed its binding")
		}
		return
	}
	if len(b.Receipts) != len(old.Receipts)+1 || !reflect.DeepEqual(b.Receipts[:len(old.Receipts)], old.Receipts) && len(old.Receipts) != 0 {
		t.Fatal("Cursor receipt prefix changed")
	}
	before := ""
	if len(old.NativeObjects) != 0 {
		before = old.NativeObjects[0].ManagedDigest
	}
	r.Binding = b
	if err := cursorCycleReceipt(r, b.Receipts[len(b.Receipts)-1], i.OperationGroupID, len(b.Receipts), "directory_swap", before, b.NativeObjects[0].ManagedDigest); err != nil {
		t.Fatal(err)
	}
}

func (c *cursorMCPProjectCycle) installedProof(t *testing.T, r *cursorCycleRecipe, stage string, mutated bool) {
	t.Helper()
	state, raw := c.readState(t, r)
	i := state.Installations[0]
	first := r.Installation.InstallationID == ""
	c.currentData(t, r, i)
	b := c.binding(t, r, i)
	c.swapProof(t, r, i, b, mutated)
	if len(state.TransactionReceipts) != 0 {
		t.Fatal("Cursor live package has state-level removal receipts")
	}
	r.Installation, r.Binding = i, b
	payload, config := c.proofPayload(t, r), c.profile(t, r)
	data := c.dataProof(t, r, first)
	if err := c.acceptFile(c.statePath(r), raw, 0600); err != nil {
		t.Fatal(err)
	}
	c.proof(t, stage, map[string]any{"scope": r.Scope, "version": r.Version, "nonce": r.Nonce, "revision": r.Revision, "source_digest": r.Digest, "removed": false, "state": c.stream(t, raw), "native_config": c.stream(t, config), "payload": payload, "data": data})
}

func (c *cursorMCPProjectCycle) removedProof(t *testing.T, r *cursorCycleRecipe, stage string) {
	t.Helper()
	state, raw := c.readState(t, r)
	i := state.Installations[0]
	c.currentData(t, r, i)
	if !i.DataRetained || len(i.Clients) != 0 || len(state.TransactionReceipts) != 1 || cursorCycleReceipt(r, state.TransactionReceipts[0], i.OperationGroupID, len(r.Binding.Receipts)+1, "directory_remove", r.Binding.NativeObjects[0].ManagedDigest, "") != nil {
		t.Fatal("Cursor exact removal/retained-data state differs")
	}
	if _, err := os.Lstat(r.Active); !os.IsNotExist(err) {
		t.Fatal("Cursor removed package still present")
	}
	c.forget(r.Active)
	c.f.absent = append(c.f.absent, r.Active)
	r.Removed, r.Installation = true, i
	config, data := c.profile(t, r), c.dataProof(t, r, false)
	if err := c.acceptFile(c.statePath(r), raw, 0600); err != nil {
		t.Fatal(err)
	}
	c.proof(t, stage, map[string]any{"scope": r.Scope, "version": r.Version, "nonce": r.Nonce, "revision": r.Revision, "source_digest": r.Digest, "removed": true, "state": c.stream(t, raw), "native_config": c.stream(t, config), "data": data})
}
