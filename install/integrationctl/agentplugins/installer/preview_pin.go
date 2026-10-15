package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// PreviewInstallAfterUpdate checks a new target against the pin of an already
// prepared, owned update. Only the shared package pin is projected in memory;
// live bindings, receipts, profiles and namespace checks are retained. The
// returned handle is preview-only: Apply on the originating engine rejects it.
// A caller must Prepare afresh after the actual update commits.
func (e *Engine) PreviewInstallAfterUpdate(ctx context.Context, req Request, update *PreparedOperation) (*PreparedOperation, error) {
	if update == nil || update.engine != e {
		return nil, ErrInvalidHandle
	}
	update.mu.Lock()
	defer update.mu.Unlock()
	if update.closed || update.busy || update.applied || update.req.Operation != OpUpdate || req.Operation != OpInstall || len(req.Targets) != 0 {
		return nil, ErrInvalidHandle
	}
	if err := validateProjectedInstallIdentity(req, update); err != nil {
		return nil, err
	}
	state, err := e.store.Load()
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(state, update.recorded) {
		return nil, ErrPlanChanged
	}
	found := false
	for i := range state.Installations {
		installation := &state.Installations[i]
		if installation.InstallationID != req.InstallationID {
			continue
		}
		for _, binding := range installation.Clients {
			if binding.ClientID == req.ClientID {
				return nil, ErrPlanChanged
			}
		}
		installation.Source.TreeDigest = update.snapshot.TreeDigest
		found = true
	}
	if !found {
		return nil, ErrPlanChanged
	}
	projected := *e
	projected.store = projectedPinStore{state: state}
	return projected.Prepare(ctx, req)
}

// validateProjectedInstallIdentity binds the projected pin to the same owner
// and installation while requiring an unbound, distinct target.
func validateProjectedInstallIdentity(req Request, update *PreparedOperation) error {
	old, wanted := update.req.EnvironmentAuthority, req.EnvironmentAuthority
	if old == nil || wanted == nil || old.EnvironmentID != wanted.EnvironmentID || old.Alias != wanted.Alias || old.SourceKey != wanted.SourceKey || req.InstallationID == "" || req.InstallationID != update.req.InstallationID || wanted.InstallationID != req.InstallationID || wanted.ClientID == old.ClientID || wanted.ReceiptID != "" || wanted.ClientBindingID != "" {
		return fmt.Errorf("%w: projected install requires the same owned installation and a new target", ErrInvalidRequest)
	}
	if req.AcquiredSnapshot == nil || req.AcquiredSnapshot.TreeDigest != update.snapshot.TreeDigest {
		return fmt.Errorf("%w: projected pin differs from prepared update", ErrInvalidRequest)
	}
	return nil
}

type projectedPinStore struct{ state domain.StateFileV2 }

func (s projectedPinStore) Load() (domain.StateFileV2, error) {
	// Each consumer gets an isolated value, including nested binding maps.
	body, err := json.Marshal(s.state)
	if err != nil {
		return domain.StateFileV2{}, err
	}
	var out domain.StateFileV2
	err = json.Unmarshal(body, &out)
	return out, err
}
func (s projectedPinStore) Save(domain.StateFileV2) error {
	return fmt.Errorf("%w: projected preflight is read-only", ErrInvalidRequest)
}
