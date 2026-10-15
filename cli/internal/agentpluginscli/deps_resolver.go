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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type dependencyResolved struct {
	locked   environment.LockedPackage
	loaded   loadedPackage
	snapshot domain.PackageSnapshot
}

type dependencyResolver struct {
	requirements environment.Requirements
	app          App
	dryRun       bool
	tempRoot     string
}

func readDependencyLock(path string, requirements environment.Requirements) (environment.Lock, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return environment.Lock{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var lock environment.Lock
	if err := decoder.Decode(&lock); err != nil {
		return environment.Lock{}, fmt.Errorf("decode uap.lock: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return environment.Lock{}, errors.New("uap.lock contains trailing JSON values")
	}
	canonicalizeRequirementsFromLock(&requirements, lock)
	if err := lock.Validate(requirements); err != nil {
		return environment.Lock{}, err
	}
	return lock, nil
}

func readCompatibleDependencyLock(path string, requirements environment.Requirements) (environment.Lock, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return environment.Lock{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var lock environment.Lock
	if err := decoder.Decode(&lock); err != nil {
		return environment.Lock{}, fmt.Errorf("decode uap.lock: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return environment.Lock{}, errors.New("uap.lock contains trailing JSON values")
	}
	canonicalizeRequirementsFromLock(&requirements, lock)
	if err := lock.ValidateCompatiblePins(requirements); err != nil {
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

func (resolver dependencyResolver) Resolve(ctx context.Context, requirement environment.Requirement, previous *environment.LockedPackage, update bool) (environment.LockedPackage, error) {
	if previous != nil && !update && sameRequestedSelector(requirement, *previous) {
		return *previous, nil
	}
	if requirement.Git != nil {
		return resolver.resolveGit(ctx, requirement, previous, update)
	}
	return resolver.resolveRegistry(ctx, requirement, previous)
}

func sameRequestedSelector(requirement environment.Requirement, previous environment.LockedPackage) bool {
	if requirement.Git != nil {
		return previous.Source.Git != nil && previous.Source.Git.Requested == requirement.Git.Selector
	}
	return previous.Source.Registry != nil && previous.Source.Registry.Requested == requirement.Registry.Selector
}

func (resolver dependencyResolver) resolveGit(ctx context.Context, requirement environment.Requirement, previous *environment.LockedPackage, update bool) (environment.LockedPackage, error) {
	source := requirement.Git
	commit := source.Selector.Value
	resolvedRef, defaultBranch := "", ""
	if source.Selector.Kind != environment.SelectorCommit {
		var resolution sourceacquisition.RefResolution
		var err error
		if named, ok := resolver.app.SourceAcquirer.(interface {
			ResolveGitHubSelector(context.Context, string, string) (sourceacquisition.RefResolution, error)
		}); ok {
			selector := source.Selector.Value
			if source.Selector.Kind == environment.SelectorDefault {
				selector = "default"
			}
			resolution, err = named.ResolveGitHubSelector(ctx, source.Repository, selector)
		} else {
			return environment.LockedPackage{}, errors.New("source acquirer cannot resolve Git refs")
		}
		if err != nil {
			return environment.LockedPackage{}, err
		}
		commit, resolvedRef, defaultBranch = resolution.Commit, resolution.ResolvedRef, resolution.DefaultBranch
	}
	identity := githubIdentity(source.Repository, commit, source.Path)
	var snapshot domain.PackageSnapshot
	var err error
	if previous != nil && previous.Source.Git != nil && previous.TreeDigest != "" && !update && sameRequestedSelector(requirement, *previous) {
		snapshot, err = resolver.app.SourceAcquirer.AcquireGitHubVerified(ctx, source.Repository, commit, source.Path, previous.TreeDigest)
	} else {
		snapshot, err = resolver.app.SourceAcquirer.AcquireGitHub(ctx, source.Repository, commit, source.Path)
	}
	if err != nil {
		return environment.LockedPackage{}, err
	}
	snapshot.Source = domain.SourceIdentity{RequestedSource: identity, CanonicalSource: identity, Repository: source.Repository,
		PackageSubpath: source.Path, ResolvedRevision: commit, SourceBindingHint: "uap-dependency-git"}
	loaded, err := resolver.app.loadSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	if _, err := directoryv1.ResolveDirectExact(loaded.envelope.Source); err != nil {
		_ = loaded.cleanup()
		return environment.LockedPackage{}, err
	}
	locked := environment.LockedPackage{
		Alias: requirement.Alias, SourceKey: mustSourceKey(requirement),
		DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: loaded.envelope.TreeDigest,
		ManifestDigest: loaded.envelope.ManifestDigest, ExecutableFiles: append([]string{}, loaded.envelope.ExecutableFiles...),
		Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: source.Repository, Path: source.Path,
			Requested: source.Selector, Commit: commit, ResolvedRef: resolvedRef, DefaultBranch: defaultBranch}},
	}
	return locked, resolver.finishLoaded(loaded)
}

func (resolver dependencyResolver) resolveRegistry(ctx context.Context, requirement environment.Requirement, previous *environment.LockedPackage) (environment.LockedPackage, error) {
	sequence, _ := parseReleaseSequence(requirement.Registry.Selector.Value)
	request := packageResolutionRequest{Operation: domain.DirectoryInstall, DesiredReleaseSequence: sequence}
	for target, aliases := range resolver.requirements.Targets {
		for _, alias := range aliases {
			if alias == requirement.Alias {
				request.Targets = append(request.Targets, domain.ClientID(target))
			}
		}
	}
	loaded, err := resolver.app.acquireDirectory(ctx, requirement.Registry.Distribution, request)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	if loaded.directorySelection == nil || loaded.directory == nil {
		_ = loaded.cleanup()
		return environment.LockedPackage{}, errors.New("Directory dependency resolved without publication identity")
	}
	selection := *loaded.directorySelection
	locked := environment.LockedPackage{
		Alias: requirement.Alias, SourceKey: mustSourceKey(requirement),
		DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: loaded.envelope.TreeDigest,
		ManifestDigest: loaded.envelope.ManifestDigest, ExecutableFiles: append([]string{}, loaded.envelope.ExecutableFiles...),
		Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{
			Distribution: requirement.Registry.Distribution, ProductID: selection.ProductID, DistributionID: selection.DistributionID,
			ReleaseSequence: selection.ReleaseSequence, Requested: requirement.Registry.Selector,
			Repository: selection.Source.Repository, Path: selection.Source.Path, Commit: selection.Source.Revision,
		}},
	}
	return locked, resolver.finishLoaded(loaded)
}

func (resolver dependencyResolver) finishLoaded(loaded loadedPackage) error {
	if !resolver.dryRun {
		snapshot := domain.PackageSnapshot{Root: loaded.envelope.SnapshotRoot, TreeDigest: loaded.envelope.TreeDigest,
			DigestAlgorithm: domain.TreeDigestAlgorithm, ExecutableFiles: append([]string{}, loaded.envelope.ExecutableFiles...),
			Source: loaded.envelope.Source}
		if _, err := resolver.app.dependencyStore().Put(context.Background(), snapshot); err != nil {
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
	changed := false
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
		var targets []domain.ClientID
		for target, aliases := range requirements.Targets {
			for _, item := range aliases {
				if item == alias {
					targets = append(targets, domain.ClientID(target))
					break
				}
			}
		}
		sequence, err := parseReleaseSequence(requirement.Registry.Selector.Value)
		if err != nil {
			return environment.Requirements{}, err
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
		changed = true
	}
	if changed {
		if err := requirements.Validate(); err != nil {
			return environment.Requirements{}, err
		}
	}
	return requirements, nil
}
