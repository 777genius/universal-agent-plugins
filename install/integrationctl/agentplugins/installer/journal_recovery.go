package installer

import (
	"context"
	"errors"
	"reflect"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

// This fence records only the existing kernel's journal boundaries. It neither
// parses journals nor performs recovery. The kernel's next state write, next
// directory context check, or successful return records the preceding directory
// recovery's completion. Only that journal may disappear from the pinned scope.
type journalRecoveryFence struct {
	before    domain.StateFileV2
	digest    string
	open      map[string]bool
	completed map[string]bool
	active    string
}

func newJournalRecoveryFence(state domain.StateFileV2, observed RecoveryObservation) *journalRecoveryFence {
	f := &journalRecoveryFence{before: state, digest: observed.StateDigest, open: map[string]bool{}, completed: map[string]bool{}}
	for _, journal := range observed.Journals {
		f.open[journal.OperationID] = true
	}
	return f
}

// Never baseline from an observed foreign change. Construct the sole permitted
// observation from the approved scope, then compare its complete value. Pending
// receipt presence follows only these recorded journal removals.
func (s *nativeRecoveryScope) advanceJournals() error {
	if s.journals == nil {
		return nil
	}
	view, err := s.engine.observe()
	if err != nil {
		return errors.Join(ErrPlanChanged, err)
	}
	want := s.expected
	want.Journals = nil
	removed := map[string]bool{}
	kept := false
	for _, journal := range s.expected.Journals {
		if containsJournal(view.Recovery.Journals, journal) {
			want.Journals = append(want.Journals, journal)
			kept = true
			continue
		}
		// Rolled-back journals have no directory callback; the existing manager
		// only removes their already recorded terminal journal.
		if kept || (journal.OperationID != s.journals.active && journal.Phase != dirswap.PhaseRolledBack) {
			return ErrPlanChanged
		}
		removed[journal.OperationID] = true
	}
	want.Receipts = append([]PendingReceipt(nil), want.Receipts...)
	for i := range want.Receipts {
		if removed[want.Receipts[i].OperationID] {
			want.Receipts[i].JournalPresent = false
		}
	}
	want.Required = len(want.Journals)+len(want.Receipts)+len(want.NativeIntents) != 0
	if !reflect.DeepEqual(view.Recovery, want) {
		return ErrPlanChanged
	}
	for id := range removed {
		s.journals.completed[id] = true
		if id == s.journals.active {
			s.journals.active = ""
		}
	}
	s.expected = want
	return nil
}

// A context is the existing directory manager's pre-effect boundary. Fence both
// sides of the caller's Err method even when it returns cancellation or nil.
// An error never grants authority to overwrite its late state/journal changes.
type journalRecoveryContext struct {
	context.Context
	scope *nativeRecoveryScope
}

func (c journalRecoveryContext) Err() error {
	if err := c.scope.advanceJournals(); err != nil {
		return err
	}
	if err := c.scope.confirm(); err != nil {
		return err
	}
	err := c.Context.Err()
	if scopeErr := c.scope.confirm(); scopeErr != nil {
		return errors.Join(scopeErr, err)
	}
	if err == nil {
		for _, journal := range c.scope.expected.Journals {
			if journal.Phase != dirswap.PhaseRolledBack {
				c.scope.journals.active = journal.OperationID
				break
			}
		}
	}
	return err
}

// Normalize only exact predeclared state_committed receipts that the kernel
// has completed (or that already lacked a journal). All other persisted fields,
// including terminal receipts and native intent, must equal the initial state.
func (f *journalRecoveryFence) validateState(state domain.StateFileV2) error {
	state.TransactionReceipts = f.normalizeReceipts(state.TransactionReceipts, f.before.TransactionReceipts)
	state.Installations = slices.Clone(state.Installations)
	for i, installation := range state.Installations {
		bindings := make(map[string]domain.ClientBinding, len(installation.Clients))
		for key, binding := range installation.Clients {
			if i < len(f.before.Installations) {
				binding.Receipts = f.normalizeReceipts(binding.Receipts, f.before.Installations[i].Clients[key].Receipts)
			}
			bindings[key] = binding
		}
		state.Installations[i].Clients = bindings
	}
	digest, err := recoveryStateDigest(state)
	if err != nil {
		return err
	}
	if digest != f.digest {
		return ErrPlanChanged
	}
	return nil
}

func (f *journalRecoveryFence) normalizeReceipts(receipts, before []domain.MutationReceipt) []domain.MutationReceipt {
	out := slices.Clone(receipts)
	for i, receipt := range out {
		if i >= len(before) || before[i].Phase != transaction.ReceiptPhaseStateCommitted || receipt.Phase != transaction.ReceiptPhaseCommitted {
			continue
		}
		if f.open[receipt.OperationID] && !f.completed[receipt.OperationID] {
			continue
		}
		receipt.Phase = transaction.ReceiptPhaseStateCommitted
		if reflect.DeepEqual(receipt, before[i]) {
			out[i] = receipt
		}
	}
	return out
}
