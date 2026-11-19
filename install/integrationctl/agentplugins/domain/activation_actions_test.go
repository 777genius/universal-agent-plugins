package domain

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestCompletedDisclosurePreservesPendingActionsAndPublicInformation(t *testing.T) {
	outcome := ActivationOutcome{Activation: ActivationActive, Verification: VerificationInstalled,
		Policy: PolicyAllowed, Authentication: AuthenticationNotRequired,
		UserActions:          []string{"Verified setup disclosure", "Independent pending action"},
		CompletedUserActions: []string{"Verified setup disclosure"}}
	for _, state := range []string{"observed", "unmarked", "prepared", "unverified", "policy", "auth", "attested", "uncertain"} {
		t.Run(state, func(t *testing.T) {
			selected := outcome
			switch state {
			case "unmarked":
				selected.CompletedUserActions = nil
			case "prepared":
				selected.Activation = ActivationPrepared
			case "unverified":
				selected.Verification = VerificationPackageValid
			case "policy":
				selected.Policy = PolicyBlocked
			case "auth":
				selected.Authentication = AuthenticationNotChecked
			case "attested":
				selected.ActivationAttested = true
			case "uncertain":
				selected.NativeEffect = NativeEffectUncertain
			}
			wanted := outcome.UserActions
			if state == "observed" {
				wanted = []string{"Independent pending action"}
			}
			pending := selected.PendingUserActions()
			if !slices.Equal(pending, wanted) {
				t.Fatalf("pending action authority changed: %v", pending)
			}
			pending[0] = "changed presentation"
			if outcome.UserActions[0] != "Verified setup disclosure" || outcome.UserActions[1] != "Independent pending action" {
				t.Fatal("pending presentation mutated the original disclosure")
			}
		})
	}
	body, err := json.Marshal(outcome)
	if err != nil || !strings.Contains(string(body), "Verified setup disclosure") || strings.Contains(string(body), "CompletedUserActions") {
		t.Fatalf("completed classification changed public disclosure: %s %v", body, err)
	}
}
