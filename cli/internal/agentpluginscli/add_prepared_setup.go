package agentpluginscli

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// This chooses normal setup confirmation rather than manual attestation. It
// grants no effect authority: confirmed Add reobserves and validates the actual
// owned delivery before the same client capability can admit an acting retry.
func canConfirmPreparedSetup(service usecase.Service, input usecase.AddInput, planned usecase.AddResult) bool {
	observed := planned.Activation
	intent := input.InstallIntent
	if intent == "" {
		intent = planned.Plan.InstallIntent
	}
	if !planned.RequiresConfirmation || input.AuthComplete || input.ActivationComplete || intent != domain.InstallIntentAutomatic ||
		observed.Activation != domain.ActivationPrepared || observed.Verification != domain.VerificationInstalled || observed.Policy != domain.PolicyAllowed ||
		observed.ActivationAttested || observed.AuthenticationAttested || (observed.NativeEffect != "" && observed.NativeEffect != domain.NativeEffectUnchanged) {
		return false
	}
	classifier, ok := service.Activator.(ports.PreparedSetupClassifier)
	if !ok {
		return false
	}
	plan := planned.Plan
	plan.Components = append([]domain.ComponentDecision(nil), plan.Components...)
	plan.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), plan.PreviousNativeObjects...)
	plan.LocalEntryObservation = plan.LocalEntryObservation.Clone()
	client := input.Client
	client.ProfileAuthority = domain.CloneProfileAuthority(client.ProfileAuthority)
	client.Surfaces = append([]domain.ClientSurface(nil), client.Surfaces...)
	if !classifier.PreparedSetupVerifierAvailable(client, plan, input.BackendExecutable) {
		return false
	}
	observed.NativeObjects = append([]domain.NativeObjectOwnership(nil), observed.NativeObjects...)
	observed.UserActions = append([]string(nil), observed.UserActions...)
	observed.LocalActions = append([]string(nil), observed.LocalActions...)
	observed.LocalEntryObservation = observed.LocalEntryObservation.Clone()
	return classifier.CanCompletePreparedSetup(domain.ActivationRequest{
		Client: client, Plan: plan, DeclaredName: input.Envelope.Manifest.Name, BackendExecutable: input.BackendExecutable, Replacing: true,
		Delivery:              domain.StagedDelivery{ClientID: client.ClientID, OwnedBase: plan.TargetRoot, ActivePath: plan.ActivePath, ArtifactDigest: planned.Receipt.AfterDigest, NativeObjects: append([]domain.NativeObjectOwnership(nil), plan.PreviousNativeObjects...)},
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), plan.PreviousNativeObjects...),
	}, observed)
}
