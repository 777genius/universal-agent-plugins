package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
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
	if err := validateSavedAddInput(app, opts, source, targets, matches != nil, activation || auth); err != nil {
		return true, err
	}
	if err := inspectSavedAddRoot(cmd.Context(), app, opts, source); err != nil {
		return true, err
	}
	targets, err = savedAddTargets(cmd, app, opts, source, targets)
	if err != nil {
		return true, err
	}
	if err := validateSavedAddAttestation(targets, activation || auth); err != nil {
		return true, err
	}
	return true, runSavedAdd(cmd, app, opts, source, targets, matches, activation, auth)
}

func validateSavedAddInput(app App, opts *options, source string, targets []domain.ClientID, exactGit, attestation bool) error {
	if !exactGit && !isDirectorySelector(source) {
		if _, err := sourcespec.Parse(source); err != nil {
			return err
		}
	}
	if len(targets) == 0 && app.Terminal && opts.format == "human" {
		return nil
	}
	if _, err := validateSavedAddTargets(app, opts, source, targets); err != nil {
		return err
	}
	return validateSavedAddAttestation(targets, attestation)
}

func validateSavedAddAttestation(targets []domain.ClientID, required bool) error {
	if required && len(targets) != 1 {
		return errors.New("explicit lifecycle attestation requires exactly one target")
	}
	return nil
}

func cloneSavedRequirements(value environment.Requirements) environment.Requirements {
	cloned := environment.Requirements{SchemaVersion: value.SchemaVersion, Dependencies: map[string]environment.Requirement{}, Targets: map[string][]string{}}
	for alias, item := range value.Dependencies {
		cloned.Dependencies[alias] = item
	}
	if len(value.Development) > 0 {
		cloned.Development = make(map[string]bool, len(value.Development))
		for alias := range value.Development {
			cloned.Development[alias] = true
		}
	}
	for target, aliases := range value.Targets {
		cloned.Targets[target] = append([]string(nil), aliases...)
	}
	return cloned
}

func resolveSavedAdd(cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID, matches []string, snapshot environmentSnapshot) (environment.Requirement, environment.LockedPackage, error) {
	return resolveSavedAddContext(cmd.Context(), app, opts, source, targets, matches, snapshot)
}

func resolveSavedAddContext(ctx context.Context, app App, opts *options, source string, targets []domain.ClientID, matches []string, snapshot environmentSnapshot) (environment.Requirement, environment.LockedPackage, error) {
	if matches == nil && !isDirectorySelector(source) {
		return resolveDirectSavedAdd(ctx, app, opts, source, snapshot)
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
		requirement, pin, alias, err = savedRegistryAddCandidate(snapshot, source, opts.alias)
		if err != nil {
			return environment.Requirement{}, environment.LockedPackage{}, err
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
		return acquireSavedAdd(ctx, app, opts, source, targets, matches, requirement, snapshot.Requirements)
	}
	requirement.Alias = alias
	return requirement, pin, nil
}

func savedRegistryAddCandidate(snapshot environmentSnapshot, source, requestedAlias string) (environment.Requirement, environment.LockedPackage, string, error) {
	pin, err := matchSavedRegistry(snapshot.Lock, source, requestedAlias)
	if err != nil {
		return environment.Requirement{}, environment.LockedPackage{}, "", err
	}
	alias := dependencyRootKey(snapshot.Requirements, snapshot.Lock, pin.Alias)
	var requirement environment.Requirement
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
	return requirement, pin, alias, nil
}
