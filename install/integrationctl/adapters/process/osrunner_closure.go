package process

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Finite protocol callers need positive closure proof even when their exchange
// failed. Keep that lifecycle fact separate from command/protocol success.
type duplexClosureProof struct {
	started bool
	closed  bool
}

func (proof *duplexClosureProof) start(started bool) {
	if proof != nil {
		proof.started = started
	}
}

func (proof *duplexClosureProof) record(termination terminationResult, waitErr error, closed bool) {
	if proof != nil {
		proof.closed = processClosureProven(termination, waitErr, closed)
	}
}

func (proof *duplexClosureProof) attachFailure(err error) error {
	if proof != nil {
		var unclosed *ports.ProcessClosureError
		proof.closed = !errors.As(err, &unclosed)
	}
	return err
}

func (proof *duplexClosureProof) finish(err error) error {
	if proof == nil || !proof.started || proof.closed {
		return err
	}
	var unclosed *ports.ProcessClosureError
	if errors.As(err, &unclosed) {
		return err
	}
	if err == nil {
		err = fmt.Errorf("supervised process cleanup did not establish closure")
	}
	return &ports.ProcessClosureError{Cause: err}
}

func processClosureProven(termination terminationResult, waitErr error, closed bool) bool {
	_, reapedWithStatus := waitErr.(*exec.ExitError) //nolint:errorlint // Only the owned Wait's raw exit status proves reap.
	return termination.leaderStopped && termination.err == nil && closed && (waitErr == nil || reapedWithStatus)
}

func attachCleanupResult(attachErr error, termination terminationResult, waitErr error) error {
	err := errors.Join(attachErr, termination.err, waitErr)
	if processClosureProven(termination, waitErr, true) {
		return err
	}
	return &ports.ProcessClosureError{Cause: err}
}
