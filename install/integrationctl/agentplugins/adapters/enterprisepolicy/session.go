package enterprisepolicy

import (
	"context"
	"sync"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// NewSession freezes the first successful policy fingerprint for one reviewed
// operation, while every subsequent check still reads and evaluates fresh rules.
// A new operation must create a new session; this is not daemon-global state.
func NewSession(gate ports.InstallationPolicy) ports.InstallationPolicy {
	if gate == nil {
		return nil
	}
	return &session{gate: gate}
}

type session struct {
	mu          sync.Mutex
	gate        ports.InstallationPolicy
	fingerprint string
	checked     bool
}

func (s *session) Check(ctx context.Context, req domain.InstallPolicyRequest) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fingerprint, err := s.gate.Check(ctx, req)
	if err != nil {
		return fingerprint, err
	}
	if s.checked && fingerprint != s.fingerprint {
		return fingerprint, domain.ErrInstallPolicyChanged
	}
	s.checked, s.fingerprint = true, fingerprint
	return fingerprint, nil
}
