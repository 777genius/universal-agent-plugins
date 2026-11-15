package transaction

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type publicationMovingStore struct {
	StateStore
	phase          string
	move           func()
	moved          bool
	loadsAfterMove int
}

func TestPublicationParentStateSaveAuthorizesProjectBeforeForeignAuthority(t *testing.T) {
	kernel, mutation, store := standaloneFixture(t)
	if _, err := kernel.ApplyDirectory(t.Context(), mutation); err != nil {
		t.Fatal(err)
	}
	foreign, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	replacePublicationAncestor(t, filepath.Dir(store.Path))
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(project, ".uap"), 0o700); err != nil {
		t.Fatal(err)
	}
	selected := statev2.Store{ProjectRoot: project, Path: filepath.Join(project, ".uap", "state-v2.json")}
	if err := selected.Save(domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(selected.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := selected.Save(foreign); err == nil || errors.Is(err, directoryidentity.ErrChanged) || !strings.Contains(err.Error(), "outside selected namespace") {
		t.Fatalf("foreign physical authority reached before selected Project scope: %v", err)
	}
	after, err := os.ReadFile(selected.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("foreign scope changed selected state: %v", err)
	}
}

func (s *publicationMovingStore) Save(state domain.StateFileV2) error {
	if !s.moved && len(state.TransactionReceipts) == 1 && state.TransactionReceipts[0].Phase == s.phase {
		s.move()
		s.moved = true
	}
	return s.StateStore.Save(state)
}

func (s *publicationMovingStore) Load() (domain.StateFileV2, error) {
	if s.moved {
		s.loadsAfterMove++
	}
	return s.StateStore.Load()
}

func TestPublicationParentStateSaveRejectsLateAncestorReplacement(t *testing.T) {
	for _, test := range []struct {
		name, phase string
		recovery    bool
	}{
		{"initial", ReceiptPhaseStateCommitted, false}, {"final", ReceiptPhaseCommitted, false},
		{"recovery-durability", ReceiptPhaseStateCommitted, true}, {"recovery-final", ReceiptPhaseCommitted, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			phase := test.phase
			kernel, mutation, store := standaloneFixture(t)
			run := func() error { _, err := kernel.ApplyDirectory(t.Context(), mutation); return err }
			if test.recovery {
				ctx, cancel := context.WithCancel(t.Context())
				observed := &publicationObservedStore{StateStore: store}
				if phase == ReceiptPhaseStateCommitted {
					observed.cancelCommit = cancel
				} else {
					observed.failFinal = true
				}
				kernel.StateStore = observed
				_, err := kernel.ApplyDirectory(ctx, mutation)
				cancel()
				if err == nil {
					t.Fatal("recovery fixture did not interrupt")
				}
				run = func() error { return kernel.Recover(t.Context()) }
			}
			journal := filepath.Join(kernel.Directory.JournalDir, mutation.OperationID+".json")
			var originalState, originalJournal []byte
			var originalInfo os.FileInfo
			moving := &publicationMovingStore{StateStore: store, phase: phase}
			moving.move = func() {
				var err error
				originalState, err = os.ReadFile(store.Path)
				if err != nil {
					t.Fatal(err)
				}
				file, err := os.Open(store.Path)
				if err != nil {
					t.Fatal(err)
				}
				originalInfo, err = file.Stat()
				if err := errors.Join(err, file.Close()); err != nil {
					t.Fatal(err)
				}
				originalJournal, err = os.ReadFile(journal)
				if phase == ReceiptPhaseStateCommitted && err != nil {
					t.Fatal(err)
				}
				if phase == ReceiptPhaseCommitted && !os.IsNotExist(err) {
					t.Fatalf("committed journal not removed: %v", err)
				}
				replacePublicationAncestor(t, filepath.Dir(store.Path))
			}
			kernel.StateStore = moving
			if err := run(); !errors.Is(err, directoryidentity.ErrChanged) || !moving.moved {
				t.Fatalf("state save did not refuse lost original ancestry: %v", err)
			}
			after, err := os.ReadFile(store.Path)
			info, statErr := os.Stat(store.Path)
			if err != nil || statErr != nil || !bytes.Equal(after, originalState) || !os.SameFile(originalInfo, info) || originalInfo.Mode() != info.Mode() || !originalInfo.ModTime().Equal(info.ModTime()) {
				t.Fatalf("late ancestry refusal rewrote original state: %v %v", err, statErr)
			}
			if moving.loadsAfterMove != 0 {
				t.Fatal("lost original ancestry triggered a state reload")
			}
			afterJournal, err := os.ReadFile(journal)
			if phase == ReceiptPhaseStateCommitted && (err != nil || !bytes.Equal(afterJournal, originalJournal)) {
				t.Fatalf("refusal changed activated journal: %v", err)
			}
			if phase == ReceiptPhaseCommitted && !os.IsNotExist(err) {
				t.Fatalf("refusal recreated removed journal: %v", err)
			}
			assertTransactionBody(t, mutation.ActivePath, "new")
		})
	}
}

type publicationDecisionStore struct {
	state        domain.StateFileV2
	loads, saves int
}

func (s *publicationDecisionStore) Load() (domain.StateFileV2, error) { s.loads++; return s.state, nil }
func (s *publicationDecisionStore) Save(state domain.StateFileV2) error {
	s.saves++
	s.state = state
	return nil
}

func TestPublicationParentStateDecisionChecksOriginalAuthorityWithoutManager(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "intact", true: "replaced"}[replace], func(t *testing.T) {
			kernel, mutation, store := standaloneFixture(t)
			if _, err := kernel.ApplyDirectory(t.Context(), mutation); err != nil {
				t.Fatal(err)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			if replace {
				replacePublicationAncestor(t, filepath.Dir(store.Path))
			}
			injected := &publicationDecisionStore{state: state}
			stateOnly := Kernel{StateStore: injected}
			disposition, err := stateOnly.PersistStateDecisionWithDisposition(state, state)
			if replace {
				if !errors.Is(err, directoryidentity.ErrChanged) || disposition != domain.StateDecisionUnknown || injected.saves != 0 || injected.loads != 0 {
					t.Fatalf("state-only stale authority reached store: disposition=%s saves=%d loads=%d err=%v", disposition, injected.saves, injected.loads, err)
				}
			} else if err != nil || disposition != domain.StateDecisionDesired || injected.saves != 1 || injected.loads != 0 {
				t.Fatalf("intact state-only decision refused: disposition=%s saves=%d loads=%d err=%v", disposition, injected.saves, injected.loads, err)
			}
			after, err := os.ReadFile(store.Path)
			if err != nil || !bytes.Equal(after, original) {
				t.Fatalf("injected decision changed concrete state: %v", err)
			}
		})
	}
}

type publicationRetryStore struct {
	StateStore
	saves  int
	reload bool
	move   func()
}

func (s *publicationRetryStore) Save(state domain.StateFileV2) error {
	s.saves++
	if err := s.StateStore.Save(state); err != nil {
		return err
	}
	if s.saves == 1 {
		s.reload = true
		return errors.New("inert ambiguous durable write")
	}
	return nil
}

func (s *publicationRetryStore) Load() (domain.StateFileV2, error) {
	state, err := s.StateStore.Load()
	if err == nil && s.reload {
		s.reload = false
		s.move()
	}
	return state, err
}

func TestPublicationParentDurabilityRetryRevalidatesDesiredAuthority(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "intact", true: "replaced"}[replace], func(t *testing.T) {
			kernel, mutation, store := standaloneFixture(t)
			var originalState, originalJournal []byte
			var originalInfo os.FileInfo
			journal := filepath.Join(kernel.Directory.JournalDir, mutation.OperationID+".json")
			retry := &publicationRetryStore{StateStore: store, move: func() {
				if !replace {
					return
				}
				var err error
				originalState, err = os.ReadFile(store.Path)
				if err != nil {
					t.Fatal(err)
				}
				originalJournal, err = os.ReadFile(journal)
				if err != nil {
					t.Fatal(err)
				}
				file, err := os.Open(store.Path)
				if err != nil {
					t.Fatal(err)
				}
				originalInfo, err = file.Stat()
				if err := errors.Join(err, file.Close()); err != nil {
					t.Fatal(err)
				}
				replacePublicationAncestor(t, filepath.Dir(store.Path))
			}}
			kernel.StateStore = retry
			_, err := kernel.ApplyDirectory(t.Context(), mutation)
			if !replace {
				if err != nil || retry.saves != 3 {
					t.Fatalf("intact durability retry failed: %v saves=%d", err, retry.saves)
				}
				state, err := store.Load()
				if err != nil || state.TransactionReceipts[0].Phase != ReceiptPhaseCommitted {
					t.Fatalf("intact decision not finalized: %v", err)
				}
				return
			}
			if !errors.Is(err, directoryidentity.ErrChanged) || retry.saves != 1 {
				t.Fatalf("retry reached stale injected writer: %v saves=%d", err, retry.saves)
			}
			after, err := os.ReadFile(store.Path)
			info, statErr := os.Stat(store.Path)
			if err != nil || statErr != nil || !bytes.Equal(after, originalState) || !os.SameFile(originalInfo, info) || originalInfo.Mode() != info.Mode() || !originalInfo.ModTime().Equal(info.ModTime()) {
				t.Fatalf("retry changed original visible decision: %v %v", err, statErr)
			}
			afterJournal, err := os.ReadFile(journal)
			if err != nil || !bytes.Equal(afterJournal, originalJournal) {
				t.Fatalf("retry changed original journal: %v", err)
			}
			assertTransactionBody(t, mutation.ActivePath, "new")
		})
	}
}
