package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func (service Service) verifyManagedTarget(
	ctx context.Context,
	client domain.DetectedClient,
	scope domain.InstallScope,
	binding domain.ClientBinding,
	operation string,
) error {
	if service.Targets == nil {
		return fmt.Errorf("delivery target resolver is required for %s", operation)
	}
	expectedDigest := managedDigest(binding)
	if expectedDigest == "" {
		return fmt.Errorf("managed package digest is missing; refusing %s and retaining state for reviewed recovery", operation)
	}
	targetClient := client
	if sameNativeBackend(domain.ClientID(binding.ClientID), client.ClientID) {
		// Validate the persisted path against the physical binding's canonical
		// owner, even when the caller addressed the shared backend through its
		// other logical surface.
		targetClient.ClientID = domain.ClientID(binding.ClientID)
	}
	target, err := service.Targets.ResolveTarget(ctx, targetClient, scope, binding.PhysicalArtifact)
	if err != nil {
		return fmt.Errorf("resolve managed %s target: %w", operation, err)
	}
	if err := service.Paths.RequireExactPath(target.ActivePath, binding.TargetLocator); err != nil {
		return fmt.Errorf("refuse %s from untrusted persisted target: %w", operation, err)
	}
	if err := service.Stager.Verify(ctx, binding.TargetLocator, expectedDigest); err != nil {
		return fmt.Errorf("managed package was changed or is missing; refusing silent %s and retaining state: %w", operation, err)
	}
	return nil
}

func (service Service) observeNativeIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) error {
	observation, err := service.nativeIdentityObservation(ctx, client, plan, managed)
	if err != nil {
		return fmt.Errorf("observe native identity for %s: %w", client.ClientID, err)
	}
	return validateNativeIdentityObservation(observation, managed)
}

func (service Service) observePreparedIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) error {
	observation, err := service.preparedIdentityObservation(ctx, client, plan, managed)
	if err != nil {
		return fmt.Errorf("observe prepared identity for %s: %w", client.ClientID, err)
	}
	return validateNativeIdentityObservation(observation, managed)
}

func validateNativeIdentityObservation(observation domain.NativeIdentityObservation, managed *domain.ClientBinding) error {
	switch observation.State {
	case NativeIdentityAbsent:
		if managed != nil && managed.Materialization != domain.MaterializationAbsent {
			return fmt.Errorf("managed native identity is unexpectedly absent")
		}
		return nil
	case NativeIdentityManaged:
		if managed == nil {
			return fmt.Errorf("native identity already exists without matching agentplugins ownership; automatic adoption is disabled")
		}
		expected := managedDigest(*managed)
		if expected == "" || observation.Digest == "" || expected != observation.Digest {
			return fmt.Errorf("native identity ownership digest is stale or does not match")
		}
		return nil
	case NativeIdentityUnmanaged:
		return fmt.Errorf("native identity is unmanaged; remove it through its owning client or choose a distinct identity")
	case NativeIdentityIndeterminate:
		return fmt.Errorf("native identity ownership is indeterminate; refusing mutation")
	default:
		return fmt.Errorf("native identity observer returned an unknown state")
	}
}

func (service Service) nativeIdentityObservation(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if service.NativeObserver != nil {
		return service.NativeObserver.ObserveNativeIdentity(ctx, client, plan, managed)
	}
	return service.filesystemIdentityObservation(ctx, plan, managed)
}

func (service Service) preparedIdentityObservation(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if observer, ok := service.NativeObserver.(PreparedIdentityObserver); ok {
		return observer.ObservePreparedIdentity(ctx, client, plan, managed)
	}
	return service.filesystemIdentityObservation(ctx, plan, managed)
}

func (service Service) filesystemIdentityObservation(ctx context.Context, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	// Every backend gets a fail-closed filesystem observation even when it has
	// no richer namespace-aware observer. A specialized observer may prove
	// qualified coexistence; the fallback never does.
	if _, err := os.Lstat(plan.ActivePath); os.IsNotExist(err) {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityAbsent}, nil
	} else if err != nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, err
	}
	if managed == nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityUnmanaged}, nil
	}
	expected := managedDigest(*managed)
	if expected == "" {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, nil
	}
	if err := service.Stager.Verify(ctx, plan.ActivePath, expected); err != nil {
		var verification *ports.VerificationError
		if errors.As(err, &verification) {
			return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate, Digest: verification.ActualDigest}, nil
		}
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, err
	}
	return domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, Digest: expected}, nil
}
