package usecase

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func validateNativeBinding(client domain.ClientBinding, detected domain.DetectedClient) error {
	if err := client.ValidateLocalEntryObservation(); err != nil {
		return err
	}
	if client.NativeActivationAttempt != "" {
		return fmt.Errorf("native activation attempt %s is unresolved; review client state before another mutation", client.NativeActivationAttempt)
	}
	if err := client.SelectedDelivery.Validate(); err != nil {
		return err
	}
	if !client.SelectedDelivery.EffectiveTraits(detected.ClientID).BindsNativeProfileRoot {
		return nil
	}
	if client.NativeProfileRoot == "" {
		return fmt.Errorf("legacy native binding has no proven native profile root; reviewed rebind is required")
	}
	if detected.ConfigRoot == "" || !filepath.IsAbs(detected.ConfigRoot) || filepath.Clean(detected.ConfigRoot) != client.NativeProfileRoot {
		return fmt.Errorf("selected native profile differs from the binding's native profile root")
	}
	return nil
}
