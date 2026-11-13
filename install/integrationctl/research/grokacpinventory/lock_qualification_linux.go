//go:build linux

package main

import "errors"

const lockMode = "lock-qualification-v1"

func validateResearchMode(r request) error {
	if r.Mode == lockMode {
		if r.Scope != "a" || r.Phase != "locked-writer" && r.Phase != "released-writer" {
			return errors.New("lock-phase-refused")
		}
		return nil
	}
	if r.Phase != "" {
		return errors.New("unexpected-phase-refused")
	}
	return validateSessionRequest(r)
}

func lockResultFields(mode, original string) string {
	if mode == lockMode {
		return original + " mode phase lock_held"
	}
	return original
}

func validateLockResult(got result, r request) error {
	if r.Mode != lockMode {
		if got.Phase != "" || got.LockHeld != nil {
			return errors.New("unexpected-lock-proof")
		}
		return nil
	}
	if got.Mode != lockMode || got.Phase != r.Phase || got.LockHeld == nil ||
		got.Complete && (*got.LockHeld != (r.Phase == "locked-writer") || len(got.Commands) != 4) {
		return errors.New("lock-proof-refused")
	}
	return nil
}
