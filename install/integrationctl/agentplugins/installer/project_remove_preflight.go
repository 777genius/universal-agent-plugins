package installer

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// Observe the recorded native file set at each existing removal fence. Empty
// Components deliberately asks the adapter to verify prior ownership only;
// there is no new desired projection, native discovery or command execution.
func (e *Engine) confirmProjectRemovalFiles(ctx context.Context, client domain.DetectedClient, name string, binding domain.ClientBinding, target domain.DeliveryTarget, digest string) error {
	if e.scope() != domain.ScopeProject || !binding.SelectedDelivery.IsZero() ||
		binding.SelectedDelivery.EffectiveTraitsForScope(client.ClientID, e.scope()).LifecycleKind != domain.LifecycleNativeConfig {
		return nil
	}
	plan := domain.DeliveryPlan{
		ClientID: client.ClientID, Scope: domain.ScopeProject, DeclaredName: name,
		TargetContext: target.TargetContext, TargetAnchor: target.TargetAnchor,
		TargetRoot: target.TargetRoot, ActivePath: target.ActivePath, NativeRegistryRoot: target.NativeRegistryRoot,
		PhysicalArtifactID: binding.PhysicalArtifact, SelectedDelivery: binding.SelectedDelivery,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...),
		LocalEntryObservation: binding.LocalEntryObservation.Clone(),
	}.WithProfileAuthority(target.ProfileAuthority(), target.ProfileNamespace())
	client.ProfileAuthority = domain.CloneProfileAuthority(client.ProfileAuthority)
	managed := projectRemovalObservationBinding(binding)
	kernel := nativeconfig.New()
	observer := providers.NativeIdentityObserver{Stager: providers.Stager{}, NativeConfig: &kernel, Registry: e.cfg.Registry}
	observed, err := observer.ObservePreparedIdentity(ctx, client, plan, &managed)
	if err != nil {
		return fmt.Errorf("%w: project native file removal preflight: %w", ErrInvalidRequest, err)
	}
	if observed.State != domain.NativeIdentityManaged || !observed.ReceiptReconciled || observed.Digest != digest ||
		observed.NativeDiscoveryState != domain.NativeIdentityAbsent || observed.NativeDiscoveryAttempted || observed.NativeDiscoveryReconciled {
		return fmt.Errorf("%w: project native file ownership could not be verified before removal", ErrInvalidRequest)
	}
	return nil
}

// Give the inspector the saved identity and complete native ownership, without
// sharing mutable state/history metadata with the frozen comparison basis.
func projectRemovalObservationBinding(binding domain.ClientBinding) domain.ClientBinding {
	return domain.ClientBinding{
		ClientBindingID: binding.ClientBindingID, ClientID: binding.ClientID, Scope: binding.Scope,
		TargetLocator: binding.TargetLocator, PhysicalArtifact: binding.PhysicalArtifact,
		NativeProfileRoot: binding.NativeProfileRoot, ProfileNamespace: binding.ProfileNamespace,
		ProfileAuthority:      domain.CloneProfileAuthority(binding.ProfileAuthority),
		LocalEntryObservation: binding.LocalEntryObservation.Clone(), SelectedDelivery: binding.SelectedDelivery,
		Materialization: binding.Materialization, Activation: binding.Activation,
		Authentication: binding.Authentication, Policy: binding.Policy, Verification: binding.Verification,
		NativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...),
	}
}
