package copilot

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// This exact positive observation is separate from the shared legacy grammars:
// a leading builtin section followed by exactly one managed live entry. A
// mismatch grants no finding and leaves each old caller's fallback intact.
func copilotUserLiveWithBuiltins(stdout, stderr []byte, plan domain.DeliveryPlan, expectedPath, executable string) bool {
	if len(stdout) > 8<<10 || len(stderr) != 0 || !copilotUserLivePlan(plan, expectedPath, executable) {
		return false
	}
	listing, ok := splitBuiltinListing(string(stdout))
	if !ok || !listing.leading || listing.lists(plan.DeclaredName) {
		return false
	}
	identity := plan.DeclaredName + "@" + shared.ManagedMarketplaceName(plan.PhysicalArtifactID)
	version := shared.MarketplaceVersion(plan.DeclaredVersion)
	entry := "  • " + identity + " (v" + version + ") (enabled)"
	live := shared.CopilotLiveHeader + "\n" + entry + "\n      from " + expectedPath + "\n"
	if listing.rest != live {
		return false
	}
	status, recognized := shared.CopilotLivePluginStatus([]byte(live), identity, version, expectedPath)
	return recognized && status == shared.CopilotStatusInstalled
}

func copilotUserLivePlan(plan domain.DeliveryPlan, expectedPath, executable string) bool {
	return copilotUserLiveScope(plan) && copilotUserLiveName(plan.DeclaredName) &&
		copilotUserLiveName(plan.PhysicalArtifactID) &&
		len(plan.DeclaredVersion) <= 256 && copilotUserLivePath(expectedPath) &&
		expectedPath == plan.ActivePath && filepath.Base(expectedPath) == plan.PhysicalArtifactID &&
		copilotUserLivePath(executable) && executable == plan.NativeRegistryExecutable
}

func copilotUserLiveScope(plan domain.DeliveryPlan) bool {
	return plan.ClientID == domain.ClientCopilot && plan.Scope == domain.ScopeUser &&
		(plan.TargetContext.Scope == "" || plan.TargetContext.Scope == domain.ScopeUser) &&
		plan.TargetContext.ProjectRoot == ""
}

func copilotUserLivePath(path string) bool {
	return len(path) > 1 && len(path) <= 4096 && filepath.IsAbs(path) &&
		filepath.Clean(path) == path && !strings.ContainsAny(path, "\x00\r\n")
}

func copilotUserLiveName(name string) bool {
	if name == "" || len(name) > 256 {
		return false
	}
	for index, ch := range name {
		alphanumeric := ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9'
		if !alphanumeric && (index == 0 || ch != '.' && ch != '_' && ch != '-') {
			return false
		}
	}
	return true
}

func copilotUserLiveManaged(plan domain.DeliveryPlan, managed *domain.ClientBinding) bool {
	return managed == nil || managed.ClientID == string(domain.ClientCopilot) &&
		managed.Scope == string(domain.ScopeUser) && managed.PhysicalArtifact == plan.PhysicalArtifactID &&
		managed.TargetLocator == plan.ActivePath
}

func copilotUserLiveActivation(stdout, stderr []byte, request domain.ActivationRequest) bool {
	return request.Client.ClientID == domain.ClientCopilot && request.Delivery.ClientID == domain.ClientCopilot &&
		request.DeclaredName == request.Plan.DeclaredName && request.BackendExecutable == request.Client.ExecutablePath &&
		request.Delivery.ActivePath == request.Plan.ActivePath &&
		copilotUserLiveWithBuiltins(stdout, stderr, request.Plan, request.Delivery.ActivePath, request.BackendExecutable)
}

func recognizeCopilotUserRegistry(listed legacyports.CommandResult, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, bool) {
	if copilotUserEmptyWithBuiltins(listed.Stdout, plan) {
		return clients.RegistryClear, true
	}
	if copilotOwnedInstalledRevision(listed, plan, managed) {
		return clients.RegistryExpected, true
	}
	expectedPath := plan.ActivePath
	if managed != nil && strings.TrimSpace(managed.TargetLocator) != "" {
		expectedPath = managed.TargetLocator
	}
	if !copilotUserLiveManaged(plan, managed) || !copilotUserLiveWithBuiltins(listed.Stdout, listed.Stderr, plan, expectedPath, plan.NativeRegistryExecutable) {
		return clients.RegistryIndeterminate, false
	}
	if managed == nil {
		return clients.RegistryCollision, true
	}
	return clients.RegistryExpected, true
}

func recognizeCopilotUserActivation(listed legacyports.CommandResult, request domain.ActivationRequest) bool {
	return copilotUserLiveActivation(listed.Stdout, listed.Stderr, request)
}

// Copilot appends its builtin section to an exact empty user registry. Builtins
// prove neither ownership nor absence for a same-name request, which is refused.
// Keep this observed shape separate from the generic Copilot/VS Code grammars.
func copilotUserEmptyWithBuiltins(stdout []byte, plan domain.DeliveryPlan) bool {
	const empty = "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\n"
	name := plan.DeclaredName
	if plan.ClientID != domain.ClientCopilot || plan.Scope != domain.ScopeUser ||
		plan.TargetContext.Scope == domain.ScopeProject || name == "" || name != strings.TrimSpace(name) {
		return false
	}
	listing, ok := splitBuiltinListing(string(stdout))
	return ok && !listing.leading && listing.rest == empty && !listing.lists(name)
}
