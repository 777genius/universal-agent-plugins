package shared

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// NativeProbeFailure keeps uncertain process closure distinct from a closed
// native query that merely failed or could not establish readiness.
func NativeProbeFailure(result legacyports.CommandResult, err error) error {
	return CommandClosureFailure("native client query", result, err)
}

// CommandClosureFailure classifies a failed client command by the runner's
// closure evidence. The ordinary OS runner reports a failed group kill or reap
// as a plain error with CleanupComplete false, never as a typed closure error,
// so the result must be consulted before any follow-up command may start.
func CommandClosureFailure(operation string, result legacyports.CommandResult, err error) error {
	err = CompletionProbeFailure(err)
	if err == nil || result.NoChildStarted || result.CleanupComplete || domain.IsUnclosedClientProbe(err) {
		return err
	}
	return &domain.UnclosedClientProbe{Operation: operation, Err: err}
}

// CompletionProbeFailure uses the finite-exchange runner's typed closure proof;
// protocol or exit-status failures with confirmed cleanup remain ordinary errors.
func CompletionProbeFailure(err error) error {
	var unclosed *legacyports.ProcessClosureError
	if !errors.As(err, &unclosed) || domain.IsUnclosedClientProbe(err) {
		return err
	}
	return &domain.UnclosedClientProbe{Operation: "native client exchange", Err: err}
}
