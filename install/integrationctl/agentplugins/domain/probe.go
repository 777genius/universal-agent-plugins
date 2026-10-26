package domain

import (
	"errors"
	"fmt"
	"time"
)

// IsolatedProbeTimeout records a read-only client probe whose own budget expired
// after the runner proved that the process and its descendants were closed.
// It preserves the deadline cause without canceling unrelated client activation.
type IsolatedProbeTimeout struct {
	Operation string
	Budget    time.Duration
	Elapsed   time.Duration
	Err       error
}

func (err *IsolatedProbeTimeout) Error() string {
	return fmt.Sprintf("%s exceeded %s probe budget after %s; process cleanup confirmed: %v", err.Operation, err.Budget, err.Elapsed.Round(time.Millisecond), err.Err)
}
func (err *IsolatedProbeTimeout) Unwrap() error { return err.Err }

// IsIsolatedProbeTimeout rejects joins with additional causes: a neighboring
// cleanup or persistence failure must not be hidden by a safe local timeout.
func IsIsolatedProbeTimeout(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*IsolatedProbeTimeout); ok {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		return len(causes) == 1 && IsIsolatedProbeTimeout(causes[0])
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return IsIsolatedProbeTimeout(wrapped.Unwrap())
	}
	return false
}

// UnclosedClientProbe preserves a runner failure whose process closure was not
// proved. This includes non-context containment, close, and reap failures.
type UnclosedClientProbe struct {
	Operation string
	Err       error
}

func (err *UnclosedClientProbe) Error() string {
	return fmt.Sprintf("%s process cleanup unconfirmed: %v", err.Operation, err.Err)
}
func (err *UnclosedClientProbe) Unwrap() error { return err.Err }
func IsUnclosedClientProbe(err error) bool {
	var unsafe *UnclosedClientProbe
	return errors.As(err, &unsafe)
}
