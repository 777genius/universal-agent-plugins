package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func seedSavedAdd(snapshot environmentSnapshot, requirement environment.Requirement, pin environment.LockedPackage) environment.Lock {
	alias := requirement.Alias
	previous := snapshot.Lock
	previous.Packages = append([]environment.LockedPackage(nil), snapshot.Lock.Packages...)
	if pin.TreeDigest == "" {
		return previous
	}
	if !reflect.DeepEqual(snapshot.Requirements.Dependencies[alias], requirement) && pin.Source.Git != nil && requirement.Git != nil && requirement.Git.Selector.Kind == environment.SelectorCommit && pin.Source.Git.Commit != requirement.Git.Selector.Value {
		return previous
	}
	for i, pkg := range previous.Packages {
		if pkg.SourceKey == pin.SourceKey {
			previous.Packages[i] = pin
			return previous
		}
	}
	previous.Packages = append(previous.Packages, pin)

	return previous
}

func loadSavedAddPackage(ctx context.Context, app App, source string, targets []domain.ClientID, matches []string, requirement environment.Requirement) (loadedPackage, error) {
	var err error
	var loaded loadedPackage
	if matches != nil {
		snapshot, e := app.acquireDependencyGit(ctx, *requirement.Git, requirement.Git.Selector.Value, "")
		if e != nil {
			return loadedPackage{}, e
		}
		identity := githubIdentity(requirement.Git.Repository, requirement.Git.Selector.Value, requirement.Git.Path)
		snapshot.Source = domain.SourceIdentity{RequestedSource: identity, CanonicalSource: identity, Repository: requirement.Git.Repository, ResolvedRevision: requirement.Git.Selector.Value, PackageSubpath: requirement.Git.Path, SourceBindingHint: "uap-dependency-git"}
		_, rootErr := os.Stat(filepath.Join(snapshot.Root, "plugin.json"))
		loaded, err = app.loadStandardSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
		if requirement.Git.Path == "" && os.IsNotExist(rootErr) {
			paths, discoverErr := app.SourceAcquirer.DiscoverGitHubPackages(ctx, requirement.Git.Repository, requirement.Git.Selector.Value)
			if discoverErr != nil {
				return loadedPackage{}, discoverErr
			}
			return loadedPackage{}, fmt.Errorf("choose //path for a standard package; candidates: %v", paths)
		}
	} else {
		detected, detectErr := detectDependencyClients(ctx, app)
		if detectErr != nil {
			return loadedPackage{}, detectErr
		}
		loaded, err = app.acquireDirectory(ctx, source, withDetectedClients(packageResolutionRequest{StandardOnly: true, Operation: domain.DirectoryInstall, Targets: targets}, detected))
	}
	if err != nil {
		return loadedPackage{}, err
	}

	return loaded, nil
}

func matchSavedRegistry(lock environment.Lock, source, alias string) (environment.LockedPackage, error) {
	var candidates []environment.LockedPackage
	for _, pkg := range lock.Packages {
		if pkg.Source.Registry == nil {
			continue
		}
		r := pkg.Source.Registry
		if r.DistributionID == source {
			candidates = []environment.LockedPackage{pkg}
			break
		}
		if r.ProductID == source || path.Base(r.DistributionID) == source {
			candidates = append(candidates, pkg)
		}
	}
	if len(candidates) > 1 && alias != "" {
		for _, candidate := range candidates {
			if candidate.Alias == alias {
				candidates = []environment.LockedPackage{candidate}
				break
			}
		}
	}
	if len(candidates) > 1 {
		names := make([]string, 0, len(candidates))
		for _, pkg := range candidates {
			names = append(names, pkg.Alias)
		}
		sort.Strings(names)
		return environment.LockedPackage{}, fmt.Errorf("ambiguous saved source; aliases %v; use canonical owner/name or --alias", names)
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}
	return environment.LockedPackage{}, nil
}

func runSavedAdd(cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID, matches []string, activation, auth bool) error {
	snapshot, err := snapshotEnvironment(cmd, app)
	if err != nil {
		return err
	}
	preview, cleanup, err := dependencyPreviewApp(app, opts)
	if err != nil {
		return err
	}
	defer cleanup()
	app = preview
	requirements := cloneSavedRequirements(snapshot.Requirements)
	requirement, pin, err := resolveSavedAdd(cmd, app, opts, source, targets, matches, snapshot)
	if err != nil {
		return err
	}
	alias := requirement.Alias
	requirement.Alias = alias
	requirements.Dependencies[alias] = requirement
	for _, target := range targets {
		key := string(target)
		if !slices.Contains(requirements.Targets[key], alias) {
			requirements.Targets[key] = append(requirements.Targets[key], alias)
			sort.Strings(requirements.Targets[key])
		}
	}
	previous := seedSavedAdd(snapshot, requirement, pin)
	locked, err := (environment.Coordinator{EnvironmentID: app.environmentID(), Resolver: dependencyResolver{app: app, requirements: requirements, dryRun: opts.dryRun}}).Resolve(cmd.Context(), requirements, previous, "")
	if err != nil {
		return err
	}
	manifest, err := editEnvironmentIntent(snapshot, requirements)
	if err != nil {
		return err
	}
	var attest *environment.Attestation
	if activation || auth {
		if len(targets) != 1 {
			return errors.New("explicit lifecycle attestation requires exactly one target")
		}
		attest = &environment.Attestation{Alias: dependencyNodeAlias(requirements, locked, alias), ClientID: string(targets[0]), ActivationComplete: activation, AuthComplete: auth}
	}
	selectedTargets := make([]string, 0, len(targets))
	for _, target := range targets {
		selectedTargets = append(selectedTargets, string(target))
	}
	opts.environmentOutput = map[string]any{"selected_targets": selectedTargets, "saved": true, "alias": alias, "manifest_path": snapshot.ManifestPath, "lock_path": snapshot.LockPath, "intent_change": !reflect.DeepEqual(snapshot.Requirements, requirements)}
	return runSavedEnvironment(cmd, app, opts, snapshot, requirements, locked, manifest, []string{dependencyNodeAlias(requirements, locked, alias)}, attest, selectedTargets)
}
