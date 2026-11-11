package agentpluginscli

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func installationHasTarget(installation domain.Installation, target domain.ClientID, scope string) bool {
	for _, binding := range installation.Clients {
		if binding.Scope == scope && binding.Materialization != domain.MaterializationAbsent && bindingAffectsTarget(binding, target) {
			return true
		}
	}
	return false
}

func installationTargets(installation domain.Installation, scope string) []domain.ClientID {
	seen := make(map[domain.ClientID]struct{}, len(installation.Clients))
	result := make([]domain.ClientID, 0, len(installation.Clients))
	for _, binding := range installation.Clients {
		if binding.Scope != scope || binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		for _, target := range bindingSurfaceTargets(binding) {
			if !supportedTarget(target) {
				continue
			}
			if _, duplicate := seen[target]; duplicate {
				continue
			}
			seen[target] = struct{}{}
			result = append(result, target)
		}
	}
	sortTargets(result)
	return result
}

func installedBindingTargets(installation domain.Installation, scope string) []domain.ClientID {
	seen := make(map[domain.ClientID]struct{}, len(installation.Clients))
	result := make([]domain.ClientID, 0, len(installation.Clients))
	for _, binding := range installation.Clients {
		target := domain.ClientID(binding.ClientID)
		if binding.Scope != scope || binding.Materialization == domain.MaterializationAbsent || !supportedTarget(target) {
			continue
		}
		if _, duplicate := seen[target]; duplicate {
			continue
		}
		seen[target] = struct{}{}
		result = append(result, target)
	}
	sortTargets(result)
	return result
}

// directoryBindingTargets retains all known affected clients except an
// implicitly shared sibling whose native owner has its own signed grant.
// Explicitly selected siblings are added by the caller and remain gated.
func directoryBindingTargets(binding domain.ClientBinding) []domain.ClientID {
	owner := domain.ClientID(binding.ClientID)
	definition, known := domain.ClientDefinitionFor(owner)
	var result []domain.ClientID
	for _, target := range bindingSurfaceTargets(binding) {
		if !supportedTarget(target) {
			continue
		}
		if known && definition.Traits.DirectoryOwnGrant && target != owner &&
			domain.SameClientBackendForScope(owner, target, domain.InstallScope(binding.Scope)) {
			continue
		}
		result = append(result, target)
	}
	return result
}

// updateSourceTargets preserves the physical compatibility set for local and
// Git sources. Signed Directory eligibility can separately authorize native
// owners without borrowing a grant for their implicit shared-backend sibling.
func updateSourceTargets(installation domain.Installation, scope string, selected []domain.ClientID) []domain.ClientID {
	if installation.OriginMode != domain.OriginModeDirectory {
		return installationTargets(installation, scope)
	}
	return append(directoryInstallationTargets(installation, scope), selected...)
}

func directoryInstallationTargets(installation domain.Installation, scope string) []domain.ClientID {
	var result []domain.ClientID
	seen := make(map[domain.ClientID]bool)
	for _, binding := range installation.Clients {
		if binding.Scope != scope || binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		for _, target := range directoryBindingTargets(binding) {
			if !seen[target] {
				seen[target] = true
				result = append(result, target)
			}
		}
	}
	sortTargets(result)
	return result
}

func bindingAffectsTarget(binding domain.ClientBinding, target domain.ClientID) bool {
	for _, surface := range bindingSurfaceTargets(binding) {
		if surface == target {
			return true
		}
	}
	return false
}

func bindingSurfaceTargets(binding domain.ClientBinding) []domain.ClientID {
	// The physical binding's own client identity is always an affected logical
	// surface, including when reading an older or partially written surface list.
	// Union it with the persisted list so target selection and compatibility
	// preflight can heal omissions instead of silently perpetuating them.
	values := append([]string(nil), binding.AffectedSurfaces...)
	values = append(values, binding.ClientID)
	values = appendBackendSiblings(values, domain.ClientID(binding.ClientID), domain.InstallScope(binding.Scope))
	result := make([]domain.ClientID, 0, len(values))
	seen := map[domain.ClientID]struct{}{}
	for _, value := range values {
		target := domain.ClientID(value)
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		result = append(result, target)
	}
	return result
}

func expandAffectedSurfaceTargets(targets []domain.ClientID, scopes ...domain.InstallScope) []domain.ClientID {
	result := expandBackendSiblingTargets(targets, scopes...)
	seen := make(map[domain.ClientID]struct{}, len(result))
	unique := result[:0]
	for _, target := range result {
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		unique = append(unique, target)
	}
	sortTargets(unique)
	return unique
}

func selectedTargetsNeedDesiredRelease(installation domain.Installation, targets []domain.ClientID, scope string) bool {
	if installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil {
		return false
	}
	desired := installation.Directory.DesiredReleaseSequence
	for _, target := range targets {
		if bindingNeedsDesiredRelease(installation, target, scope, desired) {
			return true
		}
	}
	return false
}

func bindingNeedsDesiredRelease(installation domain.Installation, target domain.ClientID, scope string, desired uint64) bool {
	for _, binding := range installation.Clients {
		if binding.Scope != scope || binding.Materialization == domain.MaterializationAbsent || !bindingAffectsTarget(binding, target) || binding.PackageRevision == nil {
			continue
		}
		if binding.PackageRevision.DistributionID == installation.Directory.DistributionID && binding.PackageRevision.ReleaseSequence < desired {
			return true
		}
	}
	return false
}
