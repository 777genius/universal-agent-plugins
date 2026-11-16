package shared

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// NativeProbeFailure keeps uncertain process closure distinct from a closed
// native query that merely failed or could not establish readiness.
func NativeProbeFailure(result legacyports.CommandResult, err error) error {
	err = CompletionProbeFailure(err)
	if err == nil || result.NoChildStarted || result.CleanupComplete || domain.IsUnclosedClientProbe(err) {
		return err
	}
	return &domain.UnclosedClientProbe{Operation: "native client query", Err: err}
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
