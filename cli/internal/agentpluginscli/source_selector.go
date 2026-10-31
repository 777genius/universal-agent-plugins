package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (app App) addResolutionRequest(_ string, targets []domain.ClientID) packageResolutionRequest {
	return packageResolutionRequest{Targets: append([]domain.ClientID(nil), targets...), Operation: domain.DirectoryInstall, RetainRecorded: true}
}

func (app App) loadInstalledPackage(ctx context.Context, installation domain.Installation, targets []domain.ClientID, operation domain.DirectoryOperation, releaseSequence uint64, resolvedRevision string, clients map[domain.ClientID]domain.DetectedClient) (loadedPackage, error) {
	if installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil {
		binding := installation.Source
		return app.loadPackageFor(ctx, updateSource(installation), packageResolutionRequest{Targets: targets, Operation: operation, Clients: clients, DirectBinding: &binding})
	}
	sequence := releaseSequence
	if sequence == 0 {
		sequence = installation.Directory.DesiredReleaseSequence
	}
	if releaseSequence == 0 && strings.TrimSpace(resolvedRevision) == "" {
		resolvedRevision = installation.Source.ResolvedRevision
	}
	return app.loadPackageFor(ctx, installation.Directory.ProductID, packageResolutionRequest{Targets: targets, Operation: operation, Clients: clients,
		RequestedSelector: strings.TrimSpace(installation.Source.RequestedSource), Recorded: recordedDirectoryRelease(installation, sequence, resolvedRevision)})
}

func recordedDirectoryRelease(installation domain.Installation, sequence uint64, resolvedRevision string) *domain.RecordedDirectoryRelease {
	recorded := &domain.RecordedDirectoryRelease{
		ProductID: installation.Directory.ProductID, DistributionID: installation.Directory.DistributionID,
		ReleaseSequence: sequence, ResolvedRevision: strings.TrimSpace(resolvedRevision),
	}
	// Installation-level source and package provenance describe the current
	// desired release. A per-client revision may describe an older partial
	// rollout and retains only the immutable fields present on that binding.
	if sequence == installation.Directory.DesiredReleaseSequence && recorded.ResolvedRevision == installation.Source.ResolvedRevision {
		recorded.Repository = installation.Source.Repository
		recorded.Path = installation.Source.PackageSubpath
		recorded.TreeDigestAlgorithm = domain.TreeDigestAlgorithm
		recorded.TreeDigest = installation.Source.TreeDigest
		recorded.ManifestDigest = installation.Package.ManifestDigest
		return recorded
	}
	for _, binding := range installation.Clients {
		revision := binding.PackageRevision
		if revision == nil || revision.ReleaseSequence != sequence || revision.ResolvedRevision != recorded.ResolvedRevision {
			continue
		}
		recorded.TreeDigestAlgorithm = domain.TreeDigestAlgorithm
		recorded.TreeDigest = revision.TreeDigest
		recorded.ManifestDigest = revision.ManifestDigest
		break
	}
	return recorded
}

func withDetectedClients(request packageResolutionRequest, clients map[domain.ClientID]domain.DetectedClient) packageResolutionRequest {
	request.Clients = clients
	return request
}

func locallyMatchedInstallation(state domain.StateFileV2, selector string) (domain.Installation, bool) {
	selector = strings.TrimSpace(selector)
	var matches []domain.Installation
	for _, installation := range state.Installations {
		matched := installationMatchesSelector(installation, selector)
		if matched {
			matches = append(matches, installation)
		}
	}
	if len(matches) != 1 {
		return domain.Installation{}, false
	}
	return matches[0], true
}

// retainDirectoryRelease resolves mutable Directory selectors only far enough
// to establish product identity, then binds a re-add to the installation's
// recorded immutable release. Selecting today's default first would let an
// alias rotation change the distribution before local state was considered.
func retainDirectoryRelease(snapshot domain.DirectorySnapshot, state domain.StateFileV2, selector string, request packageResolutionRequest) (packageResolutionRequest, error) {
	if request.Recorded != nil || !request.RetainRecorded {
		return request, nil
	}
	productID, selectorErr := directorySelectorProductID(snapshot, selector)
	if selectorErr != nil && !errors.Is(selectorErr, domain.ErrDirectoryNotFound) {
		return request, selectorErr
	}
	installation, found, err := retainedDirectoryInstallation(state, selector, productID)
	if err != nil || !found {
		if selectorErr != nil {
			return request, selectorErr
		}
		return request, err
	}
	request.Operation = domain.DirectoryNewTarget
	request.Recorded = &domain.RecordedDirectoryRelease{
		ProductID:           installation.Directory.ProductID,
		DistributionID:      installation.Directory.DistributionID,
		ReleaseSequence:     installation.Directory.DesiredReleaseSequence,
		Repository:          installation.Source.Repository,
		ResolvedRevision:    installation.Source.ResolvedRevision,
		Path:                installation.Source.PackageSubpath,
		TreeDigestAlgorithm: domain.TreeDigestAlgorithm,
		TreeDigest:          installation.Source.TreeDigest,
		ManifestDigest:      installation.Package.ManifestDigest,
	}
	// Resolve the recorded qualified distribution, not a mutable or retired
	// alias. The recorded product identity is checked above before this rewrite.
	request.Selector = installation.Directory.DistributionID
	return request, nil
}

func directorySelectorProductID(snapshot domain.DirectorySnapshot, rawSelector string) (string, error) {
	selector := strings.TrimSpace(rawSelector)
	if strings.Contains(selector, "/") {
		matches := []string{}
		for _, distribution := range snapshot.Distributions {
			if distribution.ID != selector {
				continue
			}
			for _, product := range snapshot.Products {
				if product.ID == distribution.ProductID && containsDirectoryValue(product.Distributions, selector) {
					matches = append(matches, product.ID)
				}
			}
		}
		if len(matches) == 0 {
			return "", fmt.Errorf("%w: distribution %q", domain.ErrDirectoryNotFound, selector)
		}
		if len(matches) != 1 {
			return "", fmt.Errorf("%w: qualified distribution %q", domain.ErrDirectoryAmbiguous, selector)
		}
		return matches[0], nil
	}
	matches := []string{}
	for _, product := range snapshot.Products {
		if product.ID == selector || product.ManifestName == selector || containsDirectoryValue(product.Aliases, selector) {
			matches = append(matches, product.ID)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("%w: %q", domain.ErrDirectoryNotFound, selector)
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("%w: %q", domain.ErrDirectoryAmbiguous, selector)
	}
	return matches[0], nil
}

func containsDirectoryValue(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func retainedDirectoryInstallation(state domain.StateFileV2, selector, productID string) (domain.Installation, bool, error) {
	matches := []domain.Installation{}
	for _, installation := range state.Installations {
		directoryMatch := installation.Directory != nil && ((productID != "" && installation.Directory.ProductID == productID) || installation.Directory.DistributionID == selector)
		historicalSelectorMatch := strings.TrimSpace(installation.Source.RequestedSource) == selector
		declaredMatch := installation.InstallationID == selector || installation.DeclaredName == selector
		if !directoryMatch && !historicalSelectorMatch && !declaredMatch {
			continue
		}
		if installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil ||
			strings.TrimSpace(installation.Directory.ProductID) == "" || strings.TrimSpace(installation.Directory.DistributionID) == "" || installation.Directory.DesiredReleaseSequence < 1 {
			return domain.Installation{}, false, fmt.Errorf("retained installation %q has incomplete or corrupt Directory release identity", installation.InstallationID)
		}
		if productID != "" && installation.Directory.ProductID != productID {
			return domain.Installation{}, false, fmt.Errorf("directory selector %q now resolves to product %q, but retained installation %q records product %q", selector, productID, installation.InstallationID, installation.Directory.ProductID)
		}
		matches = append(matches, installation)
	}
	if len(matches) == 0 {
		return domain.Installation{}, false, nil
	}
	if len(matches) != 1 {
		return domain.Installation{}, false, fmt.Errorf("retained Directory state for product %q is ambiguous; use installation_id", productID)
	}
	return matches[0], true, nil
}

func installationMatchesSelector(installation domain.Installation, selector string) bool {
	if installation.InstallationID == selector || installation.DeclaredName == selector ||
		strings.TrimSpace(installation.Source.RequestedSource) == selector {
		return true
	}
	if installation.Directory == nil {
		return false
	}
	return installation.Directory.ProductID == selector || installation.Directory.DistributionID == selector
}
