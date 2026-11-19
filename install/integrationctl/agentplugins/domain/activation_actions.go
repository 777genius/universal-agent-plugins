package domain

import "slices"

// PendingUserActions separates a completed disclosure from steps the installer
// must still present as manual. Unverified, attested or uncertain outcomes keep
// every action, and an unmarked action always remains pending.
func (outcome ActivationOutcome) PendingUserActions() []string {
	actions := slices.Clone(outcome.UserActions)
	if outcome.Activation != ActivationActive || outcome.Verification != VerificationInstalled ||
		outcome.Policy != PolicyAllowed || outcome.NativeEffect == NativeEffectUncertain ||
		outcome.ActivationAttested || outcome.AuthenticationAttested ||
		(outcome.Authentication != AuthenticationNotRequired && outcome.Authentication != AuthenticationComplete) {
		return actions
	}
	return slices.DeleteFunc(actions, func(action string) bool {
		return slices.Contains(outcome.CompletedUserActions, action)
	})
}
