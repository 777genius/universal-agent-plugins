package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
	attachNextActions(&result, err)
	if !errors.Is(err, usecase.ErrProjectionPinChangeRequired) || result.Outcome != OutcomeConflict || result.Reason != "pin_change_required" || len(result.NextActions) != 0 {
		t.Fatalf("result = %+v, %v", result, err)
	}
}

func TestRetainedRevisionConflictPreservesTypeWithoutUpdateAdvice(t *testing.T) {
	original := errors.New("original lifecycle cause")
	marked := &domain.TypedError{Text: "retained Directory revision cannot change; updating an empty installation is not supported",
		Kind: domain.ErrRetainedRevisionChange, Cause: original}
	wrapped := wrapLifecycleError(marked)
	if !errors.Is(wrapped, ErrUpdateRequired) || !errors.Is(wrapped, domain.ErrRetainedRevisionChange) || !errors.Is(wrapped, original) || wrapped.Error() != marked.Error() {
		t.Fatalf("retained diagnostic lost its type, cause or original wording: %v", wrapped)
	}
	result, err := (&Engine{}).finishMutatingPackage(Result{Operation: OpInstall}, usecase.AddResult{}, false, wrapped)
	attachNextActions(&result, err)
	if !errors.Is(err, ErrUpdateRequired) || result.Outcome != OutcomeConflict || result.Reason != "update_required" || len(result.NextActions) != 0 || result.Mutated {
		t.Fatalf("retained conflict contract changed: %+v %v", result, err)
	}
}

func TestRetainedRevisionNextActionsPreserveOtherPriorities(t *testing.T) {
	marked := wrapLifecycleError(domain.ErrRetainedRevisionChange)
	for _, test := range []struct {
		name, reason, action string
		outcome              Outcome
		cause                error
	}{
		{"ordinary update", "update_required", "update", OutcomeConflict, ErrUpdateRequired},
		{"recovery with retained conflict", "pending", "recover", OutcomeRecovery, errors.Join(ErrRecoveryRequired, marked)},
		{"changed plan with retained conflict", "plan_changed", "reprepare", OutcomeConflict, errors.Join(ErrPlanChanged, marked)},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := Result{Outcome: test.outcome, Reason: test.reason}
			attachNextActions(&result, test.cause)
			if len(result.NextActions) != 1 || result.NextActions[0].Kind != test.action || result.Reason != test.reason {
				t.Fatalf("specific retained guidance overrode another action: %+v", result)
			}
		})
	}
}

func TestRecordedDigestRefusalMarksOnlyEmptyDirectoryRecords(t *testing.T) {
	for _, test := range []struct {
		name, mode string
		marked     bool
	}{
		{"retained data", "retained", true},
		{"purged preferences record", "purged", true},
		{"active binding", "active", false},
		{"absent binding remains recorded", "absent", false},
		{"direct source", "direct", false},
		{"missing Directory identity", "missing-directory", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := retainedDigestRefusalState(test.mode)
			before := retainedDigestStateJSON(t, state)
			engine := &Engine{store: groupReportingStore{state: state}}
			err := engine.refuseRecordedDigestRewrite(state.Installations[0].InstallationID, "sha256:next")
			if !errors.Is(err, ErrUpdateRequired) || errors.Is(err, domain.ErrRetainedRevisionChange) != test.marked {
				t.Fatalf("explicit-IID digest refusal classification: %v", err)
			}
			if test.marked && strings.Contains(err.Error(), "use update") || !test.marked && !strings.Contains(err.Error(), "use update") {
				t.Fatalf("wrong update advice for %s: %v", test.mode, err)
			}
			if before != retainedDigestStateJSON(t, state) {
				t.Fatal("digest refusal mutated stored facts")
			}
		})
	}
}

func TestRecordedDigestWithoutConflictDoesNotMarkRetainedState(t *testing.T) {
	state := retainedDigestRefusalState("retained")
	engine := &Engine{store: groupReportingStore{state: state}}
	for _, test := range []struct{ id, digest string }{
		{state.Installations[0].InstallationID, "sha256:recorded"},
		{state.Installations[0].InstallationID, ""},
		{"", "sha256:next"},
		{"00000000-0000-4000-8000-000000000002", "sha256:next"},
	} {
		if err := engine.refuseRecordedDigestRewrite(test.id, test.digest); err != nil {
			t.Fatalf("unchanged/missing pin became a retained conflict: %v", err)
		}
	}
}

func retainedDigestStateJSON(t *testing.T, state domain.StateFileV2) string {
	t.Helper()
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func retainedDigestRefusalState(mode string) domain.StateFileV2 {
	installation := domain.Installation{InstallationID: "00000000-0000-4000-8000-000000000001",
		OriginMode: domain.OriginModeDirectory, Directory: &domain.DirectoryOrigin{ProductID: "demo", DistributionID: "owner/demo", DesiredReleaseSequence: 1},
		Source: domain.SourceBinding{TreeDigest: "sha256:recorded"}, DataRetained: mode != "purged"}
	switch mode {
	case "active", "absent":
		materialization := domain.MaterializationMaterialized
		if mode == "absent" {
			materialization = domain.MaterializationAbsent
		}
		installation.Clients = map[string]domain.ClientBinding{"owned": {Materialization: materialization}}
	case "direct":
		installation.OriginMode, installation.Directory = domain.OriginModeDirect, nil
	case "missing-directory":
		installation.Directory = nil
	case "purged":
		installation.InstallPreferences = []domain.InstallPreference{{ClientID: domain.ClientKiro, Scope: domain.ScopeUser, InstallIntent: domain.InstallIntentPrepare}}
	}
	return domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}
}
