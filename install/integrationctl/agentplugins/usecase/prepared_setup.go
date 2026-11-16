package usecase

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func (service Service) preparedSetupVerifierAvailable(input AddInput, plan domain.DeliveryPlan) bool {
	classifier, ok := service.Activator.(ports.PreparedSetupClassifier)
	return ok && classifier.PreparedSetupVerifierAvailable(clonePreparedVerifierClient(input.Client), clonePreparedVerifierPlan(plan), input.BackendExecutable)
}

func (a profileActivator) PreparedSetupVerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, executable string) bool {
	classifier, ok := a.ClientActivator.(ports.PreparedSetupClassifier)
	if !ok || a.check() != nil {
		return false
	}
	available := classifier.PreparedSetupVerifierAvailable(clonePreparedVerifierClient(client), clonePreparedVerifierPlan(plan), executable)
	return a.check() == nil && available
}

func (session *repairSession) canCompletePreparedSetup(delivery domain.StagedDelivery, observed domain.ActivationOutcome) bool {
	if observed.Activation != domain.ActivationPrepared || observed.Verification != domain.VerificationInstalled || observed.Policy != domain.PolicyAllowed {
		return false
	}
	if observed.NativeEffect != "" && observed.NativeEffect != domain.NativeEffectUnchanged {
		return false
	}
	classifier, ok := session.service.Activator.(ports.PreparedSetupClassifier)
	if !ok {
		return false
	}
	delivery.NativeObjects = append([]domain.NativeObjectOwnership(nil), delivery.NativeObjects...)
	observed.NativeObjects = append([]domain.NativeObjectOwnership(nil), observed.NativeObjects...)
	observed.UserActions = append([]string(nil), observed.UserActions...)
	observed.LocalActions = append([]string(nil), observed.LocalActions...)
	observed.LocalEntryObservation = observed.LocalEntryObservation.Clone()
	return classifier.CanCompletePreparedSetup(domain.ActivationRequest{
		Client: clonePreparedVerifierClient(session.input.Client), Plan: clonePreparedVerifierPlan(session.result.Plan), Delivery: delivery,
		DeclaredName: session.input.Envelope.Manifest.Name, Replacing: true,
		BackendExecutable:     session.input.BackendExecutable,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), session.client.NativeObjects...),
		ProjectTrustConsent:   session.input.ProjectTrustConsent,
	}, observed)
}

func (a profileActivator) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	classifier, ok := a.ClientActivator.(ports.PreparedSetupClassifier)
	if !ok || a.check() != nil {
		return false
	}
	request.Client.ProfileAuthority = domain.CloneProfileAuthority(request.Client.ProfileAuthority)
	allowed := classifier.CanCompletePreparedSetup(request, observed)
	return a.check() == nil && allowed
}

// Membership must not add profile/state observations for adapters that did
// not opt in. The actual classifier is still guarded on both sides below.
func (a profileActivator) PreparedMaterializationVerifierAvailable(client domain.DetectedClient) bool {
	verifier, ok := a.ClientActivator.(ports.PreparedMaterializationVerifier)
	return ok && verifier.PreparedMaterializationVerifierAvailable(clonePreparedVerifierClient(client))
}

func (a profileActivator) PreparedMaterializationVerified(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	verifier, ok := a.ClientActivator.(ports.PreparedMaterializationVerifier)
	if !ok || a.check() != nil {
		return false
	}
	request.Client = clonePreparedVerifierClient(request.Client)
	request.Plan = clonePreparedVerifierPlan(request.Plan)
	request.Delivery.NativeObjects = append([]domain.NativeObjectOwnership(nil), request.Delivery.NativeObjects...)
	request.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), request.PreviousNativeObjects...)
	verified := verifier.PreparedMaterializationVerified(request, cloneMaterializationOutcome(observed))
	return a.check() == nil && verified
}

func cloneMaterializationOutcome(observed domain.ActivationOutcome) domain.ActivationOutcome {
	observed.NativeObjects = append([]domain.NativeObjectOwnership(nil), observed.NativeObjects...)
	observed.UserActions = append([]string(nil), observed.UserActions...)
	observed.LocalActions = append([]string(nil), observed.LocalActions...)
	observed.LocalEntryObservation = observed.LocalEntryObservation.Clone()
	return observed
}
