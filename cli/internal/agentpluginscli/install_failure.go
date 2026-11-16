package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// failureKind is the one classification of installer failures. Review-time and
// apply-time presentations choose their own wording from it; neither re-derives
// the kind from sentinel errors.
type failureKind string

const (
	failureUnknown           failureKind = "FAILED"
	failureRecovery          failureKind = "RECOVERY_REQUIRED"
	failureNativeCleanup     failureKind = "NATIVE_PROCESS_UNCLOSED"
	failureSecurityBlocked   failureKind = "SECURITY_REVIEW_BLOCKED"
	failureForeignTarget     failureKind = "FOREIGN_DESTINATION"
	failureManagedChanged    failureKind = "MANAGED_SKILL_CHANGED"
	failureTargetUnavailable failureKind = "TARGET_UNAVAILABLE"
	failureTargetUnsupported failureKind = "TARGET_UNSUPPORTED"
	failureUpdateRequired    failureKind = "UPDATE_REQUIRED"
	failureEnvironmentChange failureKind = "ENVIRONMENT_CHANGED"
	failureTimeout           failureKind = "TIMEOUT"
	failureCanceled          failureKind = "CANCELED"
	failureSkipped           failureKind = "SKIPPED"
)

func failureKindOf(err error) failureKind {
	switch {
	case err == nil:
		return failureUnknown
	case domain.IsUnclosedClientProbe(err):
		return failureNativeCleanup
	case errors.Is(err, installer.ErrRecoveryRequired):
		return failureRecovery
	case errors.Is(err, installer.ErrAssessmentRejected):
		return failureSecurityBlocked
	case errors.Is(err, installer.ErrForeignSkillDestination), errors.Is(err, nativeconfig.ErrCollision), errors.Is(err, nativeconfig.ErrNotOwned):
		return failureForeignTarget
	case errors.Is(err, installer.ErrManagedSkillChanged):
		return failureManagedChanged
	case errors.Is(err, installer.ErrTargetFactsUnavailable), errors.Is(err, installer.ErrCompatibilityUnavailable):
		return failureTargetUnavailable
	case errors.Is(err, installer.ErrUnsupported), errors.Is(err, installer.ErrIncomplete):
		return failureTargetUnsupported
	case errors.Is(err, installer.ErrUpdateRequired):
		return failureUpdateRequired
	case errors.Is(err, installer.ErrPlanChanged), errors.Is(err, ErrSkillsPlanStale), errors.Is(err, environment.ErrSourceChange):
		return failureEnvironmentChange
	case errors.Is(err, context.DeadlineExceeded):
		return failureTimeout
	case errors.Is(err, context.Canceled), errors.Is(err, installer.ErrCancelled):
		return failureCanceled
	}
	return failureUnknown
}

// installFailure is what a person sees for one agent: a plain cause, one next
// step and whether retrying that agent can help. Detail is the full technical
// chain; it goes to the install log and never to the terminal.
type installFailure struct {
	Kind      failureKind
	Summary   string
	Next      string
	Retryable bool
	Detail    string
}

// Keep the typed cause available without exposing native diagnostics in the UI.
type installFailureWithCause struct {
	summary string
	cause   error
}

func (e installFailureWithCause) Error() string { return e.summary }
func (e installFailureWithCause) Unwrap() error { return e.cause }

// describeBindingFailure explains why one binding did not succeed. A binding
// that was never reached says only that, and stays retryable.
func describeBindingFailure(outcome environment.BindingOutcome) *installFailure {
	if outcome.Ok() {
		return nil
	}
	failure := installFailure{Kind: failureKindOf(outcome.Err), Detail: errorText(outcome.Err)}
	if outcome.Result != nil {
		if failure.Detail == "" {
			failure.Detail = outcome.Result.Reason
		}
		if failure.Kind == failureUnknown {
			failure.Kind = failureKindOfOutcome(outcome.Result.Outcome)
		}
	}
	cause := failure.Kind
	if outcome.Status == environment.BindingNotAttempted && failure.Kind != failureNativeCleanup {
		failure.Kind = failureSkipped
	}
	failure.Summary, failure.Next, failure.Retryable = applyFailureWording(failure.Kind)
	if failure.Kind == failureSkipped {
		// A skipped agent is only worth retrying when what stopped the run can be fixed by retrying.
		_, _, failure.Retryable = applyFailureWording(cause)
	}
	return &failure
}

// describeSyncFailure explains a failure that happened before any binding ran,
// such as a lock, readiness or input-guard refusal. Every binding shares it.
func describeSyncFailure(err error) *installFailure {
	failure := installFailure{Kind: failureKindOf(err), Detail: errorText(err)}
	failure.Summary, failure.Next, failure.Retryable = applyFailureWording(failure.Kind)
	return &failure
}

func failureKindOfOutcome(outcome installer.Outcome) failureKind {
	switch outcome {
	case installer.OutcomeRecovery:
		return failureRecovery
	case installer.OutcomeCancelled:
		return failureCanceled
	case installer.OutcomeConflict:
		return failureEnvironmentChange
	}
	return failureUnknown
}

func applyFailureWording(kind failureKind) (summary, next string, retryable bool) {
	switch kind {
	case failureNativeCleanup:
		return "Could not confirm that an agent process has finished.", "Confirm the earlier agent process has stopped before retrying.", false
	case failureRecovery:
		return "An earlier installation was interrupted and needs recovery first.", "Run `agentplugins doctor`, follow its steps, then try again.", false
	case failureSecurityBlocked:
		return "The automated security check blocked this item.", "Review the findings before installing it.", false
	case failureForeignTarget:
		return "Something UAP does not own already exists at the install location.", "Rename or remove it yourself, then retry. UAP never overwrites it.", true
	case failureManagedChanged:
		return "The installed copy was edited outside UAP.", "Keep or review your edits, then run `agentplugins repair`.", false
	case failureTargetUnavailable:
		return "This agent's configuration or version could not be verified.", "Open the agent once or fix its configuration, then retry.", true
	case failureTargetUnsupported:
		return "This agent cannot install everything this package needs.", "Choose another agent or another package.", false
	case failureUpdateRequired:
		return "It is already installed at a different version.", "Run `agentplugins update` for it, then try again.", false
	case failureEnvironmentChange:
		return "Installed state changed while installing.", "Run the command again.", true
	case failureTimeout:
		return "A step took too long and was stopped.", "Check network access and the agent, then retry.", true
	case failureCanceled:
		return "The installation was canceled.", "Retry when you are ready.", true
	case failureSkipped:
		return "Not attempted because an earlier problem stopped the installation.", "Retry this agent once the earlier problem is resolved.", true
	}
	return "The installation did not complete for this agent.", "Retry, or hand the technical log to an agent to diagnose.", true
}
