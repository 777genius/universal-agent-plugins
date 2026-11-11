package agentpluginscli

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func materializeLockedPackage(ctx context.Context, app App, locked environment.LockedPackage, publish bool, recorded *domain.RecordedDirectoryRelease, operation ...domain.DirectoryOperation) (loadedPackage, error) {
	if err := requireLockedDirectoryFeed(app, locked); err != nil {
		return loadedPackage{}, err
	}
	if source := locked.Source.Git; source != nil && source.Kind == environment.SkillKind && source.Normalizer != environment.SkillNormalizerV1 {
		return loadedPackage{}, errors.New("unsupported locked skill normalizer")
	}
	store := app.dependencyStore()
	if snapshot, err := store.Snapshot(ctx, locked.DigestAlgorithm, locked.TreeDigest); err == nil {
		return loadStoredDependencySnapshot(ctx, app, snapshot, locked, recorded, operation...)
	} else if !errors.Is(err, environmentstore.ErrMissing) {
		return loadedPackage{}, err
	}
	snapshot, err := app.acquireLockedSnapshot(ctx, locked)
	if err != nil {
		return loadedPackage{}, err
	}
	snapshot.Source = identityFromLocked(locked)
	origin, directory := lockedPackageOrigin(locked)
	loaded, err := app.loadStandardSnapshot(ctx, snapshot, origin, directory, nil)
	if err != nil {
		return loadedPackage{}, err
	}
	if err := validateDependencyPinEnvelope(locked, loaded.envelope); err != nil {
		if cleanupErr := loaded.cleanup(); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("%w: %w", errDependencySnapshotCleanup, cleanupErr))
		}
		return loadedPackage{}, err
	}
	checked, err := verifyLockedDirectory(ctx, app, loaded, locked, recorded, operation...)
	if err != nil {
		if cleanupErr := loaded.cleanup(); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("%w: %w", errDependencySnapshotCleanup, cleanupErr))
		}
		return loadedPackage{}, err
	}
	loaded = checked
	if publish {
		sealed := domain.PackageSnapshot{Root: loaded.envelope.SnapshotRoot, TreeDigest: loaded.envelope.TreeDigest,
			DigestAlgorithm: locked.DigestAlgorithm, ExecutableFiles: canonicalDependencyExecutables(loaded.envelope.ExecutableFiles), Source: loaded.envelope.Source}
		_, err = store.Put(ctx, sealed)
	} else {
		return loaded, nil
	}
	cleanupErr := loaded.cleanup()
	if err != nil {
		return loadedPackage{}, err
	}
	if cleanupErr != nil {
		return loadedPackage{}, cleanupErr
	}
	snapshot, err = store.Snapshot(ctx, locked.DigestAlgorithm, locked.TreeDigest)
	if err != nil {
		return loadedPackage{}, err
	}
	return loadStoredDependencySnapshot(ctx, app, snapshot, locked, recorded, operation...)
}

// lockedPackageOrigin carries the authenticated feed lineage to both freshly
// acquired and cache-restored dependency snapshots.
func lockedPackageOrigin(locked environment.LockedPackage) (domain.OriginMode, *domain.DirectoryOrigin) {
	if source := locked.Source.Registry; source != nil {
		return domain.OriginModeDirectory, &domain.DirectoryOrigin{
			ProductID: source.ProductID, DistributionID: source.DistributionID,
			FeedID: source.FeedID, DistributionKind: domain.DistributionCommunity,
			DesiredReleaseSequence: source.ReleaseSequence,
		}
	}
	return domain.OriginModeDirect, nil
}
