package agentpluginscli

import (
	"context"
	"errors"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func updateSavedRegistry(cmd *cobra.Command, app App, opts *options, snapshot environmentSnapshot, alias string, installation *domain.Installation) (environment.Requirements, environment.Lock, error) {
	requirements, changed, err := prepareSavedRegistryUpdate(cmd.Context(), app, opts.dryRun, snapshot, alias, installation)
	if err != nil || !changed {
		return requirements, snapshot.Lock, err
	}
	// Validate the authoritative old graph before rebuilding the new release's edges.
	locked, err := (environment.Coordinator{Resolver: dependencyResolver{app: app, dryRun: opts.dryRun, requirements: requirements}}).Resolve(cmd.Context(), requirements, snapshot.Lock, "")
	return requirements, locked, err
}

// prepareSavedRegistryUpdate retains one signed release without resolving a graph.
// CLI updates and bulk management updates both use this exact policy boundary;
// the latter resolves all selected declarations once after acquiring candidates.
func prepareSavedRegistryUpdate(ctx context.Context, app App, dryRun bool, snapshot environmentSnapshot, alias string, installation *domain.Installation) (environment.Requirements, bool, error) {
	requirements := snapshot.Requirements
	old, ok := snapshot.Lock.Package(alias)
	if !ok || old.Source.Registry == nil {
		return requirements, false, errors.New("registry dependency has no locked release")
	}
	request, err := savedRegistryUpdateRequest(ctx, app, old.Targets, installation)
	if err != nil {
		return requirements, false, err
	}
	loaded, err := app.acquireDirectory(ctx, old.Source.Registry.DistributionID, request)
	if errors.Is(err, domain.ErrDirectoryNoSafeUpdate) {
		return requirements, false, nil
	}
	if err != nil {
		return requirements, false, err
	}
	err = validateSavedRegistryUpdate(loaded, old.Source.Registry)
	if err != nil || loaded.directorySelection.ReleaseSequence == old.Source.Registry.ReleaseSequence {
		if loaded.cleanup != nil {
			_ = loaded.cleanup()
		}
		return requirements, false, err
	}
	requirements, candidate := savedRegistryCandidate(snapshot, alias, old, loaded)
	next, _ := candidate.Package(alias)
	if err := (dependencyResolver{app: app, dryRun: dryRun}).finishLoaded(ctx, loaded, next); err != nil {
		return requirements, false, err
	}
	return requirements, true, nil
}

func savedRegistryUpdateRequest(ctx context.Context, app App, targets []string, installation *domain.Installation) (packageResolutionRequest, error) {
	request := packageResolutionRequest{StandardOnly: true, Operation: domain.DirectoryInstall}
	detected, err := detectDependencyClients(ctx, app)
	if err != nil {
		return request, err
	}
	request.Clients = detected
	for _, target := range targets {
		request.Targets = append(request.Targets, domain.ClientID(target))
	}
	if installation == nil {
		return request, nil
	}
	if installation.Directory == nil {
		return request, errors.New("owned registry installation lacks publication provenance")
	}
	request.Operation = domain.DirectoryUpdate
	request.Recorded = recordedDirectoryRelease(*installation, installation.Directory.DesiredReleaseSequence, installation.Source.ResolvedRevision)
	return request, nil
}

func validateSavedRegistryUpdate(loaded loadedPackage, prior *environment.LockedRegistrySource) error {
	if loaded.directorySelection == nil || loaded.directory == nil {
		return errors.New("registry update lacks publication identity")
	}
	if loaded.directorySelection.ProductID != prior.ProductID || loaded.directorySelection.DistributionID != prior.DistributionID {
		return errors.New("registry update changed product or distribution identity")
	}
	return nil
}

func savedRegistryCandidate(snapshot environmentSnapshot, alias string, old environment.LockedPackage, loaded loadedPackage) (environment.Requirements, environment.Lock) {
	requirements := cloneSavedRequirements(snapshot.Requirements)
	key := dependencyRootKey(requirements, snapshot.Lock, alias)
	requirement := requirements.Dependencies[key]
	registry := *requirement.Registry
	selection := loaded.directorySelection
	registry.Selector = environment.Selector{Kind: environment.SelectorRelease, Value: strconv.FormatUint(selection.ReleaseSequence, 10)}
	requirement.Registry = &registry
	requirements.Dependencies[key] = requirement
	next := old
	next.Version = loaded.envelope.Manifest.Version
	next.TreeDigest = loaded.envelope.TreeDigest
	next.ManifestDigest = loaded.envelope.ManifestDigest
	next.ExecutableFiles = append([]string{}, loaded.envelope.ExecutableFiles...)
	next.Source.Registry = &environment.LockedRegistrySource{Distribution: registry.Distribution, ProductID: selection.ProductID, DistributionID: selection.DistributionID, ReleaseSequence: selection.ReleaseSequence, Requested: registry.Selector, Repository: selection.Source.Repository, Path: selection.Source.Path, Commit: selection.Source.Revision}
	locked := snapshot.Lock
	locked.Packages = append([]environment.LockedPackage(nil), locked.Packages...)
	for index := range locked.Packages {
		if locked.Packages[index].Alias == alias {
			locked.Packages[index] = next
		}
	}
	locked.ManifestDigest, _ = environment.ManifestDigest(requirements)
	return requirements, locked
}
