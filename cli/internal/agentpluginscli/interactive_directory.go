package agentpluginscli

import (
	"context"
	"fmt"
	"runtime"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type directoryTargetSession struct {
	app             App
	detected        []domain.DetectedClient
	intents         []map[domain.ClientID]domain.InstallIntent
	snapshot        domain.DirectorySnapshot
	resolveSelector string
	environment     directoryEvidenceEnvironment
	operation       domain.DirectoryOperation
	recorded        *domain.RecordedDirectoryRelease
}

func (app App) compatibleDirectoryTargets(ctx context.Context, selector string, detected []domain.DetectedClient, intents ...map[domain.ClientID]domain.InstallIntent) ([]domain.DetectedClient, error) {
	session, err := app.newDirectoryTargetSession(ctx, selector, detected, intents...)
	if err != nil {
		return nil, err
	}
	return session.largestCompatibleSubset(), nil
}

func (app App) newDirectoryTargetSession(ctx context.Context, selector string, detected []domain.DetectedClient, intents ...map[domain.ClientID]domain.InstallIntent) (*directoryTargetSession, error) {
	if app.DirectoryClient == nil || app.StateStore == nil {
		return nil, fmt.Errorf("signed Directory dependencies are unavailable; use a direct local or exact full-SHA source")
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return nil, err
	}
	bundle, err := app.DirectoryClient.Load(ctx, installedDirectoryFloor(state))
	if err != nil {
		return nil, fmt.Errorf("load signed Directory: %w", err)
	}
	request, err := retainDirectoryRelease(bundle.Snapshot, state, selector, app.addResolutionRequest(selector, nil))
	if err != nil {
		return nil, err
	}
	session := &directoryTargetSession{
		app: app, detected: detected, intents: intents, snapshot: bundle.Snapshot,
		resolveSelector: directoryResolveSelector(selector, request),
		environment:     directoryEnvironment(detectedClientMap(detected)),
		operation:       request.Operation, recorded: request.Recorded,
	}
	session.environment.InstallerVersion = app.Version
	if session.operation == "" {
		session.operation = domain.DirectoryInstall
	}
	session.assignContext7Intents()
	return session, nil
}

func directoryResolveSelector(selector string, request packageResolutionRequest) string {
	if request.Selector != "" {
		return request.Selector
	}
	return selector
}

func (session *directoryTargetSession) assignContext7Intents() {
	if productID, productErr := directorySelectorProductID(session.snapshot, session.resolveSelector); productErr == nil && productID == "context7" && len(session.intents) > 0 {
		if session.intents[0] == nil {
			session.intents[0] = make(map[domain.ClientID]domain.InstallIntent)
		}
		assignPrepareIntents(session.intents[0])
	}
}

func (session *directoryTargetSession) largestCompatibleSubset() []domain.DetectedClient {
	// At most ten clients are supported, so enumerating subsets is bounded to
	// 1,023 subsets, each with bounded pure resolver checks. This preserves one signed distribution/release
	// for the complete set instead of combining incompatible per-client picks.
	for size := len(session.detected); size >= 1; size-- {
		var match []domain.DetectedClient
		forEachDetectedSubset(session.detected, size, func(candidate []domain.DetectedClient) bool {
			if !session.subsetResolves(candidate) {
				return true
			}
			match = append([]domain.DetectedClient(nil), candidate...)
			return false
		})
		if len(match) > 0 {
			return match
		}
	}
	// No subset resolved. The caller reports each excluded client and stops
	// before acquisition; raw resolver diagnostics can contain source details.
	return nil
}

func (session *directoryTargetSession) subsetResolves(candidate []domain.DetectedClient) bool {
	targets := detectedClientIDs(candidate)
	resolveRequest := session.baseRequest(targets)
	_, resolveErr := domain.ResolveDirectory(session.snapshot, resolveRequest)
	if session.personalPrepareSelected() {
		if applied, prepErr := session.preparePersonal(resolveRequest, targets); applied {
			resolveErr = prepErr
		}
	}
	return resolveErr == nil
}

func (session *directoryTargetSession) baseRequest(targets []domain.ClientID) domain.DirectoryResolveRequest {
	return domain.DirectoryResolveRequest{
		Selector: session.resolveSelector, Targets: targets, Scope: domain.ScopeUser,
		InstallerVersion: session.app.Version, ClientVersions: session.environment.ClientVersions,
		OS: runtime.GOOS, Architecture: runtime.GOARCH, DependencyIdentity: session.environment.DependencyIdentity,
		SchemaVersion: "1.0.0", Operation: session.operation, Recorded: session.recorded,
	}
}

func (session *directoryTargetSession) personalPrepareSelected() bool {
	return len(session.intents) > 0 && personalMappingPrepareSelected(session.intents[0])
}

func (session *directoryTargetSession) preparePersonal(resolveRequest domain.DirectoryResolveRequest, targets []domain.ClientID) (bool, error) {
	hasPersonal := false
	var peers []domain.ClientID
	var personal domain.ClientID
	for _, target := range targets {
		if requiresPersonalMapping(target) {
			hasPersonal = true
			personal = target
		} else {
			peers = append(peers, target)
		}
	}
	if !hasPersonal {
		return false, nil
	}
	err := session.resolvePersonalPreparation(resolveRequest, personal, peers)
	if err == nil {
		session.intents[0][personal] = domain.InstallIntentPrepare
	}
	return true, err
}

func (session *directoryTargetSession) resolvePersonalPreparation(resolveRequest domain.DirectoryResolveRequest, personal domain.ClientID, peers []domain.ClientID) error {
	preparation := resolveRequest
	preparation.Targets = []domain.ClientID{personal}
	if _, purpose, ok := directoryPreparationClient([]domain.ClientID{personal}); ok {
		preparation.Purpose = purpose
	}
	selection, prepErr := domain.ResolveDirectory(session.snapshot, preparation)
	if prepErr != nil {
		return prepErr
	}
	if len(peers) == 0 {
		return nil
	}
	return session.resolvePreparationPeers(resolveRequest, selection, peers)
}

func (session *directoryTargetSession) resolvePreparationPeers(resolveRequest domain.DirectoryResolveRequest, selection domain.DirectorySelection, peers []domain.ClientID) error {
	peerRequest := resolveRequest
	peerRequest.Targets = peers
	peerRequest.Selector = selection.DistributionID
	peerRequest.Operation = domain.DirectoryNewTarget
	peerRequest.Recorded = &domain.RecordedDirectoryRelease{
		ProductID: selection.ProductID, DistributionID: selection.DistributionID, ReleaseSequence: selection.ReleaseSequence,
		Repository: selection.Source.Repository, ResolvedRevision: selection.Source.Revision, Path: selection.Source.Path,
		TreeDigestAlgorithm: selection.TreeDigestAlgorithm, TreeDigest: selection.TreeDigest, ManifestDigest: selection.ManifestDigest,
	}
	peer, err := domain.ResolveDirectory(session.snapshot, peerRequest)
	if err != nil {
		return err
	}
	if peer.DistributionID != selection.DistributionID || peer.ReleaseSequence != selection.ReleaseSequence || peer.TreeDigest != selection.TreeDigest {
		return fmt.Errorf("preparation peers require the same immutable release")
	}
	return nil
}

func forEachDetectedSubset(values []domain.DetectedClient, size int, visit func([]domain.DetectedClient) bool) {
	if size < 1 || size > len(values) {
		return
	}
	indices := make([]int, size)
	var walk func(int, int) bool
	walk = func(depth, start int) bool {
		if depth == size {
			candidate := make([]domain.DetectedClient, size)
			for index, valueIndex := range indices {
				candidate[index] = values[valueIndex]
			}
			return visit(candidate)
		}
		for index := start; index <= len(values)-(size-depth); index++ {
			indices[depth] = index
			if !walk(depth+1, index+1) {
				return false
			}
		}
		return true
	}
	walk(0, 0)
}
