package sourceacquisition

import (
	"errors"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// ErrGitCleanupUnconfirmed tells hosted callers not to admit another process
// after supervision returned without proving either no launch or full cleanup.
// Existing command diagnostics and errors.Is causes remain unchanged.
var ErrGitCleanupUnconfirmed = errors.New("git process cleanup is unconfirmed")

type gitCleanupUnconfirmedError struct{ cause error }

func (err *gitCleanupUnconfirmedError) Error() string { return err.cause.Error() }
func (err *gitCleanupUnconfirmedError) Unwrap() []error {
	return []error{err.cause, ErrGitCleanupUnconfirmed}
}

func gitCleanupEvidence(result legacyports.CommandResult, cause error) error {
	if result.NoChildStarted || result.CleanupComplete {
		return cause
	}
	if cause == nil {
		cause = ErrGitCleanupUnconfirmed
	}
	return &gitCleanupUnconfirmedError{cause: cause}
}
