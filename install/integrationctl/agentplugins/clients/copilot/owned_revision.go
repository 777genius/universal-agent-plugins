package copilot

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// copilotOwnedInstalledRevision recognizes an owned user plugin that Copilot
// still lists at its installed revision while an update plans a new version.
// The shared listing check compares against the planned version only, so every
// update that changed the package version stopped at identity preflight with
// an indeterminate finding. Ownership still requires the exact managed
// marketplace identity, enabled state and install path; the owned package
// digest is verified separately by the caller.
func copilotOwnedInstalledRevision(listed legacyports.CommandResult, plan domain.DeliveryPlan, managed *domain.ClientBinding) bool {
	if managed == nil || managed.PackageRevision == nil || !copilotUserLiveScope(plan) ||
		managed.ClientID != string(domain.ClientCopilot) || managed.Scope != string(domain.ScopeUser) ||
		managed.PhysicalArtifact != plan.PhysicalArtifactID || managed.TargetLocator == "" ||
		managed.TargetLocator != plan.ActivePath {
		return false
	}
	installed := strings.TrimSpace(managed.PackageRevision.Version)
	if installed == "" || installed == strings.TrimSpace(plan.DeclaredVersion) {
		return false
	}
	stdout := listed.Stdout
	if listing, ok := splitBuiltinListing(string(stdout)); ok {
		if listing.lists(plan.DeclaredName) {
			return false
		}
		stdout = []byte(listing.rest)
	}
	return shared.CopilotRegistryFindingAt(stdout, plan.DeclaredName, shared.ManagedMarketplaceName(plan.PhysicalArtifactID),
		shared.MarketplaceVersion(installed), managed.TargetLocator, true) == clients.RegistryExpected
}
