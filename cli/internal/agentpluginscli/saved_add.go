package agentpluginscli

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func routeSavedAdd(cmd *cobra.Command, app App, opts *options, source string, activation, auth bool) (bool, error) {
	reason := ""
	switch {
	case opts.noSave:
		reason = "no_save_requested"
	case localArchiveInput(source) || explicitLocalPath(source):
		reason = "local_source_not_portable"
	case strings.HasPrefix(source, "discovery:"):
		reason = "discovery_source_not_saved"
	}
	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return true, err
	}
	if len(targets) == 1 && containsPersonalMappingTarget(targets) && reason == "" {
		reason = "chatgpt_preparation_not_saved"
	}
	if reason != "" {
		if opts.alias != "" {
			return true, errors.New("--alias requires a saved remote dependency")
		}
		opts.environmentReason = reason
		if err := rejectKnownOwnedAdd(app, source); err != nil {
			return true, err
		}
		return false, nil
	}
	matches := exactGitPattern.FindStringSubmatch(source)
	if matches == nil && !isDirectorySelector(source) {
		if _, err := sourcespec.Parse(source); err != nil {
			return true, err
		}
	}
	targets, err = savedAddTargets(cmd, app, opts, source, targets)
	if err != nil {
		return true, err
	}
	if (activation || auth) && len(targets) != 1 {
		return true, errors.New("explicit lifecycle attestation requires exactly one target")
	}
	return true, runSavedAdd(cmd, app, opts, source, targets, matches, activation, auth)
}

func cloneSavedRequirements(value environment.Requirements) environment.Requirements {
	cloned := environment.Requirements{SchemaVersion: value.SchemaVersion, Dependencies: map[string]environment.Requirement{}, Targets: map[string][]string{}}
	for alias, item := range value.Dependencies {
		cloned.Dependencies[alias] = item
	}
	for target, aliases := range value.Targets {
		cloned.Targets[target] = append([]string(nil), aliases...)
	}
	return cloned
}

func resolveSavedAdd(cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID, matches []string, snapshot environmentSnapshot) (environment.Requirement, environment.LockedPackage, error) {
	if matches == nil && !isDirectorySelector(source) {
		return resolveDirectSavedAdd(cmd, app, opts, source, snapshot)
	}
	var requirement environment.Requirement
	alias := ""
	var pin environment.LockedPackage
	if matches != nil {
		requirement = environment.Requirement{Alias: "candidate", Git: &environment.GitSource{Repository: strings.ToLower(matches[1]), Path: matches[3], Selector: environment.Selector{Kind: environment.SelectorCommit, Value: matches[2]}}}
		key := mustSourceKey(requirement)
		for _, pkg := range snapshot.Lock.Packages {
			if pkg.SourceKey == key {
				alias = dependencyRootKey(snapshot.Requirements, snapshot.Lock, pkg.Alias)
				pin = pkg
				break
			}
		}
	} else {
		var err error
		pin, err = matchSavedRegistry(snapshot.Lock, source, opts.alias)
		if err != nil {
			return environment.Requirement{}, environment.LockedPackage{}, err
		}
		alias = dependencyRootKey(snapshot.Requirements, snapshot.Lock, pin.Alias)
		if alias != "" {
			requirement = snapshot.Requirements.Dependencies[alias]
			if requirement.Registry == nil && pin.Source.Registry != nil {
				source := pin.Source.Registry
				requirement = environment.Requirement{Alias: alias, Registry: &environment.RegistrySource{
					Distribution: source.DistributionID,
					Selector:     environment.Selector{Kind: environment.SelectorRelease, Value: fmt.Sprint(source.ReleaseSequence)},
				}}
			}
		}

	}
	if alias != "" {
		if opts.alias != "" && opts.alias != alias {
			return environment.Requirement{}, environment.LockedPackage{}, errors.New("source_change: source already saved under alias " + alias)
		}
		old := snapshot.Requirements.Dependencies[alias]
		if requirement.Git != nil && old.Git != nil && old.Git.Selector.Kind != environment.SelectorCommit {
			if pin.Source.Git.Commit != requirement.Git.Selector.Value {
				return environment.Requirement{}, environment.LockedPackage{}, errors.New("saved ref must be changed with update " + alias)
			}
			requirement = old
		}
	}

	if alias == "" {
		return acquireSavedAdd(cmd, app, opts, source, targets, matches, requirement, snapshot.Requirements)
	}
	requirement.Alias = alias
	return requirement, pin, nil

}

func acquireSavedAdd(cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID, matches []string, requirement environment.Requirement, requirements environment.Requirements) (environment.Requirement, environment.LockedPackage, error) {
	var pin environment.LockedPackage
	loaded, err := loadSavedAddPackage(cmd, app, source, targets, matches, requirement)
	if err != nil {
		return environment.Requirement{}, environment.LockedPackage{}, err
	}
	defer func() { _ = loaded.cleanup() }()
	alias := opts.alias
	if alias == "" {
		alias = strings.ReplaceAll(strings.ToLower(loaded.envelope.Manifest.Name), ".", "-")
		if alias == "" || alias[0] < 'a' || alias[0] > 'z' {
			alias = "plugin-" + alias
		}
		if len(alias) > 64 {
			alias = alias[:64]
		}
	}
	if loaded.directorySelection != nil {
		selection := loaded.directorySelection
		requirement = environment.Requirement{Registry: &environment.RegistrySource{Distribution: selection.DistributionID, Selector: environment.Selector{Kind: environment.SelectorRelease, Value: fmt.Sprint(selection.ReleaseSequence)}}}
		pin.Source.Registry = &environment.LockedRegistrySource{Distribution: selection.DistributionID, DistributionID: selection.DistributionID, ProductID: selection.ProductID, ReleaseSequence: selection.ReleaseSequence, Requested: requirement.Registry.Selector, Repository: selection.Source.Repository, Path: selection.Source.Path, Commit: selection.Source.Revision}
	} else {
		pin.Source.Git = &environment.LockedGitSource{Repository: requirement.Git.Repository, Path: requirement.Git.Path, Commit: requirement.Git.Selector.Value, Requested: requirement.Git.Selector}
	}
	requirement.Alias = alias
	if err := requirement.Validate(); err != nil {
		return environment.Requirement{}, environment.LockedPackage{}, err
	}
	if old, ok := requirements.Dependencies[alias]; ok && mustSourceKey(old) != mustSourceKey(requirement) {
		return environment.Requirement{}, environment.LockedPackage{}, errors.New("alias already belongs to another source; use --alias")
	}
	pin.Alias = alias
	pin.SourceKey = mustSourceKey(requirement)
	pin.DigestAlgorithm = domain.TreeDigestAlgorithm
	pin.TreeDigest = loaded.envelope.TreeDigest
	pin.ManifestDigest = loaded.envelope.ManifestDigest
	pin.Version = loaded.envelope.Manifest.Version
	pin.ExecutableFiles = append([]string{}, loaded.envelope.ExecutableFiles...)
	if err := (dependencyResolver{app: app, dryRun: opts.dryRun}).finishLoaded(cmd.Context(), loaded, pin); err != nil {
		return environment.Requirement{}, environment.LockedPackage{}, err
	}
	return requirement, pin, nil

}

func rejectKnownOwnedAdd(app App, source string) error {
	if app.StateStore == nil {
		return nil
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return err
	}
	key := savedDirectSourceKey(source)
	if matches := exactGitPattern.FindStringSubmatch(source); matches != nil {
		key = "github:" + strings.ToLower(matches[1]) + ":" + matches[3]
	}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID != app.environmentID() {
			continue
		}
		if key != "" && receipt.SourceKey == key || isDirectorySelector(source) && receipt.SourceKey == "directory:"+strings.ToLower(source) {
			return fmt.Errorf("source belongs to saved dependency %s; use agentplugins add without --no-save or agentplugins update %s", receipt.Alias, receipt.Alias)
		}
	}
	return nil
}

func savedAddTargets(cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID) ([]domain.ClientID, error) {
	var err error
	if len(targets) == 0 && app.Terminal && opts.format == "human" {
		clients, e := app.Detector.Detect(cmd.Context())
		if e != nil {
			return nil, e
		}
		var eligible []domain.DetectedClient
		var skipped []targetSkip
		for _, client := range clients {
			if supportsDependencyTarget(app, string(client.ClientID)) {
				eligible = append(eligible, client)
			} else {
				skipped = append(skipped, targetSkip{Client: client.ClientID, Reason: "use --no-save"})
			}
		}
		if len(eligible) > 1 {
			if app, err = withPrompter(cmd, app, opts); err != nil {
				return nil, err
			}
		}
		targets, _, err = promptTargetChoices(cmd, app, eligible, skipped, clients)
		if err != nil {
			return nil, err
		}
	}
	return validateSavedAddTargets(app, opts, source, targets)
}

func validateSavedAddTargets(app App, opts *options, source string, targets []domain.ClientID) ([]domain.ClientID, error) {
	if len(targets) == 0 {
		return nil, errors.New("automated installation requires --target")
	}
	for _, target := range targets {
		if domain.ClientTraitsFor(target).RequiresPersonalMappingForPrepare && len(targets) > 1 {
			return nil, errors.New("ChatGPT is prepared manually; run agentplugins add <source> --target chatgpt separately, or use --no-save for the mixed imperative workflow")
		}
		if !supportsDependencyTarget(app, string(target)) {
			return nil, fmt.Errorf("target %s cannot be saved; retry agentplugins add %s --target %s --no-save", target, source, opts.target)
		}
	}
	if opts.scope != "user" && opts.scope != "project" {
		return nil, errors.New("saved dependencies support user scope only")
	}

	return targets, nil
}

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

func loadSavedAddPackage(cmd *cobra.Command, app App, source string, targets []domain.ClientID, matches []string, requirement environment.Requirement) (loadedPackage, error) {
	var err error
	var loaded loadedPackage
	if matches != nil {
		snapshot, e := app.acquireDependencyGit(cmd.Context(), *requirement.Git, requirement.Git.Selector.Value, "")
		if e != nil {
			return loadedPackage{}, e
		}
		identity := githubIdentity(requirement.Git.Repository, requirement.Git.Selector.Value, requirement.Git.Path)
		snapshot.Source = domain.SourceIdentity{RequestedSource: identity, CanonicalSource: identity, Repository: requirement.Git.Repository, ResolvedRevision: requirement.Git.Selector.Value, PackageSubpath: requirement.Git.Path, SourceBindingHint: "uap-dependency-git"}
		_, rootErr := os.Stat(filepath.Join(snapshot.Root, "plugin.json"))
		loaded, err = app.loadStandardSnapshot(cmd.Context(), snapshot, domain.OriginModeDirect, nil, nil)
		if requirement.Git.Path == "" && os.IsNotExist(rootErr) {
			paths, discoverErr := app.SourceAcquirer.DiscoverGitHubPackages(cmd.Context(), requirement.Git.Repository, requirement.Git.Selector.Value)
			if discoverErr != nil {
				return loadedPackage{}, discoverErr
			}
			return loadedPackage{}, fmt.Errorf("choose //path for a standard package; candidates: %v", paths)
		}
	} else {
		detected, detectErr := detectDependencyClients(cmd.Context(), app)
		if detectErr != nil {
			return loadedPackage{}, detectErr
		}
		loaded, err = app.acquireDirectory(cmd.Context(), source, withDetectedClients(packageResolutionRequest{StandardOnly: true, Operation: domain.DirectoryInstall, Targets: targets}, detected))
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
	opts.environmentOutput = map[string]any{"saved": true, "alias": alias, "manifest_path": snapshot.ManifestPath, "lock_path": snapshot.LockPath, "intent_change": !reflect.DeepEqual(snapshot.Requirements, requirements)}
	return runSavedEnvironment(cmd, app, opts, snapshot, requirements, locked, manifest, []string{dependencyNodeAlias(requirements, locked, alias)}, attest)
}
