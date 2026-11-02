package agentpluginscli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type dependencySnapshotKey struct {
	sourceKey, revision, algorithm, digest            string
	kind, skill, requestedPath, rawDigest, normalizer string
}

// The command owns these acquired bytes until its final preflight/apply returns.
// Each caller gets a fresh verified snapshot and performs its own loading,
// scanning and Directory policy checks; eligibility is never cached here.
type dependencySnapshotCache struct {
	snapshots map[dependencySnapshotKey]domain.PackageSnapshot
}

func (cache *dependencySnapshotCache) close() {
	for key, snapshot := range cache.snapshots {
		_ = packagedigest.Remove(snapshot)
		delete(cache.snapshots, key)
	}
}

func lockedDependencySnapshotKey(locked environment.LockedPackage) (dependencySnapshotKey, error) {
	if locked.Source.Git == nil && locked.Source.Registry == nil && locked.Source.Archive == nil && locked.Source.Web == nil {
		return dependencySnapshotKey{}, errors.New("locked package has no source")
	}
	sourceKey := locked.SourceKey
	if sourceKey == "" {
		sourceKey = identityFromLocked(locked).CanonicalSource
	}
	key := dependencySnapshotKey{sourceKey: sourceKey, revision: locked.Revision(), algorithm: locked.DigestAlgorithm, digest: locked.TreeDigest}
	if source := locked.Source.Git; source != nil && source.Kind == environment.SkillKind {
		key.kind, key.skill, key.requestedPath = source.Kind, source.Skill, source.Path
		key.rawDigest, key.normalizer = source.RawTreeDigest, source.Normalizer
	}
	if source := locked.Source.Web; source != nil {
		key.kind, key.skill = "https-skill", source.Skill
		key.rawDigest, key.normalizer = source.RawTreeDigest, source.Normalizer
		body, err := json.Marshal(source)
		if err != nil {
			return dependencySnapshotKey{}, err
		}
		digest := sha256.Sum256(body)
		key.requestedPath = hex.EncodeToString(digest[:])
	}
	return key, nil
}

func (app App) acquireLockedSnapshot(ctx context.Context, locked environment.LockedPackage) (domain.PackageSnapshot, error) {
	key, err := lockedDependencySnapshotKey(locked)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	cache := app.dependencySnapshots
	var acquired domain.PackageSnapshot
	var ok bool
	if cache != nil {
		acquired, ok = cache.snapshots[key]
	}
	if !ok {
		var err error
		if locked.Source.Git != nil && locked.Source.Git.Kind == environment.SkillKind {
			acquired, err = app.acquireLockedSkill(ctx, locked)
		} else {
			acquired, err = app.acquirePinnedSource(ctx, locked)
		}
		if err != nil {
			return domain.PackageSnapshot{}, err
		}
		acquired.Source = identityFromLocked(locked)
		if cache == nil {
			return acquired, nil
		}
		if acquired.TreeDigest != key.digest || acquired.DigestAlgorithm != key.algorithm {
			_ = packagedigest.Remove(acquired)
			return domain.PackageSnapshot{}, errors.New("immutable fetch does not match uap.lock")
		}
		cache.snapshots[key] = acquired
	}
	// A consumer may release its snapshot, while the command retains the raw
	// acquisition. Recompute the digest rather than trusting retained metadata.
	snapshot, err := (packagedigest.Builder{}).SnapshotWithExecutables(ctx, acquired.Root, identityFromLocked(locked), acquired.ExecutableFiles)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	if snapshot.TreeDigest != key.digest || snapshot.DigestAlgorithm != key.algorithm {
		_ = packagedigest.Remove(snapshot)
		return domain.PackageSnapshot{}, errors.New("staged dependency no longer matches uap.lock")
	}
	return snapshot, nil
}

func lockedDependencySource(locked environment.LockedPackage) (domain.SourceIdentity, error) {
	if locked.Source.Git == nil && locked.Source.Registry == nil && locked.Source.Archive == nil && locked.Source.Web == nil {
		return domain.SourceIdentity{}, errors.New("locked package has no source")
	}
	return identityFromLocked(locked), nil
}
