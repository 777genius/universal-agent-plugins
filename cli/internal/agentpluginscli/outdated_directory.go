package agentpluginscli

import (
	"context"
	"errors"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func inspectOutdatedInstallation(bundle directoryv1.VerifiedBundle, bundleOK bool, detected map[domain.ClientID]domain.DetectedClient, detectionErr error, installerVersion string, installation domain.Installation, scope string) outdatedInstallation {
	item := outdatedInstallation{InstallationID: installation.InstallationID, Name: installation.DeclaredName}
	if status, reason, stop := outdatedIdentityGate(installation); stop {
		item.Status, item.Reason = status, reason
		return item
	}
	return inspectDirectoryOutdated(bundle, bundleOK, detected, detectionErr, installerVersion, installation, scope, item)
}

func outdatedIdentityGate(installation domain.Installation) (status, reason string, stop bool) {
	if installation.NeedsRebind {
		return "blocked", "installation requires explicit rebind before update", true
	}
	if installation.Package.LoaderKind != domain.LoaderKindAgentPlugins {
		return "unmanaged", "legacy plugin.yaml installation has no Agent Plugins release identity", true
	}
	if installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil {
		return "unmanaged", "direct source has no signed update channel; use an explicit source switch", true
	}
	return "", "", false
}

func inspectDirectoryOutdated(
	bundle directoryv1.VerifiedBundle,
	bundleOK bool,
	detected map[domain.ClientID]domain.DetectedClient,
	detectionErr error,
	installerVersion string,
	installation domain.Installation,
	scope string,
	item outdatedInstallation,
) outdatedInstallation {
	origin := installation.Directory
	item.Targets = installationTargets(installation, scope)
	item.Installed = &outdatedReleaseIdentity{
		DistributionID: origin.DistributionID, ReleaseSequence: origin.DesiredReleaseSequence,
		PackageVersion: installation.Package.Version, Revision: installation.Source.ResolvedRevision,
		TreeDigest: installation.Source.TreeDigest, ManifestDigest: installation.Package.ManifestDigest,
	}
	if !bundleOK {
		item.Status, item.Reason = "unknown", "no authenticated Directory snapshot is available"
		return item
	}
	item.Warnings = installedDirectoryWarnings(bundle.Snapshot, installation)
	if status, reason, cloneInstalled, stop := directoryOutdatedPreflight(item, detectionErr, installation, scope); stop {
		item.Status, item.Reason = status, reason
		if cloneInstalled {
			item.Available = cloneOutdatedIdentity(item.Installed)
		}
		return item
	}
	return resolveDirectoryOutdated(bundle, detected, installerVersion, installation, origin, item, domain.InstallScope(scope))
}

func directoryOutdatedPreflight(item outdatedInstallation, detectionErr error, installation domain.Installation, scope string) (status, reason string, cloneInstalled, stop bool) {
	if detectionErr != nil {
		return "unknown", detectionErr.Error(), false, true
	}
	if len(item.Targets) == 0 {
		return "blocked", "installation has no materialized target in the selected scope", false, true
	}
	if selectedTargetsNeedDesiredRelease(installation, item.Targets, scope) {
		return "outdated", "one or more installed clients have not converged to the recorded desired release", true, true
	}
	return "", "", false, false
}

func resolveDirectoryOutdated(
	bundle directoryv1.VerifiedBundle,
	detected map[domain.ClientID]domain.DetectedClient,
	installerVersion string,
	installation domain.Installation,
	origin *domain.DirectoryOrigin,
	item outdatedInstallation,
	scopes ...domain.InstallScope,
) outdatedInstallation {
	_, clientMap, err := preflightSelectedTargets(context.Background(), App{Detector: staticDetectedClientSource(detected)}, item.Targets, detectedClientValues(detected), false)
	if err != nil {
		item.Status, item.Reason = "blocked", err.Error()
		return item
	}
	environment := directoryEnvironment(clientMap)
	environment.InstallerVersion = installerVersion
	selection, err := domain.ResolveDirectory(bundle.Snapshot, domain.DirectoryResolveRequest{
		Selector: origin.ProductID, Targets: expandAffectedSurfaceTargets(item.Targets, selectedScope(scopes)), Scope: selectedScope(scopes),
		InstallerVersion: installerVersion, ClientVersions: environment.ClientVersions, OS: environment.OS,
		Architecture: environment.Architecture, DependencyIdentity: environment.DependencyIdentity,
		SchemaVersion: "1.0.0", Operation: domain.DirectoryUpdate,
		Recorded: recordedDirectoryRelease(installation, origin.DesiredReleaseSequence, installation.Source.ResolvedRevision),
	})
	if err == nil {
		item.Status, item.Reason = "outdated", "a later eligible immutable release is available"
		item.Available = &outdatedReleaseIdentity{
			DistributionID: selection.DistributionID, ReleaseSequence: selection.ReleaseSequence,
			PackageVersion: selection.PackageVersion, Revision: selection.Source.Revision,
			TreeDigest: selection.TreeDigest, ManifestDigest: selection.ManifestDigest,
		}
		return item
	}
	return directoryOutdatedResolveError(err, item)
}

func directoryOutdatedResolveError(err error, item outdatedInstallation) outdatedInstallation {
	if errors.Is(err, domain.ErrDirectoryIneligible) && errors.Is(err, domain.ErrDirectoryNoSafeUpdate) {
		if len(item.Warnings) > 0 {
			item.Status, item.Reason = "blocked", "installed release is unsafe and no later eligible release is available"
		} else {
			item.Status, item.Reason = "current", "no later eligible immutable release exists"
		}
		return item
	}
	item.Status, item.Reason = "blocked", err.Error()
	return item
}

// staticDetectedClientSource is used only to reuse the exact client-surface
// preflight rules for a previously detected, read-only snapshot.
type staticDetectedClientSource map[domain.ClientID]domain.DetectedClient

func (source staticDetectedClientSource) Detect(context.Context) ([]domain.DetectedClient, error) {
	return detectedClientValues(source), nil
}

func cloneOutdatedIdentity(identity *outdatedReleaseIdentity) *outdatedReleaseIdentity {
	if identity == nil {
		return nil
	}
	clone := *identity
	return &clone
}
