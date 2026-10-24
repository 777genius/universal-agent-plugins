package installer

import (
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Removal policy is queried directly on the captured registry adapter. Runtime
// profile guards cannot hide this optional capability behind an activator port.
func (e *Engine) removalRequiresExternalUninstalled(client domain.DetectedClient, installation domain.Installation, binding domain.ClientBinding, executable string) bool {
	policy, ok := clients.As[clients.RemovalAttestationPolicy](e.cfg.Registry, client.ClientID)
	if !ok {
		return false
	}
	return policy.RequiresExternalUninstalled(clients.Env{Runner: e.cfg.Runner}, domain.DeactivationRequest{
		Client: client, DeclaredName: installation.DeclaredName,
		CurrentActivation: binding.Activation, SelectedDelivery: binding.SelectedDelivery,
		LocalEntryObservation: binding.LocalEntryObservation.Clone(),
		RemoveOwnedEntry:      binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects),
		PhysicalArtifactID:    binding.PhysicalArtifact, BackendExecutable: executable,
		ManagedArtifactPath: binding.TargetLocator, NativeObjects: slices.Clone(binding.NativeObjects),
	})
}
