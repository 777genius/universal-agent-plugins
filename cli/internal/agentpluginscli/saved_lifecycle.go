package agentpluginscli

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// resolveSavedAlias resolves intent and live ownership together. A name never
// silently selects an alias when an unrelated installation has the same name.
func resolveSavedAlias(state domain.StateFileV2, requirements environment.Requirements, selector string) (string, *domain.Installation, error) {
	aliases := matchingSavedAliases(state, requirements, selector)
	if len(aliases) == 0 {
		return "", nil, nil
	}
	if len(aliases) != 1 {
		return "", nil, ambiguousSavedSelector(selector)
	}
	var alias string
	for key := range aliases {
		alias = key
	}
	installation, err := savedAliasInstallation(state, alias, selector)
	return alias, installation, err
}

func ambiguousSavedSelector(selector string) error {
	return fmt.Errorf("ambiguous dependency %q; specify an installation ID", selector)
}

func matchingSavedAliases(state domain.StateFileV2, requirements environment.Requirements, selector string) map[string]bool {
	aliases := map[string]bool{}
	if requirement, ok := requirements.Dependencies[selector]; ok {
		source := mustSourceKey(requirement)
		for _, receipt := range state.EnvironmentReceipts {
			if source != "" && receipt.EnvironmentID == environment.EnvironmentUserDefault && receipt.SourceKey == source {
				aliases[receipt.Alias] = true
			}
		}
		if len(aliases) == 0 {
			aliases[selector] = true
		}
	}
	names := make(map[string]string, len(state.Installations))
	for _, installation := range state.Installations {
		names[installation.InstallationID] = installation.DeclaredName
	}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID != environment.EnvironmentUserDefault {
			continue
		}
		if receipt.Alias == selector || receipt.InstallationID == selector || names[receipt.InstallationID] == selector {
			aliases[receipt.Alias] = true
		}
	}
	return aliases
}

func ownsSavedAlias(state domain.StateFileV2, installationID, alias string) bool {
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID == environment.EnvironmentUserDefault && receipt.InstallationID == installationID && receipt.Alias == alias {
			return true
		}
	}
	return false
}

func savedAliasInstallation(state domain.StateFileV2, alias, selector string) (*domain.Installation, error) {
	var selected *domain.Installation
	for index := range state.Installations {
		installation := &state.Installations[index]
		owned := ownsSavedAlias(state, installation.InstallationID, alias)
		if (installation.InstallationID == selector || installation.DeclaredName == selector) && !owned {
			return nil, ambiguousSavedSelector(selector)
		}
		if !owned {
			continue
		}
		if selected != nil && selected.InstallationID != installation.InstallationID {
			return nil, ambiguousSavedSelector(selector)
		}
		selected = installation
	}
	return selected, nil
}

func savedAliasTargets(requirements environment.Requirements, state domain.StateFileV2, alias string) []string {
	declaration := alias
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID == environment.EnvironmentUserDefault && receipt.Alias == alias {
			for key, requirement := range requirements.Dependencies {
				if receipt.SourceKey != "" && mustSourceKey(requirement) == receipt.SourceKey {
					declaration = key
				}
			}
		}
	}
	targets := map[string]bool{}
	for target, aliases := range requirements.Targets {
		if slices.Contains(aliases, declaration) {
			targets[target] = true
		}
	}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID == environment.EnvironmentUserDefault && receipt.Alias == alias {
			targets[receipt.ClientID] = true
		}
	}
	result := make([]string, 0, len(targets))
	for target := range targets {
		result = append(result, target)
	}
	sort.Strings(result)
	return result
}

func removeSavedTargets(requirements environment.Requirements, alias string, targets []string) environment.Requirements {
	out := environment.Requirements{SchemaVersion: requirements.SchemaVersion, Dependencies: map[string]environment.Requirement{}, Targets: map[string][]string{}}
	for key, value := range requirements.Dependencies {
		out.Dependencies[key] = value
	}
	remaining := false
	for target, aliases := range requirements.Targets {
		kept := append([]string(nil), aliases...)
		if slices.Contains(targets, target) {
			kept = slices.DeleteFunc(kept, func(value string) bool { return value == alias })
		}
		if len(kept) > 0 {
			out.Targets[target] = kept
		}
		if slices.Contains(kept, alias) {
			remaining = true
		}
	}
	if !remaining {
		delete(out.Dependencies, alias)
	}
	return out
}

type savedLifecycleSelection struct {
	declaration  string
	alias        string
	installation *domain.Installation
	targets      []string
	retained     *environment.LockedPackage
}

// runSavedLifecycle precedes imperative target selection: desired targets can
// exist before the first installation does.
func runSavedLifecycle(cmd *cobra.Command, app App, opts *options, selector, operation string) (bool, error) {
	selection, err := selectSavedLifecycle(app, opts, selector)
	if err != nil {
		return true, err
	}
	if selection.alias == "" {
		return false, nil
	}
	if operation == "repair" {
		return selection.repair(cmd, app, opts)
	}
	if operation == "remove" && opts.purgeData {
		return true, errors.New("remove owned bindings before purging retained data; PLUGIN_DATA is preserved")
	}
	snapshot, err := snapshotEnvironment(cmd, app, operation != "remove")
	if err != nil {
		return true, err
	}
	if operation == "remove" && selection.retained != nil {
		return true, renderSavedRetentionResult(cmd, opts, *selection.retained)
	}
	preview, cleanup, err := dependencyPreviewApp(app, opts)
	if err != nil {
		return true, err
	}
	defer cleanup()
	requirements, locked, manifest, err := selection.candidate(cmd, preview, opts, snapshot, operation)
	if err != nil {
		return true, err
	}
	if err := runSavedEnvironment(cmd, preview, opts, snapshot, requirements, locked, manifest, []string{selection.alias}, nil); err != nil {
		return true, err
	}
	return true, selection.renderNextAction(cmd, opts, requirements, locked, operation)
}

func selectSavedLifecycle(app App, opts *options, selector string) (savedLifecycleSelection, error) {
	selection := savedLifecycleSelection{}
	state, err := app.StateStore.Load()
	if err != nil {
		return selection, err
	}
	manifest, _ := app.dependencyPaths()
	requirements, err := readDependencyManifest(manifest)
	if err != nil && !os.IsNotExist(err) {
		return selection, err
	}
	selection.alias, selection.installation, err = resolveSavedAlias(state, requirements, selector)
	if err != nil {
		return selection, err
	}
	selection, err = selectLockedLifecycle(app, state, requirements, selector, selection)
	if err != nil || selection.alias == "" {
		return selection, err
	}
	if opts.scope != "" && opts.scope != "user" {
		return selection, errors.New("saved dependencies support user scope only")
	}
	targets := savedAliasTargets(requirements, state, selection.alias)
	if selection.declaration != "" {
		targets = savedAliasTargets(requirements, state, selection.declaration)
	}
	if selection.retained != nil {
		targets = selection.retained.Targets
	}
	selection.targets, err = selectSavedTargets(opts.target, targets, selection.alias)
	return selection, err
}

func selectSavedTargets(option string, allTargets []string, alias string) ([]string, error) {
	if strings.TrimSpace(option) == "" {
		return append([]string(nil), allTargets...), nil
	}
	selected, err := parseTargetOption(option)
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(selected))
	for _, target := range selected {
		if !slices.Contains(allTargets, string(target)) {
			return nil, fmt.Errorf("target %q does not belong to dependency %q", target, alias)
		}
		targets = append(targets, string(target))
	}
	sort.Strings(targets)
	return targets, nil
}

func (selection savedLifecycleSelection) repair(cmd *cobra.Command, app App, opts *options) (bool, error) {
	if selection.installation == nil {
		return true, fmt.Errorf("dependency %q is not installed; run agentplugins deps sync", selection.alias)
	}
	targets := make([]domain.ClientID, 0, len(selection.targets))
	for _, target := range selection.targets {
		targets = append(targets, domain.ClientID(target))
	}
	return repairOwnedDependencies(cmd, app, opts, selection.installation.InstallationID, targets)
}

func (selection savedLifecycleSelection) candidate(cmd *cobra.Command, app App, opts *options, snapshot environmentSnapshot, operation string) (environment.Requirements, environment.Lock, []byte, error) {
	requirements := snapshot.Requirements
	var locked environment.Lock
	var err error
	switch operation {
	case "remove":
		requirements = removeSavedTargets(requirements, dependencyRootKey(requirements, snapshot.Lock, selection.alias), selection.targets)
		locked, err = (environment.Coordinator{Resolver: dependencyResolver{app: app, requirements: requirements, dryRun: opts.dryRun}}).Resolve(cmd.Context(), requirements, snapshot.Lock, "")
	case "update":
		requirements, locked, err = selection.update(cmd, app, opts, snapshot)
	default:
		err = fmt.Errorf("unsupported saved lifecycle operation %q", operation)
	}
	if err != nil {
		return requirements, locked, nil, err
	}
	manifest, err := editEnvironmentIntent(snapshot, requirements)
	return requirements, locked, manifest, err
}

func (selection savedLifecycleSelection) update(cmd *cobra.Command, app App, opts *options, snapshot environmentSnapshot) (environment.Requirements, environment.Lock, error) {
	requirements := snapshot.Requirements
	key := dependencyRootKey(requirements, snapshot.Lock, selection.alias)
	desired := savedAliasTargets(requirements, domain.StateFileV2{}, key)
	if strings.TrimSpace(opts.target) != "" && !slices.Equal(selection.targets, desired) {
		return requirements, snapshot.Lock, fmt.Errorf("update %s must select every target of the alias", selection.alias)
	}
	requirement, exists := requirements.Dependencies[key]
	if !exists {
		return requirements, snapshot.Lock, fmt.Errorf("dependency %q is absent from uap.toml; run agentplugins deps sync", selection.alias)
	}
	if requirement.Registry != nil && requirement.Registry.Selector.Kind == environment.SelectorVersion {
		return requirements, snapshot.Lock, nil
	}
	if requirement.Registry != nil && requirement.Registry.Selector.Kind != environment.SelectorRange {
		return updateSavedRegistry(cmd, app, opts, snapshot, selection.alias, selection.installation)
	}
	updateAlias := key
	if (requirement.Git != nil && requirement.Git.Selector.Kind == environment.SelectorCommit) || (requirement.Archive != nil && requirement.Archive.SHA256 != "") {
		updateAlias = ""
	}
	locked, err := (environment.Coordinator{Resolver: dependencyResolver{requirements: requirements, app: app, dryRun: opts.dryRun}}).Resolve(cmd.Context(), requirements, snapshot.Lock, updateAlias)
	return requirements, locked, err
}

func (selection savedLifecycleSelection) renderNextAction(cmd *cobra.Command, opts *options, requirements environment.Requirements, locked environment.Lock, operation string) error {
	if opts.format == "json" {
		return nil
	}
	if operation == "remove" {
		if pkg, exists := locked.Package(selection.alias); exists && len(pkg.RequiredBy) > 0 {
			return renderSavedRetention(cmd, pkg)
		}
	}
	requirement := requirements.Dependencies[dependencyRootKey(requirements, locked, selection.alias)]
	if operation == "update" && requirement.Registry != nil && requirement.Registry.Selector.Kind == environment.SelectorVersion {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "Exact version pin retained; choose a new version in uap.toml")
		return err
	}
	if operation == "update" && requirement.Git != nil && requirement.Git.Selector.Kind == environment.SelectorCommit {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "Exact commit pin retained; choose a new pin with agentplugins add <repo>@<sha>//path")
		return err
	}
	if operation == "update" && requirement.Archive != nil && requirement.Archive.SHA256 != "" {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "Exact archive checksum retained; choose a new checksum in uap.toml")
		return err
	}
	if operation == "remove" && selection.installation != nil && len(savedAliasTargets(requirements, domain.StateFileV2{}, selection.alias)) == 0 {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "PLUGIN_DATA preserved; after removal completes, run agentplugins remove %s --purge-data to delete retained data\n", selection.installation.InstallationID)
		return err
	}
	return nil
}

func updateSavedRegistry(cmd *cobra.Command, app App, opts *options, snapshot environmentSnapshot, alias string, installation *domain.Installation) (environment.Requirements, environment.Lock, error) {
	requirements := snapshot.Requirements
	old, ok := snapshot.Lock.Package(alias)
	if !ok || old.Source.Registry == nil {
		return requirements, snapshot.Lock, errors.New("registry dependency has no locked release")
	}
	request, err := savedRegistryUpdateRequest(cmd, app, old.Targets, installation)
	if err != nil {
		return requirements, snapshot.Lock, err
	}
	loaded, err := app.acquireDirectory(cmd.Context(), old.Source.Registry.DistributionID, request)
	if errors.Is(err, domain.ErrDirectoryNoSafeUpdate) {
		return requirements, snapshot.Lock, nil
	}
	if err != nil {
		return requirements, snapshot.Lock, err
	}
	err = validateSavedRegistryUpdate(loaded, old.Source.Registry)
	if err != nil || loaded.directorySelection.ReleaseSequence == old.Source.Registry.ReleaseSequence {
		if loaded.cleanup != nil {
			_ = loaded.cleanup()
		}
		return requirements, snapshot.Lock, err
	}
	requirements, candidate := savedRegistryCandidate(snapshot, alias, old, loaded)
	next, _ := candidate.Package(alias)
	if err := (dependencyResolver{app: app, dryRun: opts.dryRun}).finishLoaded(cmd.Context(), loaded, next); err != nil {
		return requirements, snapshot.Lock, err
	}
	// Validate the authoritative old graph before rebuilding edges from the new
	// immutable manifests. Replacing a node inside old edges creates a false conflict.
	locked, err := (environment.Coordinator{Resolver: dependencyResolver{app: app, dryRun: opts.dryRun, requirements: requirements}}).Resolve(cmd.Context(), requirements, snapshot.Lock, "")
	return requirements, locked, err
}

func savedRegistryUpdateRequest(cmd *cobra.Command, app App, targets []string, installation *domain.Installation) (packageResolutionRequest, error) {
	request := packageResolutionRequest{StandardOnly: true, Operation: domain.DirectoryInstall}
	detected, err := detectDependencyClients(cmd.Context(), app)
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
