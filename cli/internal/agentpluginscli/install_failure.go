package agentpluginscli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/fs"
	"net"
	"os"
	"os/exec"

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
	failureProcessCleanup    failureKind = "PROCESS_CLEANUP_UNCONFIRMED"
	failureRecovery          failureKind = "RECOVERY_REQUIRED"
	failureSecurityBlocked   failureKind = "SECURITY_REVIEW_BLOCKED"
	failureForeignTarget     failureKind = "FOREIGN_DESTINATION"
	failureManagedChanged    failureKind = "MANAGED_SKILL_CHANGED"
	failureTargetUnavailable failureKind = "TARGET_UNAVAILABLE"
	failureTargetUnsupported failureKind = "TARGET_UNSUPPORTED"
	failureUpdateRequired    failureKind = "UPDATE_REQUIRED"
	failureEnvironmentChange failureKind = "ENVIRONMENT_CHANGED"
	failureTimeout           failureKind = "TIMEOUT"
	failurePermission        failureKind = "PERMISSION_DENIED"
	failureDiskFull          failureKind = "DISK_FULL"
	failureReadOnly          failureKind = "READ_ONLY_LOCATION"
	failureNetwork           failureKind = "NETWORK_UNAVAILABLE"
	failureAgentMissing      failureKind = "AGENT_PROGRAM_MISSING"
	failureCanceled          failureKind = "CANCELED"
	failureSkipped           failureKind = "SKIPPED"
)

func failureKindOf(err error) failureKind {
	switch {
	case err == nil:
		return failureUnknown
	case domain.IsUnclosedClientProbe(err):
		return failureProcessCleanup
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
	return environmentFailureKind(err)
}

// environmentFailureKind recognises ordinary machine problems that a person can
// fix without reading technical detail. It only looks at typed causes that
// survive error wrapping, never at message text.
func environmentFailureKind(err error) failureKind {
	var (
		execErr *exec.Error
		opErr   *net.OpError
		dnsErr  *net.DNSError
	)
	switch {
	case isDiskFull(err):
		return failureDiskFull
	case isReadOnly(err):
		return failureReadOnly
	// A program that is missing or cannot be run is checked before the generic
	// permission case: an unexecutable binary is not a write problem.
	case errors.Is(err, exec.ErrNotFound), errors.As(err, &execErr):
		return failureAgentMissing
	case errors.As(err, &opErr), errors.As(err, &dnsErr):
		// Network-shaped failures are not automatically connectivity defects:
		// socket permission and wrapped certificate/TLS errors stay generic.
		if timeoutCause(err) {
			return failureTimeout
		}
		if errors.Is(err, fs.ErrPermission) || certificateValidationFailure(err) {
			return failureUnknown
		}
		if opErr != nil && tlsAlert(opErr, err) {
			return failureUnknown
		}
		return failureNetwork
	case errors.Is(err, fs.ErrPermission):
		return failurePermission
	}
	return failureUnknown
}

// Certificate validation errors may occur inside net.OpError. They are not
// evidence that the network is unreachable, even when a dial failed.
func certificateValidationFailure(err error) bool {
	var (
		unknownAuthority x509.UnknownAuthorityError
		invalid          x509.CertificateInvalidError
		hostname         x509.HostnameError
		roots            x509.SystemRootsError
	)
	return errors.As(err, &unknownAuthority) || errors.As(err, &invalid) ||
		errors.As(err, &hostname) || errors.As(err, &roots)
}

// tlsAlert reports a TLS failure that surfaces as a net.OpError: an alert sent
// by the remote end ("remote error"), one raised by the local client ("local
// error"), or a handshake or certificate error that an https proxy path wraps
// in an operation error. The network was reachable, so none of them is an
// unreachable-network problem.
func tlsAlert(opErr *net.OpError, err error) bool {
	if opErr.Op == "remote error" || opErr.Op == "local error" {
		return true
	}
	var (
		certErr   *tls.CertificateVerificationError
		headerErr tls.RecordHeaderError
	)
	return errors.As(err, &certErr) || errors.As(err, &headerErr)
}

func timeoutCause(err error) bool {
	var timeout interface{ Timeout() bool }
	return errors.Is(err, os.ErrDeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout()
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

	// Keep the run-wide stop policy when a binding's displayed kind becomes SKIPPED.
	stopsNamespaceRetry bool
}

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
	failure.stopsNamespaceRetry = cause == failureProcessCleanup || cause == failureRecovery
	if outcome.Status == environment.BindingNotAttempted && cause != failureProcessCleanup {
		failure.Kind = failureSkipped
	}
	failure.Summary, failure.Next, failure.Retryable = applyFailureWording(failure.Kind)
	if failure.Kind == failureSkipped {
		// A skipped agent is only worth retrying when what stopped the run can be fixed by retrying.
		_, _, failure.Retryable = applyFailureWording(cause)
	}
	explainRetainedRevision(&failure, cause, outcome.Err)
	return &failure
}

// describeSyncFailure explains a failure that happened before any binding ran,
// such as a lock, readiness or input-guard refusal. Every binding shares it.
func describeSyncFailure(err error) *installFailure {
	failure := installFailure{Kind: failureKindOf(err), Detail: errorText(err)}
	failure.Summary, failure.Next, failure.Retryable = applyFailureWording(failure.Kind)
	explainRetainedRevision(&failure, failure.Kind, err)
	return &failure
}

const retainedRevisionSummary = "This Directory installation has no installed targets."
const retainedRevisionGuidance = "Replacing its recorded release through add is not supported, and update requires an installed target. The recorded revision is kept."

func explainRetainedRevision(failure *installFailure, cause failureKind, err error) {
	if cause != failureUpdateRequired || !errors.Is(err, domain.ErrRetainedRevisionChange) {
		return
	}
	if failure.Kind != failureSkipped {
		failure.Summary = retainedRevisionSummary
	}
	failure.Next, failure.Retryable = retainedRevisionGuidance, false
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
	case failureProcessCleanup:
		return "The agent's process cleanup could not be confirmed.", "Confirm the agent's processes have stopped before starting another installation.", false
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
	case failurePermission:
		return "UAP does not have permission to use a folder this agent needs.", "Fix the folder's permissions, then retry this agent.", true
	case failureReadOnly:
		return "A folder this agent needs is read-only.", "Make the location writable or choose another one, then retry this agent.", true
	case failureDiskFull:
		return "There is not enough free disk space.", "Free some space, then retry this agent.", true
	case failureNetwork:
		return "A download needed for this agent failed because the network could not be reached.", "Check your connection, then retry this agent.", true
	case failureAgentMissing:
		return "A program this agent needs was not found or cannot be run.", "Install it or add it to your PATH, then retry.", true
	case failureCanceled:
		return "The installation was canceled.", "Retry when you are ready.", true
	case failureSkipped:
		return "Not attempted because an earlier problem stopped the installation.", "Retry this agent once the earlier problem is resolved.", true
	}
	return "The installation did not complete for this agent.", "Retry, or hand the technical log to an agent to diagnose.", true
}

// installFailureWithCause keeps a classifiable cause without putting its
// technical detail in the public command error.
type installFailureWithCause struct {
	summary string
	cause   error
}

func (err installFailureWithCause) Error() string { return err.summary }
func (err installFailureWithCause) Unwrap() error { return err.cause }

// retainUnclosedInstallCause keeps cleanup uncertainty through the group's
// public aggregate. Ordinary group failures keep their existing presentation.
func retainUnclosedInstallCause(summary, cause error) error {
	if !domain.IsUnclosedClientProbe(cause) {
		return summary
	}
	return installFailureWithCause{summary: summary.Error(), cause: cause}
}
