//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func windComponentState(raw []byte) (domain.StateFileV2, error) {
	var state domain.StateFileV2
	if len(raw) > 64<<10 {
		return state, fmt.Errorf("component state bound")
	}
	if _, err := projectNativeRuntimeJSON(raw); err != nil {
		return state, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&state) != nil || state.SchemaVersion != domain.StateSchemaVersion {
		return state, fmt.Errorf("component current state schema")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return state, fmt.Errorf("component state trailing")
	}
	if len(state.Installations) != 1 || len(state.TransactionReceipts) != 0 || len(state.EnvironmentReceipts) != 0 {
		return state, fmt.Errorf("component closed single installation")
	}
	return state, nil
}
func windComponentSource(i domain.Installation, r *windComponentRecipe) error {
	source := domain.SourceBinding{SourceBindingID: "src_" + windsurfConfigDigest([]byte(r.Source + "\x00\x00"))[:24], RequestedSource: r.Source, CanonicalSource: r.Source, TreeDigest: r.Digest}
	inventory := domain.ComponentInventory{Skills: []string{windComponentName}, MCPPresent: true, MCPEnabled: true, MCPServers: []string{windComponentName}}
	pkg := domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: windComponentName, Version: "1.0.0", ManifestDigest: "sha256:" + windsurfConfigDigest(r.Authored["plugin.json"]), Inventory: inventory}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(i.InstallationID) || i.DeclaredName != windComponentName || !reflect.DeepEqual(i.Source, source) || !reflect.DeepEqual(i.Package, pkg) || i.DataRetained || i.NeedsRebind || i.Directory != nil || i.LocalChatGPTMapping != nil || len(i.InstallPreferences) != 0 || i.OriginMode != domain.OriginModeDirect {
		return fmt.Errorf("component exact source/package authority")
	}
	return nil
}
func (w *windComponentFixture) publicProof(t *testing.T, r *windComponentRecipe, stdout []byte) {
	t.Helper()
	if windComponentPublicJSON(stdout) != nil {
		t.Fatal("component public installer JSON")
	}
	statePath := filepath.Join(r.StateRoot, "state-v2.json")
	raw, err := projectNativeRuntimeRead(statePath, 64<<10)
	if err != nil {
		t.Fatal("component state read")
	}
	proof, err := w.validateProof(r, raw)
	if err != nil {
		t.Fatal("component independent public authority proof", err)
	}
	w.proofs = append(w.proofs, proof)
	// All native/config/payload/data facts were checked independently before this
	// exact selected mutation closure becomes a new immutable baseline.
	w.freezeProof(t, r)
}
func (w *windComponentFixture) validateProof(r *windComponentRecipe, raw []byte) (map[string]any, error) {
	state, err := windComponentState(raw)
	if err != nil {
		return nil, err
	}
	i := state.Installations[0]
	if err = windComponentSource(i, r); err != nil {
		return nil, err
	}
	if len(i.Clients) != 1 {
		return nil, fmt.Errorf("component one binding")
	}
	var b domain.ClientBinding
	for id, candidate := range i.Clients {
		b = candidate
		if id != b.ClientBindingID {
			return nil, fmt.Errorf("component binding map key")
		}
	}
	physical := windComponentName + "-" + windsurfConfigDigest([]byte(i.InstallationID))[:12]
	active := filepath.Join(r.StateRoot, "managed", "clients", "windsurf", physical)
	bindingID := "client_" + windsurfConfigDigest([]byte(strings.Join([]string{i.InstallationID, "windsurf", r.scopeName(), active}, "\x00")))[:24]
	profile, activation := "", domain.ActivationActive
	if r.Scope != "user" {
		profile = r.Profile
		activation = domain.ActivationPrepared
	}
	revision := &domain.ClientPackageRevision{Version: "1.0.0", TreeDigest: r.Digest, ManifestDigest: "sha256:" + windsurfConfigDigest(r.Authored["plugin.json"])}
	if b.InstallIntent != domain.InstallIntentAutomatic || b.ClientID != "windsurf" || b.Scope != r.scopeName() || b.ClientBindingID != bindingID || b.PhysicalArtifact != physical || b.TargetLocator != active || b.NativeProfileRoot != profile || b.Materialization != domain.MaterializationMaterialized || b.Activation != activation || b.Verification != domain.VerificationInstalled || b.Authentication != domain.AuthenticationNotChecked || b.Policy != domain.PolicyAllowed || !reflect.DeepEqual(b.PackageRevision, revision) || !b.SelectedDelivery.IsZero() || b.ProfileAuthority != nil || b.ProfileNamespace != "" || b.PendingNativeIntent != nil || b.LocalEntryObservation != nil || b.NativeActivationAttempt != "" || !reflect.DeepEqual(b.AffectedSurfaces, []string{"windsurf"}) {
		return nil, fmt.Errorf("component exact binding identity/status")
	}
	for scope, other := range w.recipes {
		if scope != r.Scope && other.Installation.InstallationID != "" && (other.Installation.InstallationID == i.InstallationID || other.Binding.ClientBindingID == b.ClientBindingID || other.Binding.PhysicalArtifact == physical || other.Binding.DataReceiptID == b.DataReceiptID) {
			return nil, fmt.Errorf("component same-name scopes alias authority")
		}
	}
	frames, err := w.proofFrames(r)
	if err != nil {
		return nil, err
	}
	r.Active = active
	data, err := windComponentData(r, i, b)
	if err != nil {
		return nil, err
	}
	r.Data = data.Locator
	entry, err := w.nativeProof(r)
	if err != nil {
		return nil, err
	}
	digest, err := w.payloadProof(r, entry)
	if err != nil {
		return nil, err
	}
	if err = windComponentObjects(r, b, digest, entry); err != nil {
		return nil, err
	}
	if err = windComponentReceipts(i, b, digest); err != nil {
		return nil, err
	}
	if err = w.scopeClosure(r); err != nil {
		return nil, err
	}
	if err = w.projectClosure(r); err != nil {
		return nil, err
	}
	if err = w.proofStable(r, frames, raw); err != nil {
		return nil, err
	}
	r.Installation, r.Binding = i, b
	return map[string]any{"scope": r.Scope, "source": r.Source, "revision": r.Revision, "nonce": r.Nonce, "source_digest": r.Digest, "state": bytes.Clone(raw), "state_bytes": len(raw), "state_sha256": windsurfConfigDigest(raw), "binding": b, "native_entry": entry, "data": data, "payload_digest": digest, "user_skill_activated": false}, nil
}
func windComponentData(r *windComponentRecipe, i domain.Installation, b domain.ClientBinding) (domain.DataReceipt, error) {
	id := "data_" + windsurfConfigDigest([]byte(strings.Join([]string{i.InstallationID, b.PhysicalArtifact, r.scopeName()}, "\x00")))[:24]
	locator := filepath.Join(r.StateRoot, "plugin-data", b.PhysicalArtifact)
	d := i.DataReceipts[id]
	if len(i.DataReceipts) != 1 || b.DataReceiptID != id || d.DataReceiptID != id || d.PhysicalBackend != b.PhysicalArtifact || d.Scope != r.scopeName() || d.Locator != locator || d.State != domain.DataReceiptOwned || d.OwnershipDigest != "sha256:"+windsurfConfigDigest([]byte("agentplugins-plugin-data-v1\x00"+id+"\x00"+locator)) {
		return domain.DataReceipt{}, fmt.Errorf("component exact owned data receipt")
	}
	marker, err := projectNativeRuntimeRead(filepath.Join(locator, ".agentplugins-data-owner.json"), 64<<10)
	if err != nil {
		return domain.DataReceipt{}, err
	}
	if _, err = projectNativeRuntimeJSON(marker); err != nil {
		return domain.DataReceipt{}, err
	}
	var actual domain.DataReceipt
	decoder := json.NewDecoder(bytes.NewReader(marker))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&actual) != nil || !reflect.DeepEqual(actual, d) {
		return domain.DataReceipt{}, fmt.Errorf("component data marker differs")
	}
	return d, nil
}
func windComponentReceipts(i domain.Installation, b domain.ClientBinding, digest string) error {
	if len(b.Receipts) != 1 || i.OperationGroupID == "" {
		return fmt.Errorf("component exact committed receipt count")
	}
	r := b.Receipts[0]
	if r.OperationID == "" || r.OperationGroupID != i.OperationGroupID || r.Sequence != 1 || r.MutationType != "directory_swap" || r.ClientBindingID != b.ClientBindingID || r.ActivePath != b.TargetLocator || r.BeforeDigest != "" || r.AfterDigest != digest || r.Phase != "committed" || r.EnvironmentAuthority != nil || r.DataReceiptID != "" || len(r.ProfileOwners) != 0 || len(r.DirectoryProof) != 0 {
		return fmt.Errorf("component committed receipt identity")
	}
	base := filepath.Dir(b.TargetLocator)
	for _, path := range []string{r.StagingPath, r.BackupPath} {
		rel, err := filepath.Rel(base, path)
		if err != nil || path == "" || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || filepath.Clean(path) != path {
			return fmt.Errorf("component receipt path authority")
		}
	}
	return nil
}
