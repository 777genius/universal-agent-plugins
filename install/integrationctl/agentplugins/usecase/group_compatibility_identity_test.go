package usecase

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A new live binding with the same client/backend is not the host-verified
// unselected binding. Recheck exact identity after the usecase acquires its lock.
func TestGroupCompatibilityRejectsChangedUnselectedBindingUnderLock(t *testing.T) {
	recorded := domain.ClientBinding{
		ClientBindingID: "verified-binding", ClientID: "test-client",
		Scope: string(domain.ScopeUser), Materialization: domain.MaterializationMaterialized,
	}
	changed := recorded
	changed.ClientBindingID = "replacement-binding"
	changedNamespace := recorded
	changedNamespace.ProfileNamespace = "TEST-other-namespace"
	changedAuthority := recorded
	changedAuthority.ProfileAuthority = &domain.ProfileAuthority{}
	unselected := AddInput{
		Client: domain.DetectedClient{ClientID: domain.ClientID("test-client")},
		Scope:  domain.ScopeUser, CompatibilityBindingID: recorded.ClientBindingID,
	}
	for _, tc := range []struct {
		name      string
		clients   map[string]domain.ClientBinding
		wantError bool
	}{
		{"unchanged", map[string]domain.ClientBinding{recorded.ClientBindingID: recorded}, false},
		{"replaced-after-observation", map[string]domain.ClientBinding{changed.ClientBindingID: changed}, true},
		{"different-profile-namespace", map[string]domain.ClientBinding{recorded.ClientBindingID: changedNamespace}, true},
		{"different-profile-authority", map[string]domain.ClientBinding{recorded.ClientBindingID: changedAuthority}, true},
		{"missing", map[string]domain.ClientBinding{}, true},
		{"different-client", map[string]domain.ClientBinding{recorded.ClientBindingID: {
			ClientBindingID: recorded.ClientBindingID, ClientID: "other-client", Scope: string(domain.ScopeUser),
			Materialization: domain.MaterializationMaterialized,
		}}, true},
		{"different-scope", map[string]domain.ClientBinding{recorded.ClientBindingID: {
			ClientBindingID: recorded.ClientBindingID, ClientID: recorded.ClientID, Scope: string(domain.ScopeProject),
			Materialization: domain.MaterializationMaterialized,
		}}, true},
		{"absent-binding", map[string]domain.ClientBinding{recorded.ClientBindingID: {
			ClientBindingID: recorded.ClientBindingID, ClientID: recorded.ClientID, Scope: string(domain.ScopeUser),
			Materialization: domain.MaterializationAbsent,
		}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := groupSession{state: domain.StateFileV2{
				Installations: []domain.Installation{{Clients: tc.clients}},
			}}
			err := session.requireObservedCompatibilityBinding(unselected)
			if (err != nil) != tc.wantError {
				t.Fatalf("exact binding identity mismatch: error=%v wantError=%v", err, tc.wantError)
			}
		})
	}
	// Selected targets do not carry observational sibling facts.
	session := groupSession{state: domain.StateFileV2{Installations: []domain.Installation{{}}}}
	if err := session.requireObservedCompatibilityBinding(AddInput{}); err != nil {
		t.Fatalf("ordinary selected input gained a new preflight requirement: %v", err)
	}
}
