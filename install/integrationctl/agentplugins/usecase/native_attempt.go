package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (service Service) persistLifecycleState(desired domain.StateFileV2) error {
	before, err := service.StateStore.Load()
	if err != nil {
		return err
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	return kernel.PersistStateDecision(before, desired)
}

// beginNativeAttempt is called under the service mutation lock, before an
// adapter can change client configuration. A retained marker means a previous
// effect could not be reconciled and forbids another blind mutation.
func (service Service) beginNativeAttempt(installationID, bindingID string) error {
	state, err := service.StateStore.Load()
	if err != nil {
		return err
	}
	for i, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		client, ok := installation.Clients[bindingID]
		if !ok {
			return fmt.Errorf("client binding disappeared before native activation")
		}
		if client.NativeActivationAttempt != "" {
			return fmt.Errorf("native activation attempt %s is unresolved; inspect owned objects before retry or removal", client.NativeActivationAttempt)
		}
		attemptID, err := newOperationID()
		if err != nil {
			return err
		}
		state.Installations = append([]domain.Installation(nil), state.Installations...)
		installation.Clients = cloneClientBindings(installation.Clients)
		client.NativeActivationAttempt = attemptID
		installation.Clients[bindingID] = client
		state.Installations[i] = installation
		return service.persistLifecycleState(state)
	}
	return fmt.Errorf("installation disappeared before native activation")
}

func cloneClientBindings(source map[string]domain.ClientBinding) map[string]domain.ClientBinding {
	result := make(map[string]domain.ClientBinding, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func (service Service) activateWithNativeAttempt(ctx context.Context, installationID, bindingID string, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if nativeLifecycleClient(request.Client.ClientID) && !request.VerifyOnly {
		if err := service.beginNativeAttempt(installationID, bindingID); err != nil {
			return domain.ActivationOutcome{}, err
		}
	}
	return service.Activator.Activate(ctx, request)
}

// completeNativeRemoval records a known deactivation before a later managed
// directory transaction. A failed Deactivate leaves the attempt marker in
// place, because a partial native write cannot safely be inferred away.
func (service Service) completeNativeRemoval(installationID, bindingID string, removed bool) error {
	state, err := service.StateStore.Load()
	if err != nil {
		return err
	}
	for i, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		client, ok := installation.Clients[bindingID]
		if !ok || client.NativeActivationAttempt == "" {
			return fmt.Errorf("native removal attempt disappeared before recording its effect")
		}
		state.Installations = append([]domain.Installation(nil), state.Installations...)
		installation.Clients = cloneClientBindings(installation.Clients)
		client.NativeActivationAttempt = ""
		if removed {
			ownedPackage := make([]domain.NativeObjectOwnership, 0, 1)
			for _, object := range client.NativeObjects {
				if object.Kind == "managed_package_directory" {
					ownedPackage = append(ownedPackage, object)
				}
			}
			client.NativeObjects = ownedPackage
			client.Activation = domain.ActivationNotRequired
			client.Verification = domain.VerificationPackageValid
		}
		client.UpdatedAt = service.now().Format(time.RFC3339Nano)
		installation.Clients[bindingID] = client
		installation.UpdatedAt = client.UpdatedAt
		state.Installations[i] = installation
		return service.persistLifecycleState(state)
	}
	return fmt.Errorf("installation disappeared before recording native removal")
}
