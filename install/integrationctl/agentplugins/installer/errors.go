package installer

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

var (
	ErrInvalidConfig            = errors.New("installer config rejected")
	ErrUnsupported              = errors.New("installer operation is not published in this beta")
	ErrInvalidHandle            = errors.New("prepared operation does not belong to this engine")
	ErrHandleClosed             = errors.New("prepared operation is closed")
	ErrHandleBusy               = errors.New("prepared operation is applying")
	ErrAlreadyApplied           = errors.New("prepared operation already reached a terminal apply")
	ErrCancelled                = errors.New("installer apply canceled")
	ErrRecoveryRequired         = errors.New("installer recovery required")
	ErrPlanChanged              = errors.New("installer recovery plan changed")
	ErrInvalidRequest           = errors.New("installer request rejected")
	ErrAmbiguousInstallations   = errors.New("ambiguous installations")
	ErrIncomplete               = errors.New("required components missing from plan")
	ErrUpdateRequired           = errors.New("install cannot change an active revision; use update")
	ErrNotInstalled             = errors.New("update and repair require an existing owned binding")
	ErrAssessmentRejected       = errors.New("package assessment is not allow")
	ErrCompatibilityUnavailable = errors.New("sibling compatibility checks are unavailable")
	ErrTargetFactsUnavailable   = errors.New("target facts are unavailable or conflict with owned state")
	ErrMigrationRequired        = statev2.ErrMigrationRequired
)

// MigrationRequiredError reports the readable on-disk schema and required
// writer schema. Use errors.As to read its fields and errors.Is with
// ErrMigrationRequired to classify it through installer error wrappers.
type MigrationRequiredError = statev2.MigrationRequiredError

// lifecycleConflictReason names the conflict a lifecycle error reports instead
// of an incomplete outcome. A saved dependency waiting for a pin change has
// its own reason, which carries no update next action: an update cannot help.
func lifecycleConflictReason(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrPlanChanged):
		return "plan_changed", true
	case errors.Is(err, ErrUpdateRequired):
		return "update_required", true
	case errors.Is(err, ErrTargetFactsUnavailable):
		return "target_facts_unavailable", true
	case errors.Is(err, usecase.ErrProjectionPinChangeRequired):
		return "pin_change_required", true
	}
	return "", false
}
