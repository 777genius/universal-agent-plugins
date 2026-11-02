package management

import (
	"errors"
	"time"
)

// reconcileObservations links observations only after the backend proves a
// freshly reviewed recovery. Public plans and journal absence are not proof.
func (s *Server) reconcileObservations(plan *planEntry, currentID string, outcome Outcome) error {
	if plan.request.Action == Recover && outcome.RecoverySettled {
		records, err := s.store.list()
		if err != nil {
			return err
		}
		for _, previous := range records {
			if previous.Operation.OperationId == currentID || previous.Operation.LocalSkill != nil || previous.IntentRoot != s.opts.IntentRoot || !pendingOperation(previous.Operation) {
				continue
			}
			if err = s.resolveObservation(previous, currentID); err != nil {
				return err
			}
		}
	}
	for _, previousID := range outcome.ReconciledOperationIDs {
		if plan.localSkill == nil || plan.request.OperationId != previousID || previousID == currentID {
			return errors.New("unexpected local reconciliation identity")
		}
		previous, err := s.store.byOperation(previousID)
		if err != nil {
			return err
		}
		if previous == nil || previous.Operation.Action != LocalSkill || previous.Operation.LocalSkill == nil || *previous.Operation.LocalSkill != *plan.localSkill {
			return errors.New("local reconciliation belongs to a different Skill")
		}
		if err = s.resolveObservation(*previous, currentID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) resolveObservation(previous acceptance, currentID string) error {
	previous.Operation.ResolvedBy = currentID
	// Keep the original result. Recovery resolves uncertainty without pretending
	// that the previous operation succeeded. Retain this link for at least 30 days.
	previous.Operation.UpdatedAt = time.Now().UTC()
	return s.store.save(previous)
}
