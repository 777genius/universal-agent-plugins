package usecase

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

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
