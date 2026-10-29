//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func geminiCycleState(raw []byte) (domain.StateFileV2, error) {
	var state domain.StateFileV2
	if _, err := projectNativeRuntimeJSON(raw); err != nil {
		return state, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&state); err != nil {
		return state, err
	}
	if _, err := d.Token(); err != io.EOF {
		return state, fmt.Errorf("Gemini current state trailing value")
	}
	if state.SchemaVersion != domain.StateSchemaVersion {
		return state, fmt.Errorf("Gemini current state schema required")
	}
	return state, statev2.Validate(state)
}
func (c *geminiProjectCycle) proof(t *testing.T, r *geminiCycleRecipe, stage string) {
	t.Helper()
	c.proofWithAssessment(t, r, stage, nil)
}
func (c *geminiProjectCycle) proofWithAssessment(t *testing.T, r *geminiCycleRecipe, stage string, assessment *geminiCycleAssessment) {
	t.Helper()
	// Capture the full selected authorities before interpretation and freeze
	// these same maps only after a second complete snapshot proves stability.
	before := map[string]map[string]geminiCycleRecord{}
	global := c.recipes["user"].StateRoot
	for _, path := range []string{r.StateRoot, r.Profile, r.Project, global} {
		if _, exists := before[path]; exists {
			continue
		}
		tree, err := geminiCycleTree(c.f.ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = tree
	}
	configFrame, err := geminiCycleFile(c.f.ctx, r.Config)
	if err != nil || configFrame.Record != before[r.Profile]["settings.json"] || !geminiCycleFileParents(c.fileFrames[r.Config], configFrame) {
		t.Fatal("Gemini selected settings and original held authority differ")
	}
	if !c.open[r.Config] && c.fileFrames[r.Config].Record != configFrame.Record {
		t.Fatal("Gemini original settings changed outside selected public operation")
	}
	raw, err := projectNativeRuntimeRead(filepath.Join(r.StateRoot, "state-v2.json"), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	state, err := geminiCycleState(raw)
	if err != nil || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatal("Gemini single exact current installation", err)
	}
	i := state.Installations[0]
	previousBinding := r.Binding
	if err := geminiCycleSource(i, r); err != nil {
		t.Fatal(err)
	}
	if r.Installation.InstallationID != "" && i.InstallationID != r.Installation.InstallationID {
		t.Fatal("Gemini stable installation identity changed")
	}
	if r.Removed {
		c.removedProof(t, r, state)
	} else {
		c.bindingProof(t, r, i)
		want := len(previousBinding.Receipts)
		if stage == "add-user" || stage == "add-a" || stage == "add-b" || stage == "intact-repair-a" || stage == "update-a" || stage == "missing-repair-a" {
			want++
		}
		if len(r.Binding.Receipts) != want {
			t.Fatal("Gemini exact selected public committed-receipt delta")
		}
		if len(state.TransactionReceipts) != 0 {
			t.Fatal("Gemini unexpected standalone receipt")
		}
	}
	if err := c.skillProof(r, !r.Removed); err != nil {
		t.Fatal(err)
	}
	if prior, ok := c.profileBefore[r.Scope]; ok {
		if err := geminiCycleProfileClosure(prior, before[r.Profile], r); err != nil {
			t.Fatal(err)
		}
	}
	cache, err := geminiCycleAssessmentClosure(c.f.ctx, c.frozen[global], before[global], c.assessments, assessment)
	if err != nil {
		t.Fatal(err)
	}
	stateBefore, stateAfter := c.frozen[r.StateRoot], before[r.StateRoot]
	if global == r.StateRoot {
		stateBefore, stateAfter = cache.Before, cache.After
	} else if err := cache.globalUnchanged(assessment != nil); err != nil {
		t.Fatal(err)
	}
	if err := geminiCycleStateClosure(stateBefore, stateAfter, r); err != nil {
		t.Fatal(err)
	}
	if err := geminiCycleCWDClosure(c.frozen[r.Project], before[r.Project], r.Scope != "user"); err != nil {
		t.Fatal(err)
	}
	lock, hasLock := before[r.Profile]["settings.json.agentplugins.lock"]
	if hasLock && (lock.Mode != 0100000|0600 || lock.Size != 0 || lock.SHA256 != geminiCycleSHA(nil)) {
		t.Fatal("Gemini exact native writer lock")
	}
	for path, want := range before {
		after, err := geminiCycleTree(c.f.ctx, path)
		if err != nil || !reflect.DeepEqual(want, after) {
			t.Fatal("Gemini authority changed during independent public proof")
		}
	}
	if before[r.StateRoot]["state-v2.json"].SHA256 != geminiCycleSHA(raw) {
		t.Fatal("Gemini raw state and held state file differ")
	}
	for _, path := range []string{r.StateRoot, r.Project, global} {
		c.frozen[path] = before[path]
		delete(c.open, path)
	}
	c.assessments = cache.Ledger
	skills := filepath.Join(r.Profile, "skills")
	skillTree := map[string]geminiCycleRecord{}
	for name, record := range before[r.Profile] {
		if name == "skills" {
			skillTree["."] = record
		} else if len(name) > 7 && name[:7] == "skills/" {
			skillTree[name[7:]] = record
		}
	}
	c.frozen[skills] = skillTree
	delete(c.open, skills)
	c.f.protected[r.Config] = before[r.Profile]["settings.json"].SHA256
	c.fileFrames[r.Config] = configFrame
	delete(c.open, r.Config)
	c.f.protected[filepath.Join(r.StateRoot, "state-v2.json")] = geminiCycleSHA(raw)
	if hasLock {
		c.f.protected[r.Config+".agentplugins.lock"] = lock.SHA256
		parents := map[string]geminiCycleRecord{}
		for path, record := range configFrame.Parents {
			parents[path] = record
		}
		c.fileFrames[r.Config+".agentplugins.lock"] = geminiCycleFileFrame{Record: lock, Parents: parents}
	}
	r.Installation = i
	delete(c.profileBefore, r.Scope)
	c.append(t, "cycle-proof.jsonl", stage, map[string]any{"scope": r.Scope, "revision": r.Revision, "removed": r.Removed, "source_digest": r.Digest, "state": c.stream(t, raw), "binding": r.Binding})
	c.check(t)
}
func (c *geminiProjectCycle) bindingProof(t *testing.T, r *geminiCycleRecipe, i domain.Installation) {
	t.Helper()
	b, err := geminiCycleBindingIdentity(r, i)
	if err != nil {
		t.Fatal(err)
	}
	physical, active, id := b.PhysicalArtifact, b.TargetLocator, b.ClientBindingID
	if r.Binding.ClientBindingID != "" && (r.Binding.ClientBindingID != id || r.Binding.PhysicalArtifact != physical || r.Binding.DataReceiptID != b.DataReceiptID) {
		t.Fatal("Gemini update/repair replaced stable scope identities")
	}
	for scope, other := range c.recipes {
		if scope != r.Scope && other.Installation.InstallationID != "" && (other.Installation.InstallationID == i.InstallationID || other.Binding.ClientBindingID == id || other.Binding.PhysicalArtifact == physical || other.Data == filepath.Join(r.StateRoot, "plugin-data", physical)) {
			t.Fatal("Gemini same-name scopes share authority")
		}
	}
	d, err := c.dataProof(r, i, b.DataReceiptID, physical)
	if err != nil {
		t.Fatal(err)
	}
	r.Active, r.Data = active, d.Locator
	entry, err := c.configProof(r, true)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := c.payloadProof(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := geminiCycleObjects(r, b, payload, entry); err != nil {
		t.Fatal(err)
	}
	if err := geminiCycleReceipts(b, r.Binding, payload); err != nil {
		t.Fatal(err)
	}
	r.Binding = b
}
func geminiCycleBindingIdentity(r *geminiCycleRecipe, i domain.Installation) (domain.ClientBinding, error) {
	if len(i.Clients) != 1 || i.DataRetained {
		return domain.ClientBinding{}, fmt.Errorf("Gemini exact installed binding count")
	}
	physical := domain.ComputePhysicalArtifactID(geminiCycleName, i.InstallationID)
	active := filepath.Join(r.StateRoot, "managed", "clients", "gemini", physical)
	id := domain.ComputeClientBindingID(i.InstallationID, "gemini", r.scopeName(), active)
	b, ok := i.Clients[id]
	profile := r.Profile
	activation := domain.ActivationPrepared
	if r.Scope == "user" {
		activation = domain.ActivationActive
	}
	revision := &domain.ClientPackageRevision{Version: "1.0.0", TreeDigest: r.Digest, ManifestDigest: "sha256:" + geminiCycleSHA(r.Authored["plugin.json"])}
	if !ok || b.InstallIntent != domain.InstallIntentAutomatic || !b.SelectedDelivery.IsZero() || b.ProfileAuthority != nil || b.ProfileNamespace != "" || b.LocalEntryObservation != nil || b.PendingNativeIntent != nil || b.NativeActivationAttempt != "" || b.ClientBindingID != id || b.ClientID != "gemini" || b.Scope != r.scopeName() || b.NativeProfileRoot != profile || b.TargetLocator != active || b.PhysicalArtifact != physical || b.Materialization != domain.MaterializationMaterialized || b.Activation != activation || b.Verification != domain.VerificationInstalled || b.Authentication != domain.AuthenticationNotChecked || b.Policy != domain.PolicyAllowed || !reflect.DeepEqual(b.AffectedSurfaces, []string{"gemini"}) || !reflect.DeepEqual(b.PackageRevision, revision) {
		return b, fmt.Errorf("Gemini exact binding/materialization/revision authority")
	}
	return b, nil
}
func geminiCycleReceipts(b, previous domain.ClientBinding, payload string) error {
	if len(b.Receipts) == 0 || len(b.Receipts) > 16 || len(b.Receipts) < len(previous.Receipts) {
		return fmt.Errorf("Gemini exact committed receipt prefix")
	}
	seen := map[string]bool{}
	for index, receipt := range b.Receipts {
		if index < len(previous.Receipts) && !reflect.DeepEqual(receipt, previous.Receipts[index]) {
			return fmt.Errorf("Gemini past committed receipt changed")
		}
		base := filepath.Dir(b.TargetLocator)
		before := ""
		if index > 0 {
			before = b.Receipts[index-1].AfterDigest
		}
		if receipt.OperationID != receipt.OperationGroupID+"-001" || receipt.OperationGroupID == "" || receipt.Sequence != index+1 || receipt.ClientBindingID != b.ClientBindingID || receipt.ActivePath != b.TargetLocator || receipt.MutationType != "directory_swap" || receipt.Phase != "committed" || receipt.EnvironmentAuthority != nil || seen[receipt.OperationID] || receipt.BeforeDigest != before || filepath.Dir(receipt.StagingPath) != base || !strings.HasPrefix(filepath.Base(receipt.StagingPath), ".agentplugins-staging-") || receipt.BackupPath != filepath.Join(base, ".agentplugins-backup-"+geminiCycleSHA([]byte(receipt.OperationID))[:16]) {
			return fmt.Errorf("Gemini exact receipt identity/phase")
		}
		seen[receipt.OperationID] = true
	}
	if b.Receipts[len(b.Receipts)-1].AfterDigest != payload {
		return fmt.Errorf("Gemini latest receipt and complete payload differ")
	}
	return nil
}
func (c *geminiProjectCycle) removedProof(t *testing.T, r *geminiCycleRecipe, state domain.StateFileV2) {
	t.Helper()
	i := state.Installations[0]
	if len(i.Clients) != 0 || !i.DataRetained || r.Binding.ClientBindingID == "" {
		t.Fatal("Gemini exact data-retained removal")
	}
	if _, err := c.dataProof(r, i, r.Binding.DataReceiptID, r.Binding.PhysicalArtifact); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{r.Active, filepath.Dir(r.SkillPath)} {
		if err := geminiCycleAbsent(path); err != nil {
			t.Fatal(err)
		}
		c.absent[path] = true
	}
	if _, err := c.configProof(r, false); err != nil {
		t.Fatal(err)
	}
	if len(state.TransactionReceipts) != 1 {
		t.Fatal("Gemini exact final removal receipt count")
	}
	receipt := state.TransactionReceipts[0]
	if receipt.ClientBindingID != r.Binding.ClientBindingID || receipt.ActivePath != r.Active || receipt.MutationType != "directory_remove" || receipt.Phase != "committed" || receipt.AfterDigest != "" || receipt.OperationID != receipt.OperationGroupID+"-001" || receipt.OperationGroupID == "" || receipt.Sequence != len(r.Binding.Receipts)+1 || receipt.EnvironmentAuthority != nil || receipt.BeforeDigest != r.Binding.Receipts[len(r.Binding.Receipts)-1].AfterDigest || receipt.BackupPath != filepath.Join(filepath.Dir(r.Active), ".agentplugins-backup-"+geminiCycleSHA([]byte(receipt.OperationID))[:16]) {
		t.Fatal("Gemini exact retained standalone removal receipt")
	}
}
