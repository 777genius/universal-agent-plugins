package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type dependencySnapshotKey struct {
	repository, commit, path, algorithm, digest                  string
	kind, skill, requestedPath, sourceKey, rawDigest, normalizer string
}

func lockedDependencyCoordinates(locked environment.LockedPackage) (string, string, string, error) {
	if source := locked.Source.Git; source != nil {
		if source.Kind == environment.SkillKind {
			return source.Repository, source.Commit, source.ResolvedSkillPath, nil
		}
		return source.Repository, source.Commit, source.Path, nil
	}
	if source := locked.Source.Registry; source != nil {
		return source.Repository, source.Commit, source.Path, nil
	}
	return "", "", "", errors.New("locked package has no source")
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
	repository, commit, path, err := lockedDependencyCoordinates(locked)
	if err != nil {
		return dependencySnapshotKey{}, err
	}
	key := dependencySnapshotKey{repository: repository, commit: commit, path: path, algorithm: locked.DigestAlgorithm, digest: locked.TreeDigest}
	if source := locked.Source.Git; source != nil && source.Kind == environment.SkillKind {
		key.kind, key.skill, key.requestedPath = source.Kind, source.Skill, source.Path
		key.sourceKey, key.rawDigest, key.normalizer = locked.SourceKey, source.RawTreeDigest, source.Normalizer
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
			acquired, err = app.SourceAcquirer.AcquireGitHubVerified(ctx, key.repository, key.commit, key.path, key.digest)
		}
		if err != nil {
			return domain.PackageSnapshot{}, err
		}
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
	snapshot, err := (packagedigest.Builder{}).SnapshotWithExecutables(ctx, acquired.Root, acquired.Source, acquired.ExecutableFiles)
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
	repository, commit, subpath, err := lockedDependencyCoordinates(locked)
	if err != nil {
		return domain.SourceIdentity{}, err
	}
	if locked.Source.Git != nil && locked.Source.Git.Kind == environment.SkillKind {
		return skillSnapshotSource(locked), nil
	}
	identity := githubIdentity(repository, commit, subpath)
	return domain.SourceIdentity{RequestedSource: identity, CanonicalSource: identity, Repository: repository, PackageSubpath: subpath, ResolvedRevision: commit, SourceBindingHint: dependencySourceHint(locked)}, nil
}
