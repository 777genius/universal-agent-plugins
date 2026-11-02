package agentpluginscli

import (
	"context"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (app App) acquireDirectory(ctx context.Context, selector string, request packageResolutionRequest) (loadedPackage, error) {
	if app.DirectoryClient == nil || app.SourceAcquirer == nil {
		return loadedPackage{}, fmt.Errorf("signed Directory dependencies are unavailable; use a direct local or exact full-SHA source")
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return loadedPackage{}, err
	}
	bundle, err := app.DirectoryClient.Load(ctx, installedDirectoryFloor(state))
	if err != nil {
		return loadedPackage{}, fmt.Errorf("load signed Directory: %w", err)
	}
	request, err = retainDirectoryRelease(bundle.Snapshot, state, selector, request)
	if err != nil {
		return loadedPackage{}, err
	}
	operation := request.Operation
	if operation == "" {
		operation = domain.DirectoryInstall
	}
	environment := directoryEnvironment(request.Clients)
	environment.InstallerVersion = app.Version
	resolveSelector := selector
	if request.Selector != "" {
		resolveSelector = request.Selector
	}
	productID, _ := directorySelectorProductID(bundle.Snapshot, resolveSelector)
	affectedTargets := expandAffectedSurfaceTargets(request.Targets, app.scope())
	resolveRequest := domain.DirectoryResolveRequest{
		Selector: resolveSelector, Targets: affectedTargets, Scope: app.scope(),
		InstallerVersion: app.Version, ClientVersions: environment.ClientVersions, OS: environment.OS, Architecture: environment.Architecture,
		DependencyIdentity: environment.DependencyIdentity,
		SchemaVersion:      "1.0.0", Operation: operation, Recorded: request.Recorded,
		DesiredReleaseSequence: request.DesiredReleaseSequence,
	}
	preparingChatGPT := false
	var retainedMapping *domain.ChatGPTLocalMapping
	installation, _ := locallyMatchedInstallation(state, resolveSelector)
	intents := lifecycleInstallIntents(installation, string(app.scope()), nil)
	for _, target := range affectedTargets {
		if requiresPersonalMapping(target) && (productID == "context7" || app.chatGPTPreparation || intents[target] == domain.InstallIntentPrepare) {
			preparingChatGPT = true
		}
	}
	if preparingChatGPT {
		if id, purpose, ok := directoryPreparationClient(affectedTargets); ok {
			resolveRequest.Purpose = purpose
			resolveRequest.Targets = []domain.ClientID{id}
		}
		retainedMapping = installation.LocalChatGPTMapping
	}
	selection, err := domain.ResolveDirectory(bundle.Snapshot, resolveRequest)
	if err != nil {
		return loadedPackage{}, err
	}
	product, distribution, release, policy := directoryRecords(bundle.Snapshot, selection)
	if product == nil || distribution == nil || release == nil || policy == nil {
		return loadedPackage{}, fmt.Errorf("signed Directory selection cannot be reproduced from its snapshot")
	}
	if err := validateDirectoryCompatibilityPolicy(*policy); err != nil {
		return loadedPackage{}, err
	}
	snapshot, err := app.SourceAcquirer.AcquireGitHubVerified(ctx, selection.Source.Repository, selection.Source.Revision, selection.Source.Path, selection.TreeDigest)
	if err != nil {
		return loadedPackage{}, err
	}
	snapshot.Source.RequestedSource = selector
	if request.RequestedSelector != "" {
		snapshot.Source.RequestedSource = request.RequestedSelector
	}
	snapshot.Source.SourceBindingHint = "directory-v1"
	origin := &domain.DirectoryOrigin{
		ProductID: selection.ProductID, DistributionID: selection.DistributionID, DistributionKind: selection.DistributionKind,
		DesiredReleaseSequence: selection.ReleaseSequence, SnapshotSchema: directoryv1.SnapshotSchemaVersion,
		SnapshotSequence: selection.SnapshotSequence, SnapshotDigest: bundle.Digest,
	}
	load := app.loadSnapshot
	if request.StandardOnly {
		load = app.loadStandardSnapshot
	}
	loaded, err := load(ctx, snapshot, domain.OriginModeDirectory, origin, &selection)
	if err != nil {
		return loadedPackage{}, err
	}
	if loaded.envelope.ManifestDigest != selection.ManifestDigest {
		_ = loaded.cleanup()
		return loadedPackage{}, fmt.Errorf("acquired package manifest digest %s does not match signed Directory digest %s", loaded.envelope.ManifestDigest, selection.ManifestDigest)
	}
	if loaded.envelope.Manifest.Name != product.ManifestName || loaded.envelope.Manifest.Name != release.ManifestName {
		_ = loaded.cleanup()
		return loadedPackage{}, fmt.Errorf("signed Directory identity does not match acquired package manifest")
	}
	environment.DependencyIdentity = deterministicDependencyIdentity(loaded.envelope, affectedTargets)
	exactRequest := resolveRequest
	exactRequest.DesiredReleaseSequence = 0
	exactRequest.Selector = selection.DistributionID
	exactRequest.Operation = domain.DirectoryNewTarget
	exactRequest.Recorded = &domain.RecordedDirectoryRelease{
		ProductID: selection.ProductID, DistributionID: selection.DistributionID, ReleaseSequence: selection.ReleaseSequence,
		Repository: selection.Source.Repository, ResolvedRevision: selection.Source.Revision, Path: selection.Source.Path,
		TreeDigestAlgorithm: selection.TreeDigestAlgorithm, TreeDigest: selection.TreeDigest, ManifestDigest: selection.ManifestDigest,
	}
	exactRequest.DependencyIdentity = environment.DependencyIdentity
	checked, err := domain.ResolveDirectory(bundle.Snapshot, exactRequest)
	if err != nil || checked.DistributionID != selection.DistributionID || checked.ReleaseSequence != selection.ReleaseSequence {
		_ = loaded.cleanup()
		if err != nil {
			return loadedPackage{}, fmt.Errorf("acquired exact Directory release failed compatibility recheck: %w", err)
		}
		return loadedPackage{}, fmt.Errorf("acquired exact Directory release changed during compatibility recheck")
	}
	// The preparation resolver intentionally accepts exactly ChatGPT. Compose
	// mixed requests by checking peers against its exact immutable selection with
	// normal signed eligibility; never acquire another target's substitute package.
	if preparingChatGPT {
		var peers []domain.ClientID
		for _, target := range affectedTargets {
			if !requiresPersonalMapping(target) {
				peers = append(peers, target)
			}
		}
		if len(peers) > 0 {
			peerRequest := exactRequest
			peerRequest.Purpose = ""
			peerRequest.Targets = peers
			peer, peerErr := domain.ResolveDirectory(bundle.Snapshot, peerRequest)
			if peerErr != nil || peer.DistributionID != selection.DistributionID || peer.ReleaseSequence != selection.ReleaseSequence || peer.TreeDigest != selection.TreeDigest {
				_ = loaded.cleanup()
				return loadedPackage{}, fmt.Errorf("mixed preparation peers must qualify the same immutable Context7 release: %v", peerErr)
			}
		}
		if err := domain.ValidateContext7PreparationPackage(loaded.envelope); err != nil {
			_ = loaded.cleanup()
			return loadedPackage{}, err
		}
		if loaded.envelope.App.Present || loaded.envelope.App.Declared {
			_ = loaded.cleanup()
			return loadedPackage{}, fmt.Errorf("refuse personal registration for a publisher-mapped package")
		}
		loaded.chatGPTPreparation = true
		loaded.localChatGPTMapping = retainedMapping
		if retainedMapping != nil && retainedMapping.IsLegacyContext7Registration() {
			// The 0.1.56 receipt came from guidance that cannot produce a working
			// ChatGPT registration. Require a new explicit app ID to migrate it.
			loaded.localChatGPTMapping = nil
		}
		if app.chatGPTAppID != "" {
			mapping := domain.ChatGPTLocalMapping{ProductID: "context7", Repository: "upstash/context7", PackagePath: "plugins/agent-plugins/context7", Server: "context7", URL: domain.Context7ChatGPTURL, AppID: app.chatGPTAppID}
			if retainedMapping != nil && *retainedMapping != mapping && !retainedMapping.IsLegacyContext7Registration() {
				_ = loaded.cleanup()
				return loadedPackage{}, fmt.Errorf("explicit ChatGPT ID conflicts with retained personal registration receipt")
			}
			loaded.localChatGPTMapping = &mapping
		}
		if loaded.localChatGPTMapping != nil {
			if err := loaded.localChatGPTMapping.ValidatePackage(loaded.envelope); err != nil {
				_ = loaded.cleanup()
				return loadedPackage{}, err
			}
		}
	}
	loaded.distributionSuspended = distribution.Status == domain.DistributionSuspended
	loaded.releaseRevoked = policy.Status == domain.ReleaseRevoked || snapshotRevokes(bundle.Snapshot, selection)
	if err := applyDirectoryCompatibility(&loaded, bundle, selection, *policy, environment); err != nil {
		_ = loaded.cleanup()
		return loadedPackage{}, err
	}
	return loaded, nil
}
