package installer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// A host that keeps an installed tree after a command rules change has to
// run an update, so the engine reports the conflict the way it reports every
// other update requirement, and the original error stays recognizable.
func TestProjectionRefreshRequiredIsAnUpdateConflict(t *testing.T) {
	t.Parallel()
	wrapped := wrapLifecycleError(fmt.Errorf("target kiro: %w", usecase.ErrProjectionRefreshRequired))
	if !errors.Is(wrapped, ErrUpdateRequired) || !errors.Is(wrapped, usecase.ErrProjectionRefreshRequired) {
		t.Fatalf("wrapped error = %v", wrapped)
	}
	result, err := (&Engine{}).finishMutatingPackage(Result{Operation: OpInstall}, usecase.AddResult{}, false, wrapped)
	if !errors.Is(err, usecase.ErrProjectionRefreshRequired) || result.Outcome != OutcomeConflict || result.Reason != "update_required" {
		t.Fatalf("result = %+v, %v", result, err)
	}
}
