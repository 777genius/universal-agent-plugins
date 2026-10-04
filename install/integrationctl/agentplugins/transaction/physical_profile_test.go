package transaction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type physicalPort struct{}

func (physicalPort) CaptureProfileAuthority(ctx context.Context, c domain.DetectedClient) (domain.ProfileAuthority, error) {
	return profileauthority.Capture(ctx, c.ConfigRoot)
}
func (physicalPort) RevalidateProfileAuthority(ctx context.Context, _ domain.ClientID, p domain.ProfileAuthority) error {
	return profileauthority.Revalidate(ctx, p)
}

type physicalCountingStore struct {
	StateStore
	saves int
}

func (s *physicalCountingStore) Save(state domain.StateFileV2) error {
	s.saves++
	return s.StateStore.Save(state)
}
func physicalTransaction(t *testing.T, id string) (Kernel, DirectoryMutation, string) {
	t.Helper()
	k, m, store := transactionFixture(t, id)
	profile := filepath.Join(t.TempDir(), "TEST-profile")
	if err := os.Mkdir(profile, 0700); err != nil {
		t.Fatal(err)
	}
	token, err := profileauthority.Capture(t.Context(), profile)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	b := state.Installations[0].Clients[m.ClientBindingID]
	k.Namespace = filepath.Dir(store.Path)
	k.Directory.Namespace = k.Namespace
	k.PhysicalAuthority = physicalPort{}
	b.ProfileAuthority = &token
	b.ProfileNamespace = k.Namespace
	b.NativeProfileRoot = profile
	state.Installations[0].Clients[m.ClientBindingID] = b
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	m.DesiredState = state
	return k, m, profile
}

// Regression: a later invalid open journal allows the first journal's state Save/replay.
func physicalWholeRecoveryBeforeSave(t *testing.T, fault string) {
	k, m, profile := physicalTransaction(t, "TEST-a")
	interrupted := errors.New("TEST interrupted")
	reached := 0
	k.Directory.Fault = func(phase string) error {
		if phase == dirswap.PhaseCommitPending {
			reached++
			return interrupted
		}
		return nil
	}
	_, err := k.ApplyDirectory(t.Context(), m)
	if reached != 1 || !errors.Is(err, interrupted) {
		t.Fatalf("first durable commit fixture did not reach fault: %d %v", reached, err)
	}
	firstJournal, err := k.Directory.Load(m.OperationID)
	if err != nil || firstJournal.Phase != dirswap.PhaseCommitPending {
		t.Fatalf("first commit-pending journal missing: phase=%s err=%v", firstJournal.Phase, err)
	}
	k.Directory.Fault = func(phase string) error {
		if phase == dirswap.PhaseActivated {
			reached++
			return interrupted
		}
		return nil
	}
	secondBase := filepath.Join(t.TempDir(), "managed")
	secondActive := filepath.Join(secondBase, "plugin")
	secondStaging := filepath.Join(secondBase, "staging")
	writeTransactionBody(t, secondActive, "old")
	writeTransactionBody(t, secondStaging, "new")
	p2 := filepath.Join(t.TempDir(), "TEST-second-profile")
	if err := os.Mkdir(p2, 0700); err != nil {
		t.Fatal(err)
	}
	token, err := profileauthority.Capture(t.Context(), p2)
	if err != nil {
		t.Fatal(err)
	}
	neutral, err := profileauthority.ToNeutral(token)
	if err != nil {
		t.Fatal(err)
	}
	_, err = k.Directory.Apply(t.Context(), dirswap.Input{OperationID: "TEST-z", ClientBindingID: "TEST-second-binding", Sequence: 1, OwnedBase: secondBase, ActivePath: secondActive, StagingPath: secondStaging, VerifyActive: verifyTransactionBody("old"), ProfileOwners: []dirswap.ProfileOwner{{Namespace: k.Namespace, InstallationID: m.InstallationID, ClientID: "vscode", ClientBindingID: "TEST-second-binding", Authority: &neutral}}})
	if reached != 2 || !errors.Is(err, interrupted) {
		t.Fatalf("second fixture did not reach intended fault: reached=%d err=%v", reached, err)
	}
	secondJournal, err := k.Directory.Load("TEST-z")
	if err != nil || secondJournal.Phase != dirswap.PhaseActivated {
		t.Fatalf("second activated journal missing: phase=%s err=%v", secondJournal.Phase, err)
	}
	switch fault {
	case "profile":
		if err := os.Rename(p2, p2+"-old"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(p2, 0700); err != nil {
			t.Fatal(err)
		}
	case "digest":
		before, err := os.Stat(secondJournal.BackupPath)
		if err != nil {
			t.Fatal(err)
		}
		writeTransactionBody(t, secondJournal.BackupPath, "TEST-corrupt")
		after, err := os.Stat(secondJournal.BackupPath)
		if err != nil || !os.SameFile(before, after) {
			t.Fatal("fixture replaced backup identity")
		}
	case "terminal", "unsupported":
		secondJournal.Phase = dirswap.PhaseCommitPending
		if fault == "unsupported" {
			secondJournal.Phase = "TEST-unsupported"
		}
		raw, err := json.Marshal(secondJournal)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(k.Directory.JournalDir, "TEST-z.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	secondRaw, err := os.ReadFile(filepath.Join(k.Directory.JournalDir, "TEST-z.json"))
	if err != nil {
		t.Fatal(err)
	}
	firstRaw, err := os.ReadFile(filepath.Join(k.Directory.JournalDir, "TEST-a.json"))
	if err != nil {
		t.Fatal(err)
	}
	stateBefore, err := k.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	counted := &physicalCountingStore{StateStore: k.StateStore}
	k.StateStore = counted
	k.Directory.Fault = nil
	if err := k.Recover(t.Context()); err == nil {
		t.Fatal("late invalid journal authorized an earlier replay")
	}
	after, err := os.ReadFile(filepath.Join(k.Directory.JournalDir, "TEST-a.json"))
	if err != nil || !bytes.Equal(firstRaw, after) || counted.saves != 0 {
		t.Fatal("whole-set refusal changed first journal or saved state")
	}
	secondAfter, err := os.ReadFile(filepath.Join(k.Directory.JournalDir, "TEST-z.json"))
	if err != nil || !bytes.Equal(secondRaw, secondAfter) {
		t.Fatal("whole-set refusal changed second journal")
	}
	stateAfter, err := k.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	beforeJSON, _ := marshalComparableState(stateBefore)
	afterJSON, _ := marshalComparableState(stateAfter)
	if !bytes.Equal(beforeJSON, afterJSON) {
		t.Fatal("refusal changed state")
	}
	if _, err := os.Stat(profile); err != nil {
		t.Fatal(err)
	}
}

func TestPhysicalProfileWholeRecoveryBeforeSave(t *testing.T) {
	for _, fault := range []string{"profile", "digest", "terminal", "unsupported"} {
		t.Run(fault, func(t *testing.T) { physicalWholeRecoveryBeforeSave(t, fault) })
	}
}

// Regression: terminal receipt loses owner when desired state deletes the binding;
// closed historical receipts must not become an eternal profile gate.
func physicalRemovalAfterBindingDeletion(t *testing.T, fault string) {
	k, m, profile := physicalTransaction(t, "TEST-remove")
	before, err := k.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	desired := before
	desired.Installations[0].Clients = map[string]domain.ClientBinding{}
	k.Directory.Fault = func(phase string) error {
		if phase == fault {
			return errors.New("TEST terminal interruption")
		}
		return nil
	}
	receipts, err := k.RemoveDirectoryGroup(t.Context(), DirectoryRemovalGroup{OperationGroupID: "TEST-group", DesiredState: desired, Removals: []DirectoryRemoval{{OperationID: m.OperationID, InstallationID: m.InstallationID, ClientBindingID: m.ClientBindingID, Sequence: 1, OwnedBase: m.OwnedBase, ActivePath: m.ActivePath, Verify: verifyTransactionBody("old")}}})
	if err == nil || len(receipts) != 1 {
		t.Fatalf("terminal boundary missing: %v", err)
	}
	state, err := k.StateStore.Load()
	if err != nil || len(state.Installations[0].Clients) != 0 || len(state.TransactionReceipts) != 1 || len(state.TransactionReceipts[0].ProfileOwners) != 1 || len(state.TransactionReceipts[0].DirectoryProof) == 0 {
		t.Fatal("removed owner scope was lost")
	}
	k.Directory.Fault = nil
	if err := k.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(profile, profile+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(profile, 0700); err != nil {
		t.Fatal(err)
	}
	if err := k.Recover(t.Context()); err != nil {
		t.Fatalf("closed history was treated as active authority: %v", err)
	}
}

func TestPhysicalProfileRemovalAfterBindingDeletion(t *testing.T) {
	for _, fault := range []string{dirswap.FaultCommitQuarantined, dirswap.FaultBackupRemoved} {
		t.Run(fault, func(t *testing.T) { physicalRemovalAfterBindingDeletion(t, fault) })
	}
}

// Regression: zero/historical-nil carrier remains usable; a recorded opted token
// without its verifier must refuse before the actual manager creates a journal.
func TestPhysicalProfileMissingVerifierBeforeIntent(t *testing.T) {
	k, m, store := transactionFixture(t, "TEST-missing-verifier")
	token, err := domain.NewProfileAuthority(domain.ProfileAuthorityFacts{Version: 1, CanonicalRoot: "/TEST-profile", Ancestry: []domain.ProfileAuthorityEntry{{CanonicalPath: "/", Scheme: "linux-fsuuid-inode-v1", VolumeID: "01", ObjectID: "1"}, {CanonicalPath: "/TEST-profile", Scheme: "linux-fsuuid-inode-v1", VolumeID: "01", ObjectID: "2"}}})
	if err != nil {
		t.Fatal(err)
	}
	b := m.DesiredState.Installations[0].Clients[m.ClientBindingID]
	b.ProfileAuthority = &token
	b.NativeProfileRoot = token.Facts().CanonicalRoot
	b.ProfileNamespace = "TEST-namespace"
	m.DesiredState.Installations[0].Clients[m.ClientBindingID] = b
	k.Namespace = "TEST-namespace"
	if err := store.Save(m.DesiredState); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.ApplyDirectory(t.Context(), m); err == nil || !strings.Contains(err.Error(), "verifier is required") {
		t.Fatalf("missing verifier did not refuse before intent: %v", err)
	}
	after, err := os.ReadFile(store.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("missing verifier changed actual state")
	}
	assertTransactionBody(t, m.ActivePath, "old")
	if _, err := os.Lstat(k.Directory.JournalDir); !os.IsNotExist(err) {
		t.Fatal("missing verifier created journal directory")
	}
	b.ProfileAuthority = nil
	b.ProfileNamespace = ""
	m.DesiredState.Installations[0].Clients[m.ClientBindingID] = b
	if err := store.Save(m.DesiredState); err != nil {
		t.Fatal(err)
	}
	if _, err := k.ApplyDirectory(t.Context(), m); err != nil {
		t.Fatalf("historical nil lost behavior: %v", err)
	}
}

// Regression: selecting a legacy binding drops another opted owner of the same data operation.
func TestPhysicalProfileSharedDataMissingVerifierBeforeIntent(t *testing.T) {
	k, m, store := transactionFixture(t, "TEST-shared-data")
	k.Namespace = filepath.Dir(store.Path)
	state := m.DesiredState
	first := state.Installations[0].Clients[m.ClientBindingID]
	data, _, err := (providers.PluginDataManager{Base: filepath.Join(t.TempDir(), "TEST-data")}).EnsureData(t.Context(), m.InstallationID, first.PhysicalArtifact, first.Scope)
	if err != nil {
		t.Fatal(err)
	}
	first.DataReceiptID = data.DataReceiptID
	state.Installations[0].DataReceipts = map[string]domain.DataReceipt{data.DataReceiptID: data}
	state.Installations[0].Clients[m.ClientBindingID] = first
	token, err := domain.NewProfileAuthority(domain.ProfileAuthorityFacts{Version: 1, CanonicalRoot: "/TEST-profile", Ancestry: []domain.ProfileAuthorityEntry{{CanonicalPath: "/", Scheme: "linux-fsuuid-inode-v1", VolumeID: "01", ObjectID: "1"}, {CanonicalPath: "/TEST-profile", Scheme: "linux-fsuuid-inode-v1", VolumeID: "01", ObjectID: "2"}}})
	if err != nil {
		t.Fatal(err)
	}
	peer := first
	peer.ClientID = "claude"
	peer.ClientBindingID = domain.ComputeClientBindingID(m.InstallationID, peer.ClientID, peer.Scope, peer.TargetLocator)
	peer.NativeProfileRoot = token.Facts().CanonicalRoot
	peer.ProfileNamespace, peer.ProfileAuthority = k.Namespace, &token
	state.Installations[0].Clients[peer.ClientBindingID] = peer
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	m.DesiredState = state
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.ApplyDirectory(t.Context(), m); err == nil || !strings.Contains(err.Error(), "verifier is required") {
		t.Fatalf("shared owner bypassed missing verifier: %v", err)
	}
	after, err := os.ReadFile(store.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("shared refusal changed state")
	}
	assertTransactionBody(t, m.ActivePath, "old")
	if _, err := os.Lstat(k.Directory.JournalDir); !os.IsNotExist(err) {
		t.Fatal("shared refusal created intent")
	}
}

// Regression: shared-data intent invents a binding or drops an owner through detach.
func TestPhysicalProfileSharedDataJournalScopes(t *testing.T) {
	k, m, _ := physicalTransaction(t, "TEST-shared-scopes")
	state, err := k.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	first := state.Installations[0].Clients[m.ClientBindingID]
	dataManager := providers.PluginDataManager{Base: filepath.Join(t.TempDir(), "TEST-data")}
	data, _, err := dataManager.EnsureData(t.Context(), m.InstallationID, first.PhysicalArtifact, first.Scope)
	if err != nil {
		t.Fatal(err)
	}
	first.DataReceiptID = data.DataReceiptID
	state.Installations[0].DataReceipts = map[string]domain.DataReceipt{data.DataReceiptID: data}
	state.Installations[0].Clients[m.ClientBindingID] = first
	profile := filepath.Join(t.TempDir(), "TEST-peer-profile")
	if err := os.Mkdir(profile, 0700); err != nil {
		t.Fatal(err)
	}
	token, err := profileauthority.Capture(t.Context(), profile)
	if err != nil {
		t.Fatal(err)
	}
	peer := first
	peer.ClientID, peer.NativeProfileRoot, peer.ProfileAuthority = "claude", profile, &token
	peer.ClientBindingID = domain.ComputeClientBindingID(m.InstallationID, peer.ClientID, peer.Scope, peer.TargetLocator)
	state.Installations[0].Clients[peer.ClientBindingID] = peer
	if err := k.StateStore.Save(state); err != nil {
		t.Fatal(err)
	}
	desired := state
	desired.Installations[0].Clients = map[string]domain.ClientBinding{}
	interrupted := errors.New("TEST terminal interruption")
	reached := false
	k.Directory.Fault = func(phase string) error {
		if phase == dirswap.FaultCommitQuarantined {
			reached = true
			return interrupted
		}
		return nil
	}
	_, err = k.RemoveDirectoryGroup(t.Context(), DirectoryRemovalGroup{OperationGroupID: "TEST-data-group", DesiredState: desired, Removals: []DirectoryRemoval{{OperationID: m.OperationID, InstallationID: m.InstallationID, ClientBindingID: first.DataReceiptID, DataReceiptID: first.DataReceiptID, Standalone: true, Sequence: 1, OwnedBase: filepath.Dir(data.Locator), ActivePath: data.Locator, BeforeDigest: data.OwnershipDigest, Verify: func(ctx context.Context, path string) error { return dataManager.ValidateDataAt(ctx, data, path) }}}})
	if !reached || !errors.Is(err, interrupted) {
		t.Fatalf("fixture did not reach intended fault: reached=%t err=%v", reached, err)
	}
	observed, err := k.Directory.Load(m.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Phase != dirswap.PhaseCommitPending || observed.SchemaVersion != 5 || observed.ClientBindingID != "" || observed.DataReceiptID != first.DataReceiptID || len(observed.ProfileOwners) != 2 {
		t.Fatal("shared operation identity lost exact scopes")
	}
	bindings := map[string]bool{}
	for _, o := range observed.ProfileOwners {
		if o.InstallationID != m.InstallationID || o.Namespace != k.Namespace {
			t.Fatal("invented scope")
		}
		bindings[o.ClientBindingID] = true
	}
	if !bindings[m.ClientBindingID] || !bindings[peer.ClientBindingID] {
		t.Fatal("dropped actual shared binding")
	}
	recorded, err := k.StateStore.Load()
	if err != nil || len(recorded.Installations[0].Clients) != 0 || len(recorded.TransactionReceipts) != 1 || len(recorded.TransactionReceipts[0].ProfileOwners) != 2 {
		t.Fatal("detach lost shared scopes")
	}
	receipt := recorded.TransactionReceipts[0]
	if receipt.ClientBindingID != "" || receipt.DataReceiptID != data.DataReceiptID || receipt.Phase != ReceiptPhaseStateCommitted || !sameReceiptOwners(receipt.ProfileOwners, observed.ProfileOwners) || !sameDirectoryProof(receipt.DirectoryProof, observed) {
		t.Fatal("terminal receipt lost actual data identity or owner proofs")
	}
	// Exercise the real Store boundary using the actual detached data receipt.
	for _, test := range []struct {
		name    string
		mutate  func(*domain.MutationReceipt)
		wantErr string
	}{
		{name: "state committed"},
		{name: "committed", mutate: func(r *domain.MutationReceipt) { r.Phase = ReceiptPhaseCommitted }},
		{name: "missing identities", mutate: func(r *domain.MutationReceipt) { r.DataReceiptID = "" }, wantErr: "incomplete"},
		{name: "unsafe data id", mutate: func(r *domain.MutationReceipt) { r.DataReceiptID = "../TEST-data" }, wantErr: "invalid data receipt id"},
		{name: "blank data id", mutate: func(r *domain.MutationReceipt) { r.DataReceiptID = " " }, wantErr: "invalid data receipt id"},
		{name: "missing owners", mutate: func(r *domain.MutationReceipt) { r.ProfileOwners = nil }, wantErr: "incomplete"},
		{name: "missing proof", mutate: func(r *domain.MutationReceipt) { r.DirectoryProof = nil }, wantErr: "incomplete"},
		{name: "missing sequence", mutate: func(r *domain.MutationReceipt) { r.Sequence = 0 }, wantErr: "incomplete"},
		{name: "missing mutation", mutate: func(r *domain.MutationReceipt) { r.MutationType = "" }, wantErr: "incomplete"},
		{name: "unrelated mutation", mutate: func(r *domain.MutationReceipt) { r.MutationType = "directory_swap" }, wantErr: "incomplete"},
		{name: "invalid phase", mutate: func(r *domain.MutationReceipt) { r.Phase = "tampered" }, wantErr: "invalid phase"},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := recorded
			r := receipt
			if test.mutate != nil {
				test.mutate(&r)
			}
			candidate.TransactionReceipts = []domain.MutationReceipt{r}
			before, err := marshalComparableState(recorded)
			if err != nil {
				t.Fatal(err)
			}
			err = k.StateStore.Save(candidate)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("Save error=%v; want %q", err, test.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			loaded, err := k.StateStore.Load()
			if err != nil {
				t.Fatal(err)
			}
			if test.wantErr != "" {
				after, err := marshalComparableState(loaded)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatal("refused receipt changed persisted state")
				}
			} else if got := loaded.TransactionReceipts[0]; got.ClientBindingID != "" || got.DataReceiptID != data.DataReceiptID || got.Phase != r.Phase || !sameReceiptOwners(got.ProfileOwners, observed.ProfileOwners) || !sameDirectoryProof(got.DirectoryProof, observed) {
				t.Fatal("Store roundtrip changed real shared data receipt")
			}
			if err := k.StateStore.Save(recorded); err != nil {
				t.Fatal(err)
			}
		})
	}
	raw, err := os.ReadFile(filepath.Join(k.Directory.JournalDir, m.OperationID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(profile, profile+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(profile, 0700); err != nil {
		t.Fatal(err)
	}
	k.Directory.Fault = nil
	if err := k.Recover(t.Context()); err == nil {
		t.Fatal("terminal shared owner drift authorized cleanup")
	}
	after, err := os.ReadFile(filepath.Join(k.Directory.JournalDir, m.OperationID+".json"))
	if err != nil || !bytes.Equal(raw, after) {
		t.Fatal("terminal refusal changed journal")
	}
}
