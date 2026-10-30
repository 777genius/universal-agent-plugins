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
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func windCycleState(raw []byte) (domain.StateFileV2, error) {
	var state domain.StateFileV2
	if len(raw) > 64<<10 {
		return state, fmt.Errorf("Windsurf cycle state bound")
	}
	if _, err := projectNativeRuntimeJSON(raw); err != nil {
		return state, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&state) != nil || state.SchemaVersion != domain.StateSchemaVersion || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
		return state, fmt.Errorf("Windsurf cycle closed state")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return state, fmt.Errorf("Windsurf cycle trailing state")
	}
	return state, nil
}

func (c *windProjectCycle) binding(r *windComponentRecipe, i domain.Installation) (domain.ClientBinding, error) {
	var b domain.ClientBinding
	if len(i.Clients) != 1 || len(i.DataReceipts) != 1 {
		return b, fmt.Errorf("Windsurf cycle binding membership")
	}
	for id, row := range i.Clients {
		b = row
		if id != row.ClientBindingID {
			return b, fmt.Errorf("Windsurf binding map key")
		}
	}
	physical := windComponentName + "-" + windsurfConfigDigest([]byte(i.InstallationID))[:12]
	active := filepath.Join(r.StateRoot, "managed", "clients", "windsurf", physical)
	id := "client_" + windsurfConfigDigest([]byte(strings.Join([]string{i.InstallationID, "windsurf", r.scopeName(), active}, "\x00")))[:24]
	profile, activation := "", domain.ActivationActive
	if r.Scope != "user" {
		profile, activation = r.Profile, domain.ActivationPrepared
	}
	revision := &domain.ClientPackageRevision{Version: windCycleVersion(r), TreeDigest: r.Digest, ManifestDigest: "sha256:" + windsurfConfigDigest(r.Authored["plugin.json"])}
	if b.InstallIntent != "" || b.ClientID != "windsurf" || b.Scope != r.scopeName() || b.ClientBindingID != id || b.PhysicalArtifact != physical || b.TargetLocator != active || b.NativeProfileRoot != profile || b.Materialization != domain.MaterializationMaterialized || b.Activation != activation || b.Verification != domain.VerificationInstalled || b.Authentication != domain.AuthenticationNotChecked || b.Policy != domain.PolicyAllowed || !reflect.DeepEqual(b.PackageRevision, revision) || !b.SelectedDelivery.IsZero() || b.ProfileAuthority != nil || b.ProfileNamespace != "" || b.PendingNativeIntent != nil || b.LocalEntryObservation != nil || b.NativeActivationAttempt != "" || !reflect.DeepEqual(b.AffectedSurfaces, []string{"windsurf"}) {
		return b, fmt.Errorf("Windsurf cycle exact binding")
	}
	r.Active = active
	for scope, other := range c.w.recipes {
		if scope != r.Scope && other.Installation.InstallationID != "" && (other.Installation.InstallationID == i.InstallationID || other.Binding.ClientBindingID == id || other.Binding.PhysicalArtifact == physical || other.Binding.DataReceiptID == b.DataReceiptID) {
			return b, fmt.Errorf("Windsurf cycle scopes share authority")
		}
	}
	return b, nil
}

func windCycleReceipt(r *windComponentRecipe, row domain.MutationReceipt, group, kind, before, after string, sequence int) error {
	if len(group) != 35 || !strings.HasPrefix(group, "op-") {
		return fmt.Errorf("Windsurf cycle operation group")
	}
	for _, b := range group[3:] {
		if !strings.ContainsRune("0123456789abcdef", b) {
			return fmt.Errorf("Windsurf cycle operation hex")
		}
	}
	op := group + "-001"
	base := filepath.Dir(r.Active)
	want := domain.MutationReceipt{OperationID: op, OperationGroupID: group, Sequence: sequence, MutationType: kind, ClientBindingID: r.Binding.ClientBindingID, ActivePath: r.Active, BackupPath: filepath.Join(base, ".agentplugins-backup-"+clineAttemptSHA([]byte(op))[:16]), BeforeDigest: before, AfterDigest: after, Phase: "committed"}
	if kind == "directory_swap" {
		want.StagingPath = filepath.Join(base, ".agentplugins-staging-"+clineAttemptSHA([]byte(op))[:16])
	}
	return windCycleRequired(reflect.DeepEqual(row, want), "exact committed receipt")
}

func (c *windProjectCycle) publicProof(t *testing.T, label string, r *windComponentRecipe, result ports.CommandResult, start, end time.Time) {
	t.Helper()
	w := c.w
	progressLabel := label
	if label == "add-user" {
		progressLabel = "add-a"
	}
	if label != "repeat-remove-a" && string(result.Stderr) != grokCycleProgress(progressLabel, windComponentName) {
		t.Fatal("Windsurf exact public progress")
	}
	raw, err := projectNativeRuntimeRead(filepath.Join(r.StateRoot, "state-v2.json"), 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	state, err := windCycleState(raw)
	if err != nil {
		t.Fatal(err)
	}
	i := state.Installations[0]
	oldI, oldB := r.Installation, r.Binding
	var markerStart time.Time
	removed := label == "remove-a" || label == "remove-b" || label == "repeat-remove-a"
	if oldI.InstallationID != "" && i.InstallationID != oldI.InstallationID {
		t.Fatal("Windsurf held installation changed")
	}
	if removed {
		if !reflect.DeepEqual(i.Source, oldI.Source) || !reflect.DeepEqual(i.Package, oldI.Package) || len(i.Clients) != 0 || !i.DataRetained || !reflect.DeepEqual(i.DataReceipts, oldI.DataReceipts) || len(state.TransactionReceipts) != 1 {
			t.Fatal("Windsurf removal retained state")
		}
		if err := windCycleReceipt(r, state.TransactionReceipts[0], i.OperationGroupID, "directory_remove", oldB.Receipts[len(oldB.Receipts)-1].AfterDigest, "", len(oldB.Receipts)+1); err != nil {
			t.Fatal(err)
		}
		if label == "repeat-remove-a" && !reflect.DeepEqual(i, oldI) {
			t.Fatal("Windsurf repeated removal changed state")
		}
		c.removed[r.Scope] = true
	} else {
		if err := windComponentSource(i, r); err != nil {
			t.Fatal(err)
		}
		if len(state.TransactionReceipts) != 0 {
			t.Fatal("Windsurf installed state has removal receipts")
		}
		b, err := c.binding(r, i)
		if err != nil {
			t.Fatal(err)
		}
		data, err := windComponentData(r, i, b)
		if err != nil {
			t.Fatal(err)
		}
		r.Data = data.Locator
		markerStart = time.Now()
		entry := c.entry(r)
		digest, err := w.payloadProof(r, entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := windComponentObjects(r, b, digest, entry); err != nil {
			t.Fatal(err)
		}
		if windCycleMutated(label) {
			if len(b.Receipts) != len(oldB.Receipts)+1 || len(oldB.Receipts) > 0 && !reflect.DeepEqual(b.Receipts[:len(oldB.Receipts)], oldB.Receipts) {
				t.Fatal("Windsurf receipt prefix changed")
			}
			before := ""
			if len(oldB.Receipts) > 0 {
				before = oldB.Receipts[len(oldB.Receipts)-1].AfterDigest
			}
			r.Binding = b
			if err := windCycleReceipt(r, b.Receipts[len(b.Receipts)-1], i.OperationGroupID, "directory_swap", before, digest, len(b.Receipts)); err != nil {
				t.Fatal(err)
			}
		} else if !reflect.DeepEqual(i, oldI) || !reflect.DeepEqual(b, oldB) {
			t.Fatal("Windsurf no-op changed installation")
		}
		r.Binding = b
	}
	frames, err := w.proofFrames(r)
	if err != nil {
		t.Fatal(err)
	}
	cache, err := c.cacheProof(label, r, frames, result.Stdout, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.frames(label, r, frames, start, end); err != nil {
		t.Fatal(err)
	}
	if !removed && oldI.InstallationID == "" {
		c.marker(t, r, frames, markerStart)
	}
	if err := w.proofStable(r, frames, raw); err != nil {
		t.Fatal(err)
	}
	r.Installation = i
	if label != "repeat-remove-a" {
		if err := c.publicDTO(label, r, result.Stdout); err != nil {
			t.Fatal(err)
		}
	}
	if label == "repeat-remove-a" && !bytes.Equal(raw, mustWindCycleState(t, w.trees[r.StateRoot]["state-v2.json"], filepath.Join(r.StateRoot, "state-v2.json"))) {
		t.Fatal("Windsurf repeated removal state bytes")
	}
	config, err := projectNativeRuntimeRead(r.Config, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	c.proof(t, label, map[string]any{"scope": r.Scope, "version": windCycleVersion(r), "revision": r.Revision, "nonce": r.Nonce, "source": r.Source, "source_digest": r.Digest, "removed": removed, "state": windCycleStream(raw), "native_config": windCycleStream(config), "cache": cache, "data_marker": windCycleStream(c.markers[r.Scope]), "frames": frames, "mutation_lock": windCycleStream(mustWindCycleLock(t, r)), "data_owner": windCycleStream(mustWindCycleOwner(t, r)), "payload_files": c.payloadFiles(t, r, removed)})
	w.freezeProof(t, r)
}

func mustWindCycleState(t *testing.T, want windProfileRecord, path string) []byte {
	t.Helper()
	raw, err := projectNativeRuntimeRead(path, 64<<10)
	if err != nil || want.Size != int64(len(raw)) || want.SHA256 != clineAttemptSHA(raw) {
		t.Fatal("Windsurf held raw state changed")
	}
	return raw
}

func mustWindCycleLock(t *testing.T, r *windComponentRecipe) []byte {
	t.Helper()
	raw, err := projectNativeRuntimeRead(filepath.Join(r.StateRoot, "mutation.lock"), 4096)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustWindCycleOwner(t *testing.T, r *windComponentRecipe) []byte {
	t.Helper()
	raw, err := projectNativeRuntimeRead(filepath.Join(r.Data, ".agentplugins-data-owner.json"), 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (c *windProjectCycle) payloadFiles(t *testing.T, r *windComponentRecipe, removed bool) map[string]clineCycleBytes {
	t.Helper()
	files := map[string]clineCycleBytes{}
	if removed {
		return files
	}
	for _, name := range []string{"plugin.json", "mcp.json", "skills/" + windComponentName + "/SKILL.md", "io.github.777genius.agentplugins/managed-stdio-v1/metadata.json"} {
		raw, err := projectNativeRuntimeRead(filepath.Join(r.Active, name), 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = windCycleStream(raw)
	}
	return files
}
