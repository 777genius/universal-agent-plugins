package enterprisepolicy

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// IsUnrestricted is a narrow optimization for this adapter only. Unknown gates
// must receive actual package facts; probing them with an empty request would
// allow a custom source-sensitive gate to be skipped accidentally.
func IsUnrestricted(ctx context.Context, gate ports.InstallationPolicy) (bool, error) {
	if gate == nil {
		return true, nil
	}
	if frozen, ok := gate.(*session); ok {
		return frozen.isUnrestricted(ctx)
	}
	snapshot, recognized, err := loadKnownFiles(ctx, gate)
	return recognized && err == nil && snapshot.RestrictionCount == 0, err
}

func (s *session) isUnrestricted(ctx context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, recognized, err := loadKnownFiles(ctx, s.gate)
	if !recognized || err != nil {
		return false, err
	}
	if s.checked && snapshot.Fingerprint != s.fingerprint {
		return false, domain.ErrInstallPolicyChanged
	}
	if snapshot.RestrictionCount != 0 {
		return false, nil
	}
	// Capture the exact inspected snapshot. Calling Check again here would
	// introduce a second read that could authorize a different policy version.
	s.checked, s.fingerprint = true, snapshot.Fingerprint
	return true, nil
}

func loadKnownFiles(ctx context.Context, gate ports.InstallationPolicy) (Snapshot, bool, error) {
	switch concrete := gate.(type) {
	case Files:
		snapshot, err := concrete.Load(ctx)
		return snapshot, true, err
	case *Files:
		if concrete == nil {
			return Snapshot{}, false, nil
		}
		snapshot, err := concrete.Load(ctx)
		return snapshot, true, err
	case *session:
		return loadKnownFiles(ctx, concrete.gate)
	default:
		return Snapshot{}, false, nil
	}
}
