package agentpluginscli

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestBindingNeedsAttentionMatchesLifecycleStates(t *testing.T) {
	cases := []struct {
		name    string
		binding domain.ClientBinding
		want    bool
	}{
		{name: "none", binding: domain.ClientBinding{}, want: false},
		{name: "degraded materialization", binding: domain.ClientBinding{Materialization: domain.MaterializationDegraded}, want: true},
		{name: "failed activation", binding: domain.ClientBinding{Activation: domain.ActivationFailed}, want: true},
		{name: "manual activation", binding: domain.ClientBinding{Activation: domain.ActivationManual}, want: true},
		{name: "prepared activation", binding: domain.ClientBinding{Activation: domain.ActivationPrepared}, want: true},
		{name: "failed authentication", binding: domain.ClientBinding{Authentication: domain.AuthenticationFailed}, want: true},
		{name: "pending authentication", binding: domain.ClientBinding{Authentication: domain.AuthenticationPending}, want: true},
		{name: "failed verification", binding: domain.ClientBinding{Verification: domain.VerificationFailed}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bindingNeedsAttention(tc.binding); got != tc.want {
				t.Fatalf("bindingNeedsAttention(%+v) = %t, want %t", tc.binding, got, tc.want)
			}
		})
	}
}
