package domain

import "errors"

type Code string

const (
	ErrUsage              Code = "usage"
	ErrSourceResolve      Code = "source_resolve"
	ErrManifestLoad       Code = "manifest_load"
	ErrUnsupportedTarget  Code = "unsupported_target"
	ErrEnvironmentBlocked Code = "environment_blocked"
	ErrStateConflict      Code = "state_conflict"
	ErrLockAcquire        Code = "lock_acquire"
	ErrActivationPending  Code = "activation_pending"
	ErrAuthPending        Code = "auth_pending"
	ErrMutationApply      Code = "mutation_apply"
	ErrRepairApply        Code = "repair_apply"
	ErrEvidenceViolation  Code = "evidence_violation"
)

type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error { return e.Cause }

func NewError(code Code, msg string, cause error) error {
	return &Error{Code: code, Message: msg, Cause: cause}
}

func ExitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	var de *Error
	if errors.As(err, &de) {
		switch de.Code {
		case ErrUsage:
			return 2
		case ErrSourceResolve:
			return 3
		case ErrManifestLoad:
			return 4
		case ErrUnsupportedTarget:
			return 5
		case ErrEnvironmentBlocked:
			return 6
		case ErrStateConflict:
			return 7
		case ErrLockAcquire:
			return 8
		case ErrActivationPending, ErrAuthPending:
			return 9
		case ErrMutationApply, ErrRepairApply:
			return 10
		case ErrEvidenceViolation:
			return 11
		}
	}
	return 1
}
