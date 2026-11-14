package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func (service Service) guardNativeEnvironmentRecovery(state domain.StateFileV2) error {
	observer, ok := service.Kernel.NativeRecovery.(ports.NativeTransitionObserver)
	if !ok {
		return nil
	}
	pending, err := observer.Pending()
	if err != nil {
		return err
	}
	for _, item := range pending {
		owner, valid := ports.NativeTransitionOwner(state, item)
		if !valid || owner != "" || item.EnvironmentID != "" || service.environmentAuthority != nil {
			return fmt.Errorf("recovery_required: pending native operation requires explicit environment recovery")
		}
	}
	return nil
}
