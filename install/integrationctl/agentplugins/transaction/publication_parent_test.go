package transaction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type publicationObservedStore struct {
	StateStore
	onLoad       func()
	beforeFinal  func()
	saved        domain.StateFileV2
	failFinal    bool
	cancelCommit context.CancelFunc
}

func (s *publicationObservedStore) Load() (domain.StateFileV2, error) {
	if s.onLoad != nil {
		hook := s.onLoad
		s.onLoad = nil
		hook()
	}
	return s.StateStore.Load()
}

func (s *publicationObservedStore) Save(state domain.StateFileV2) error {
	if s.beforeFinal != nil && len(state.TransactionReceipts) == 1 && state.TransactionReceipts[0].Phase == ReceiptPhaseCommitted {
		hook := s.beforeFinal
		s.beforeFinal = nil
		hook()
	}
	if s.failFinal && len(state.TransactionReceipts) == 1 && state.TransactionReceipts[0].Phase == ReceiptPhaseCommitted {
		s.failFinal = false
		return errors.New("inert final receipt interruption")
	}
	s.saved = state
	err := s.StateStore.Save(state)
	if err == nil && s.cancelCommit != nil && len(state.TransactionReceipts) == 1 && state.TransactionReceipts[0].Phase == ReceiptPhaseStateCommitted {
		s.cancelCommit()
		s.cancelCommit = nil
	}
	return err
}

func TestStandalonePublicationCopiesOriginalParentAcrossBoundaries(t *testing.T) {
	kernel, mutation, store := standaloneFixture(t)
	original := *mutation.PublicationParentAuthority
	foreign, err := profileauthority.Capture(t.Context(), filepath.Dir(mutation.OwnedBase))
	if err != nil {
		t.Fatal(err)
	}
	observed := &publicationObservedStore{StateStore: store, onLoad: func() { *mutation.PublicationParentAuthority = foreign }}
	kernel.StateStore = observed
	var journalParent *directoryidentity.Authority
	kernel.Directory.Fault = func(phase string) error {
		if phase == dirswap.PhaseActivated {
			receipt, err := kernel.Directory.Load(mutation.OperationID)
			if err != nil {
				return err
			}
			journalParent = receipt.PublicationParentAuthority
		}
		return nil
	}
	receipt, err := kernel.ApplyDirectory(t.Context(), mutation)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.PublicationParentAuthority == nil || !receipt.PublicationParentAuthority.Equal(original) || !samePublicationParent(receipt.PublicationParentAuthority, journalParent) {
		t.Fatal("state did not copy the actual original journal token")
	}
	*receipt.PublicationParentAuthority = foreign
	if !observed.saved.TransactionReceipts[0].PublicationParentAuthority.Equal(original) {
		t.Fatal("returned receipt aliased saved original authority")
	}
	loaded, err := store.Load()
	if err != nil || !loaded.TransactionReceipts[0].PublicationParentAuthority.Equal(original) {
		t.Fatalf("caller mutation changed durable token: %v", err)
	}
}

func TestStandalonePublicationRecoversAfterJournalRemoval(t *testing.T) {
	kernel, mutation, store := standaloneFixture(t)
	kernel.StateStore = &publicationObservedStore{StateStore: store, failFinal: true}
	if _, err := kernel.ApplyDirectory(t.Context(), mutation); err == nil {
		t.Fatal("final receipt interruption not reached")
	}
	if _, err := os.Stat(filepath.Join(kernel.Directory.JournalDir, mutation.OperationID+".json")); !os.IsNotExist(err) {
		t.Fatalf("journal was not removed: %v", err)
	}
	before, err := store.Load()
	if err != nil || before.TransactionReceipts[0].Phase != ReceiptPhaseStateCommitted {
		t.Fatalf("lost original state decision: %v", err)
	}
	if err := kernel.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, err := store.Load()
	if err != nil || after.TransactionReceipts[0].Phase != ReceiptPhaseCommitted || !domain.SameProfileAuthority(before.TransactionReceipts[0].PublicationParentAuthority, after.TransactionReceipts[0].PublicationParentAuthority) {
		t.Fatalf("finalization changed original token: %v", err)
	}
	assertTransactionBody(t, mutation.ActivePath, "new")
}

func TestPublicationParentGuardsTerminalStateSave(t *testing.T) {
	kernel, mutation, store := standaloneFixture(t)
	var unchanged func()
	calls := 0
	kernel.StateStore = &publicationObservedStore{StateStore: store, beforeFinal: func() {
		calls++
		state, err := store.Load()
		if err != nil || len(state.TransactionReceipts) != 1 || state.TransactionReceipts[0].Phase != ReceiptPhaseStateCommitted {
			t.Fatal("terminal fixture lost its durable state decision", err)
		}
		replacePublicationAncestor(t, filepath.Dir(store.Path))
		unchanged = preservePublicationState(t, store.Path)
	}}
	if _, err := kernel.ApplyDirectory(t.Context(), mutation); !errors.Is(err, directoryidentity.ErrChanged) || calls != 1 {
		t.Errorf("terminal state write did not refuse the replaced ancestor: %v (%d callbacks)", err, calls)
	}
	if unchanged == nil {
		t.Fatal("terminal state boundary was not reached")
	}
	unchanged()
	assertTransactionBody(t, mutation.ActivePath, "new")
}

func TestStandalonePublicationRejectsStrippedJournalTokenBeforeRollback(t *testing.T) {
	kernel, mutation, store := standaloneFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	kernel.StateStore = &publicationObservedStore{StateStore: store, cancelCommit: cancel}
	if _, err := kernel.ApplyDirectory(ctx, mutation); !errors.Is(err, context.Canceled) {
		t.Fatalf("commit interruption not reached: %v", err)
	}
	journalPath := filepath.Join(kernel.Directory.JournalDir, mutation.OperationID+".json")
	raw, err := kernel.Directory.Load(mutation.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if raw.Phase != dirswap.PhaseActivated || raw.PublicationParentAuthority == nil {
		t.Fatal("test did not retain the original activated journal")
	}
	raw.PublicationParentAuthority = nil
	body, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	kernel.Directory.Fault = nil
	if err := kernel.Recover(t.Context()); err == nil || !strings.Contains(err.Error(), "journal and durable state receipt disagree") {
		t.Fatalf("stripped journal was not refused before replay: %v", err)
	}
	for path, want := range map[string][]byte{journalPath: body, store.Path: stateBefore} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("refusal changed original object: %v", err)
		}
	}
	assertTransactionBody(t, mutation.ActivePath, "new")
}

type originalPublicationState struct {
	state domain.StateFileV2
	saves int
}

func (s *originalPublicationState) Load() (domain.StateFileV2, error) { return s.state, nil }
func (s *originalPublicationState) Save(domain.StateFileV2) error {
	s.saves++
	return errors.New("unexpected state write")
}

func TestPublicationParentGuardsConcreteAndInjectedStoresBeforeJournalRead(t *testing.T) {
	kernel, mutation, store := standaloneFixture(t)
	if _, err := kernel.ApplyDirectory(t.Context(), mutation); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	injected := &originalPublicationState{state: state}
	missing := Kernel{StateStore: injected}
	for _, run := range []func() error{func() error { return missing.PrevalidateRecovery(t.Context()) }, func() error { return missing.Recover(t.Context()) }} {
		if err := run(); err == nil || !strings.Contains(err.Error(), "journal dir is required") {
			t.Fatalf("missing manager admitted publication: %v", err)
		}
	}
	project := filepath.Dir(store.Path)
	replacePublicationAncestor(t, project)
	unchanged := preservePublicationState(t, store.Path)
	if _, err := store.Load(); !errors.Is(err, directoryidentity.ErrChanged) {
		t.Fatalf("concrete store admitted changed parent: %v", err)
	}
	if err := store.Save(state); !errors.Is(err, directoryidentity.ErrChanged) {
		t.Errorf("concrete store wrote beneath a replaced ancestor: %v", err)
	}
	unchanged()
	journalSentinel := filepath.Join(project, "not-a-journal-directory")
	if err := os.WriteFile(journalSentinel, []byte("never enumerate"), 0o600); err != nil {
		t.Fatal(err)
	}
	kernel.StateStore, kernel.Directory.JournalDir = injected, journalSentinel
	for _, run := range []func() error{func() error { return kernel.PrevalidateRecovery(t.Context()) }, func() error { return kernel.Recover(t.Context()) }, func() error { return kernel.PersistStateDecision(state, state) }} {
		if err := run(); !errors.Is(err, directoryidentity.ErrChanged) {
			t.Errorf("accessed journal or state before checking original ancestry: %v", err)
		}
	}
	if injected.saves != 0 {
		t.Fatal("ancestry refusal wrote state")
	}
	assertTransactionBody(t, mutation.ActivePath, "new")
}

func replacePublicationAncestor(t *testing.T, project string) {
	t.Helper()
	original := project + "-original"
	if err := os.Rename(project, original); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(original); err != nil {
			t.Error(err)
		}
	})
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	children, err := os.ReadDir(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range children {
		if err := os.Rename(filepath.Join(original, child.Name()), filepath.Join(project, child.Name())); err != nil {
			t.Fatal(err)
		}
	}
}

func preservePublicationState(t *testing.T, path string) func() {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		after, readErr := os.ReadFile(path)
		info, statErr := os.Stat(path)
		if readErr != nil || statErr != nil || !bytes.Equal(body, after) || !os.SameFile(before, info) || before.Mode() != info.Mode() || !before.ModTime().Equal(info.ModTime()) {
			t.Errorf("ancestry refusal changed original state bytes or identity: %v %v", readErr, statErr)
		}
	}
}
