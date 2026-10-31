//go:build linux

package process

import "errors"

type ordinaryCleanupOriginFailure struct {
	cause  error
	reason string
}

func (failure *ordinaryCleanupOriginFailure) Error() string { return failure.cause.Error() }
func (failure *ordinaryCleanupOriginFailure) Unwrap() error { return failure.cause }

// Metadata never changes disappearance/unsupported decisions or replaces an
// already-owned specific origin with a broader contextual origin.
func withOrdinaryCleanupOrigin(reason string, cause error) error {
	if cause == nil || ordinarySupervisorCleanupReason(reason) == "" || ordinaryCleanupFailureReason(cause) != "" {
		return cause
	}
	if linuxProcessGone(cause) || errors.Is(cause, ErrOrdinaryContainmentUnsupported) {
		return cause
	}
	return &ordinaryCleanupOriginFailure{cause: cause, reason: reason}
}

func ordinaryCleanupFailureReason(err error) string {
	var failure *ordinaryCleanupOriginFailure
	if errors.As(err, &failure) && failure != nil {
		return ordinarySupervisorCleanupReason(failure.reason)
	}
	return ""
}

func setOrdinarySupervisorCleanupFailure(result *linuxOrdinarySupervisorResult, err error) {
	if err == nil {
		return
	}
	result.Error = err.Error()
	result.FailureStage = ordinarySupervisorDescendantCleanup
	result.FailureReason = ordinaryCleanupFailureReason(err)
}
