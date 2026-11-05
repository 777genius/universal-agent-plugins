package agentpluginscli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func restoreCatalogEvidence(loaded *loadedPackage, binding domain.ClientBinding) {
	if loaded == nil || binding.PackageRevision == nil {
		return
	}
	// Directory acquisition has already composed compatibility evidence for the
	// currently detected installer, client, platform, and dependency tuple. The
	// recorded copy identifies what was applicable when this binding was last
	// written and must not replace that freshly filtered evidence during repair.
	if loaded.origin == domain.OriginModeDirectory && loaded.envelope.CatalogEvidence != nil {
		return
	}
	if binding.PackageRevision.CatalogEvidence == nil {
		loaded.envelope.CatalogEvidence = nil
		loaded.hints.Compatibility = nil
		return
	}
	evidence := *binding.PackageRevision.CatalogEvidence
	evidence.Compatibility = cloneCatalogCompatibility(evidence.Compatibility)
	loaded.envelope.CatalogEvidence = &evidence
	loaded.hints.Compatibility = cloneCatalogCompatibility(evidence.Compatibility)
}

func isDirectorySelector(value string) bool {
	if isShortName(value) {
		return true
	}
	parts := strings.Split(value, "/")
	return len(parts) == 2 && isShortName(parts[0]) && isShortName(parts[1])
}

func isShortName(value string) bool {
	return shortNamePattern.MatchString(value) && !strings.ContainsAny(value, `/\\`)
}

func explicitLocalPath(value string) bool {
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") || strings.HasPrefix(value, `.\`) || strings.HasPrefix(value, `..\`) {
		return true
	}
	// Classify syntax independently of the host OS, including Unix roots and
	// only genuinely absolute Windows spellings, so copied sources are stable;
	// drive-relative forms such as C:plugin intentionally remain selectors.
	if len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
		return true
	}
	if !strings.HasPrefix(value, `\\`) {
		return false
	}
	unc := strings.TrimPrefix(value, `\\`)
	server, share, ok := strings.Cut(unc, `\`)
	return ok && server != "" && strings.Trim(share, `\`) != ""
}

// selectDirectorySourceForPlacement separates signed source qualification from
// explicit local Project placement. A v1 user grant authenticates which package
// can be selected; it is NOT evidence of project-native runtime execution.
// Domain.ResolveDirectory deliberately keeps its strict publisher grant rules.
func selectDirectorySourceForPlacement(snapshot domain.DirectorySnapshot, request domain.DirectoryResolveRequest) (domain.DirectorySelection, error) {
	if snapshot.SnapshotSchemaVersion != 1 || request.Scope != domain.ScopeProject {
		return domain.ResolveDirectory(snapshot, request)
	}
	for _, client := range request.Targets {
		definition, ok := domain.ClientDefinitionFor(client)
		if !ok || !definition.ProjectPolicy.Valid() || !slices.Contains(definition.Capabilities.Scopes, domain.ScopeProject) {
			return domain.DirectorySelection{}, fmt.Errorf("%w: %s has no qualified project installation policy", domain.ErrDirectoryIneligible, client)
		}
	}
	// The signed v1 representation has no project grants. The existing
	// resolver still verifies its exact release, user publication policy,
	// minimum version, current revocations and trusted source evidence.
	// The scoped installer independently admits native Project components.
	request.Scope = domain.ScopeUser
	return domain.ResolveDirectory(snapshot, request)
}
