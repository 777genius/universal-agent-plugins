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
	if sameNativeBackend(domain.ClientID(binding.ClientID), client.ClientID, scope) {
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
		return &domain.TypedError{Text: fmt.Sprintf("managed package was changed or is missing; refusing silent %s and retaining state: %v", operation, err), Kind: domain.ErrManagedPackageChanged, Cause: err}
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
			return &domain.TypedError{Text: "managed native identity is unexpectedly absent", Kind: domain.ErrManagedPackageChanged}
		}
		return nil
	case NativeIdentityManaged:
		if managed == nil {
			return &domain.TypedError{Text: "native identity already exists without matching agentplugins ownership; automatic adoption is disabled", Kind: domain.ErrForeignNativeIdentity}
		}
		expected := managedDigest(*managed)
		if expected == "" || observation.Digest == "" || expected != observation.Digest {
			return &domain.TypedError{Text: "native identity ownership digest is stale or does not match", Kind: domain.ErrManagedPackageChanged}
		}
		return nil
	case NativeIdentityUnmanaged:
		return &domain.TypedError{Text: "native identity is unmanaged; remove it through its owning client or choose a distinct identity", Kind: domain.ErrForeignNativeIdentity}
	case NativeIdentityIndeterminate:
		return fmt.Errorf("native identity ownership is indeterminate; refusing mutation")
	default:
		return fmt.Errorf("native identity observer returned an unknown state")
	}
}

func (service Service) nativeIdentityObservation(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if service.ClientPreparation != nil {
		if err := service.ClientPreparation.RevalidateClient(ctx, client, plan); err != nil {
			return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, err
		}
	}
	if service.NativeObserver != nil {
		return service.NativeObserver.ObserveNativeIdentity(ctx, client, cloneLocalObservationPlan(plan), cloneLocalObservationBinding(managed))
	}
	return service.filesystemIdentityObservation(ctx, plan, managed)
}

func (service Service) preparedIdentityObservation(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if observer, ok := service.NativeObserver.(PreparedIdentityObserver); ok {
		return observer.ObservePreparedIdentity(ctx, client, cloneLocalObservationPlan(plan), cloneLocalObservationBinding(managed))
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
