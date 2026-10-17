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

// A saved dependency cannot take new command rules through an update, so it
// is not told to run one: its own reason carries no update next action, and
// it is not an update requirement.
func TestProjectionPinChangeIsNotAnUpdateRequirement(t *testing.T) {
	t.Parallel()
	wrapped := wrapLifecycleError(usecase.ErrProjectionPinChangeRequired)
	if errors.Is(wrapped, ErrUpdateRequired) || errors.Is(wrapped, usecase.ErrProjectionRefreshRequired) {
		t.Fatalf("wrapped error = %v", wrapped)
	}
	result, err := (&Engine{}).finishMutatingPackage(Result{Operation: OpUpdate}, usecase.AddResult{}, false, wrapped)
	attachNextActions(&result)
	if !errors.Is(err, usecase.ErrProjectionPinChangeRequired) || result.Outcome != OutcomeConflict || result.Reason != "pin_change_required" || len(result.NextActions) != 0 {
		t.Fatalf("result = %+v, %v", result, err)
	}
}
