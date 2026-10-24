package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

type dependencyResolver struct {
	requirements environment.Requirements
	app          App
	dryRun       bool
}

func readDependencyLock(path string, requirements environment.Requirements) (environment.Lock, error) {
	return readValidatedDependencyLock(path, requirements, false)
}
func readCompatibleDependencyLock(path string, requirements environment.Requirements) (environment.Lock, error) {
	return readValidatedDependencyLock(path, requirements, true)
}
func readValidatedDependencyLock(path string, requirements environment.Requirements, compatible bool) (environment.Lock, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return environment.Lock{}, err
	}
	return decodeDependencyLockBytes(body, requirements, compatible)
}

func decodeDependencyLockBytes(body []byte, requirements environment.Requirements, compatible bool) (environment.Lock, error) {
	var err error
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var lock environment.Lock
	if err := decoder.Decode(&lock); err != nil {
		return environment.Lock{}, fmt.Errorf("decode uap.lock: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return environment.Lock{}, errors.New("uap.lock contains trailing JSON values")
	}
	if err := canonicalizeRequirementsFromLock(&requirements, lock); err != nil {
		return environment.Lock{}, err
	}
	if compatible {
		err = lock.ValidateCompatiblePins(requirements)
	} else {
		err = lock.Validate(requirements)
	}
	if err != nil {
		return environment.Lock{}, err
	}
	return lock, nil
}

func writeDependencyLock(path string, lock environment.Lock) error {
	body, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(path, append(body, '\n'), 0o600)
}

func (resolver dependencyResolver) Resolve(ctx context.Context, requirement environment.Requirement, targets []string, previous *environment.LockedPackage, update bool) (environment.LockedPackage, error) {
	if previous != nil && !update && sameRequestedSelector(requirement, *previous) {
		return *previous, nil
	}
	if requirement.Git != nil {
		return resolver.resolveGit(ctx, requirement, targets, previous, update)
	}
	if requirement.Archive != nil {
		return resolver.resolveArchive(ctx, requirement, targets)
	}
	return resolver.resolveRegistry(ctx, requirement, targets)
}

func sameRequestedSelector(requirement environment.Requirement, previous environment.LockedPackage) bool {
	return requirement.RequestedSelector() == previous.Requirement().RequestedSelector() && mustSourceKey(requirement) == previous.SourceKey
}

func (resolver dependencyResolver) resolveGit(ctx context.Context, requirement environment.Requirement, targets []string, previous *environment.LockedPackage, update bool) (environment.LockedPackage, error) {
	source := requirement.Git
	resolution, err := resolver.resolveGitSelector(ctx, source)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	commit, resolvedRef, defaultBranch := resolution.Commit, resolution.ResolvedRef, resolution.DefaultBranch
	if source.Kind == environment.SkillKind {
		return resolver.resolveSkill(ctx, requirement, targets, previous, commit, resolvedRef, defaultBranch)
	}
	snapshot, err := resolver.app.acquireDependencyGit(ctx, *source, commit, "")
	if err != nil {
		return environment.LockedPackage{}, err
	}
	sourcePin := environment.LockedPackage{Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: source.Repository, URL: source.URL, Path: source.Path, Commit: commit}}}
	snapshot.Source = identityFromLocked(sourcePin)
	loaded, err := resolver.app.loadStandardSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	locked := environment.LockedPackage{
		Alias: requirement.Alias, Targets: append([]string{}, targets...), SourceKey: mustSourceKey(requirement), Version: loaded.envelope.Manifest.Version,
		DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: loaded.envelope.TreeDigest,
		ManifestDigest: loaded.envelope.ManifestDigest, ExecutableFiles: append([]string{}, loaded.envelope.ExecutableFiles...),
		Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: source.Repository, URL: source.URL, Path: source.Path,
			Requested: source.Selector, Commit: commit, ResolvedRef: resolvedRef, DefaultBranch: defaultBranch}},
	}
	if strings.Contains(locked.Alias, "/") || locked.Alias == "" {
		locked.Alias = dependencyManifestAlias(loaded.envelope.Manifest.Name)
	}
	return locked, resolver.finishLoaded(ctx, loaded, locked)
}

func (resolver dependencyResolver) resolveArchive(ctx context.Context, requirement environment.Requirement, targets []string) (environment.LockedPackage, error) {
	source := requirement.Archive
	direct, err := resolver.app.directAcquirer()
	if err != nil {
		return environment.LockedPackage{}, err
	}
	snapshot, err := direct.AcquireArchive(ctx, sourcespec.Archive{URL: source.URL, Path: source.Path}, source.SHA256, "")
	if err != nil {
		return environment.LockedPackage{}, err
	}
	loaded, err := resolver.app.loadStandardSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	locked := environment.LockedPackage{Alias: requirement.Alias, Targets: append([]string{}, targets...),
		SourceKey: mustSourceKey(requirement), Version: loaded.envelope.Manifest.Version, DigestAlgorithm: domain.TreeDigestAlgorithm,
		TreeDigest: loaded.envelope.TreeDigest, ManifestDigest: loaded.envelope.ManifestDigest, ExecutableFiles: append([]string{}, loaded.envelope.ExecutableFiles...),
		Source: environment.LockedSource{Archive: &environment.LockedArchiveSource{URL: source.URL, Path: loaded.envelope.Source.PackageSubpath,
			RequestedSHA256: source.SHA256, SHA256: loaded.envelope.Source.ArchiveSHA256}}}
	if locked.Source.Archive.Path != source.Path {
		_ = loaded.cleanup()
		return environment.LockedPackage{}, errors.New("archive requirement must explicitly select its logical package path")
	}
	if strings.Contains(locked.Alias, "/") || locked.Alias == "" {
		locked.Alias = dependencyManifestAlias(loaded.envelope.Manifest.Name)
	}
	return locked, resolver.finishLoaded(ctx, loaded, locked)
}

func (resolver dependencyResolver) resolveGitSelector(ctx context.Context, source *environment.GitSource) (sourceacquisition.RefResolution, error) {
	if source.Selector.Kind == environment.SelectorCommit {
		return sourceacquisition.RefResolution{Commit: source.Selector.Value}, nil
	}
	return resolver.app.resolveDependencyGit(ctx, *source)
}

func (resolver dependencyResolver) resolveRegistry(ctx context.Context, requirement environment.Requirement, targets []string) (environment.LockedPackage, error) {
	sequence, _ := parseReleaseSequence(requirement.Registry.Selector.Value)
	request := packageResolutionRequest{StandardOnly: true, Operation: domain.DirectoryInstall, DesiredReleaseSequence: sequence}
	for _, target := range targets {
		request.Targets = append(request.Targets, domain.ClientID(target))
	}
	loaded, err := resolver.app.acquireDirectory(ctx, requirement.Registry.Distribution, request)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	if loaded.directorySelection == nil || loaded.directory == nil {
		_ = loaded.cleanup()
		return environment.LockedPackage{}, errors.New("directory dependency resolved without publication identity")
	}
	locked, err := registryLockedPackage(requirement, loaded)
	if err != nil {
		_ = loaded.cleanup()
		return environment.LockedPackage{}, err
	}
	if strings.Contains(locked.Alias, "/") || locked.Alias == "" {
		locked.Alias = dependencyManifestAlias(loaded.envelope.Manifest.Name)
	}
	return locked, resolver.finishLoaded(ctx, loaded, locked)
}

func registryLockedPackage(requirement environment.Requirement, loaded loadedPackage) (environment.LockedPackage, error) {
	if loaded.directorySelection == nil || loaded.directory == nil {
		return environment.LockedPackage{}, errors.New("directory dependency resolved without publication identity")
	}
	selection := *loaded.directorySelection
	locked := environment.LockedPackage{
		Alias: requirement.Alias, SourceKey: mustSourceKey(requirement), Version: loaded.envelope.Manifest.Version,
		DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: loaded.envelope.TreeDigest,
		ManifestDigest: loaded.envelope.ManifestDigest, ExecutableFiles: append([]string{}, loaded.envelope.ExecutableFiles...),
		Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{
			Distribution: requirement.Registry.Distribution, ProductID: selection.ProductID, DistributionID: selection.DistributionID,
			ReleaseSequence: selection.ReleaseSequence, Requested: requirement.Registry.Selector,
			Repository: selection.Source.Repository, Path: selection.Source.Path, Commit: selection.Source.Revision,
		}},
	}
	return locked, nil
}

func (resolver dependencyResolver) finishLoaded(ctx context.Context, loaded loadedPackage, pins ...environment.LockedPackage) error {
	if err := ctx.Err(); err != nil {
		_ = loaded.cleanup()
		return err
	}
	if _, err := packageDependencyFacts(loaded.envelope); err != nil {
		_ = loaded.cleanup()
		return err
	}
	if len(pins) > 0 && resolver.app.dependencySnapshots != nil {
		pin := pins[0]
		snapshot, err := (packagedigest.Builder{}).SnapshotWithExecutables(ctx, loaded.envelope.SnapshotRoot, identityFromLocked(pin), loaded.envelope.ExecutableFiles)
		if err != nil {
			_ = loaded.cleanup()
			return err
		}
		key := dependencySnapshotKey{sourceKey: pin.SourceKey, revision: pin.Revision(), algorithm: pin.DigestAlgorithm, digest: pin.TreeDigest}
		cache := resolver.app.dependencySnapshots
		if old, ok := cache.snapshots[key]; ok {
			_ = packagedigest.Remove(old)
		}
		cache.snapshots[key] = snapshot
		return loaded.cleanup()
	}
	if !resolver.dryRun {
		snapshot := domain.PackageSnapshot{Root: loaded.envelope.SnapshotRoot, TreeDigest: loaded.envelope.TreeDigest,
			DigestAlgorithm: domain.TreeDigestAlgorithm, ExecutableFiles: append([]string{}, loaded.envelope.ExecutableFiles...),
			Source: loaded.envelope.Source}
		if _, err := resolver.app.dependencyStore().Put(ctx, snapshot); err != nil {
			_ = loaded.cleanup()
			return err
		}
	}
	return loaded.cleanup()
}

func githubIdentity(repository, commit, packagePath string) string {
	value := "github:" + repository + "@" + commit
	if packagePath != "" {
		value += "//" + packagePath
	}
	return value
}

func mustSourceKey(requirement environment.Requirement) string {
	// A fresh transitive acquisition has no owner alias until manifest loading.
	// This copy only permits strict alias-free source-tuple derivation; the
	// temporary scalar is never installed or persisted as binding identity.
	if requirement.Alias == "" {
		requirement.Alias = "source-acquisition"
	}
	key, _ := requirement.SourceKey()
	return key
}

func parseReleaseSequence(value string) (uint64, error) {
	var sequence uint64
	if _, err := fmt.Sscanf(value, "%d", &sequence); err != nil || sequence == 0 || fmt.Sprint(sequence) != value {
		return 0, fmt.Errorf("release %q is not a canonical sequence", value)
	}
	return sequence, nil
}

func (app App) dependencyStore() environmentstore.Store {
	root := strings.TrimSpace(app.EnvironmentStoreRoot)
	if root == "" {
		root = filepath.Join(filepath.Dir(app.ManagedRoot), "store")
	}
	return environmentstore.Store{Root: root, LockPath: filepath.Join(root, "store.lock")}
}

func (app App) canonicalizeDependencyRequirements(ctx context.Context, requirements environment.Requirements) (environment.Requirements, error) {
	for alias, requirement := range requirements.Dependencies {
		if requirement.Registry == nil || strings.Contains(requirement.Registry.Distribution, "/") {
			continue
		}
		if app.DirectoryClient == nil {
			return environment.Requirements{}, errors.New("signed Directory client is required to canonicalize short registry names")
		}
		bundle, err := app.DirectoryClient.Load(ctx, 0)
		if err != nil {
			return environment.Requirements{}, err
		}
		targets := dependencyRequirementTargets(requirements, alias)
		sequence := uint64(0)
		if requirement.Registry.Selector.Kind == environment.SelectorRelease {
			var err error
			sequence, err = parseReleaseSequence(requirement.Registry.Selector.Value)
			if err != nil {
				return environment.Requirements{}, err
			}
		}

		selection, err := domain.ResolveDirectory(bundle.Snapshot, domain.DirectoryResolveRequest{
			Selector: requirement.Registry.Distribution, DesiredReleaseSequence: sequence, Targets: targets, Scope: domain.ScopeUser,
			Operation: domain.DirectoryInstall, SchemaVersion: "1.0.0", InstallerVersion: app.Version,
		})
		if err != nil {
			return environment.Requirements{}, fmt.Errorf("canonicalize registry dependency %s: %w", alias, err)
		}
		requirement.Registry.Distribution = selection.DistributionID
		requirements.Dependencies[alias] = requirement
	}
	if err := requirements.Validate(); err != nil {
		return environment.Requirements{}, err
	}
	return requirements, nil
}

func dependencyRequirementTargets(requirements environment.Requirements, alias string) []domain.ClientID {
	targets := []domain.ClientID{}
	for target, aliases := range requirements.Targets {
		for _, item := range aliases {
			if item == alias {
				targets = append(targets, domain.ClientID(target))
				break
			}
		}
	}
	return targets
}
