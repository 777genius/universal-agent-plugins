package shared

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ErrRecognizedNegativeEvidence marks a failure the client itself proved, as
// opposed to one we only failed to disprove. Only the former is an
// authoritative observation.
var ErrRecognizedNegativeEvidence = errors.New("recognized negative client evidence")

// FailedActivation is the single shape of a failed activation outcome: failed
// activation, failed verification, one retry action for the user and the
// client-specific next step locally.
func FailedActivation(outcome domain.ActivationOutcome, next string, err error) (domain.ActivationOutcome, error) {
	outcome.Activation = domain.ActivationFailed
	outcome.Verification = domain.VerificationFailed
	outcome.AuthoritativeObservation = errors.Is(err, ErrRecognizedNegativeEvidence)
	outcome.UserActions = []string{"retry client activation and verify client visibility"}
	outcome.LocalActions = []string{next}
	return outcome, err
}

// RequireExternalUninstall records that removal has to finish in the client
// itself. Until it does, the managed artifact must stay in place: deleting it
// while the client still points at it would leave a dangling registration.
func RequireExternalUninstall(outcome domain.DeactivationOutcome, complete bool, action string) domain.DeactivationOutcome {
	if complete {
		outcome.ExternalRemovalComplete = true
		return outcome
	}
	outcome.Activation = domain.ActivationManual
	outcome.ArtifactRemovalAllowed = false
	outcome.UserActions = append(outcome.UserActions, action)
	return outcome
}

// AttestedUnknownVerification accepts the operator's confirmation when the
// client's own output contract was not recognized. The outcome is marked
// attested so a later reader can tell it apart from a machine-verified one.
func AttestedUnknownVerification(outcome domain.ActivationOutcome, request domain.ActivationRequest) (domain.ActivationOutcome, bool) {
	if !request.ActivationComplete {
		return outcome, false
	}
	outcome.Activation = domain.ActivationActive
	outcome.Verification = domain.VerificationInstalled
	outcome.ActivationAttested = true
	return outcome, true
}
