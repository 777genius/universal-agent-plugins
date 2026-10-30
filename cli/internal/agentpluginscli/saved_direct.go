package agentpluginscli

import (
	"context"
	"errors"
	"net/url"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func directRequirement(spec sourcespec.Spec) environment.Requirement {
	requirement := environment.Requirement{Alias: "candidate"}
	if source := spec.Git; source != nil && source.Browser == nil {
		selector := environment.Selector{Kind: environment.SelectorDefault}
		if source.Ref != "" {
			selector = environment.Selector{Kind: environment.SelectorRef, Value: source.Ref}
		}
		if source.Commit != "" {
			selector = environment.Selector{Kind: environment.SelectorCommit, Value: source.Commit}
		}
		requirement.Git = &environment.GitSource{Repository: source.Repository, Path: source.Path, Selector: selector}
		if source.Repository == "" {
			requirement.Git.URL = source.URL
		}
	}
	if source := spec.Archive; source != nil && spec.Kind == sourcespec.KindArchive {
		requirement.Archive = &environment.ArchiveSource{URL: source.URL, Path: source.Path, SHA256: source.SHA256}
	}
	return requirement
}

func retainedDirectAdd(snapshot environmentSnapshot, requirement environment.Requirement, requestedAlias string) (environment.Requirement, environment.LockedPackage, bool, error) {
	if requirement.Git == nil && requirement.Archive == nil {
		return requirement, environment.LockedPackage{}, false, nil
	}
	key := mustSourceKey(requirement)
	for _, pin := range snapshot.Lock.Packages {
		if pin.SourceKey != key {
			continue
		}
		alias := dependencyRootKey(snapshot.Requirements, snapshot.Lock, pin.Alias)
		if alias == "" {
			return requirement, pin, false, nil
		}
		if requestedAlias != "" && requestedAlias != alias {
			return requirement, pin, true, errors.New("source_change: source already saved under alias " + alias)
		}
		old, declared := snapshot.Requirements.Dependencies[alias]
		if !declared {
			// A retained transitive node already owns its alias and immutable pin.
			// Promotion saves that pin's requested intent, without resolving it again.
			old = pin.Requirement()
		}
		same := old.RequestedSelector() == requirement.RequestedSelector()
		if requirement.Git != nil && pin.Source.Git != nil && requirement.Git.Selector.Kind == environment.SelectorCommit {
			same = same || requirement.Git.Selector.Value == pin.Source.Git.Commit
		}
		if !same {
			return requirement, pin, true, errors.New("saved selector must be changed with update " + alias + " or an explicit uap.toml edit")
		}
		return old, pin, true, nil
	}
	return requirement, environment.LockedPackage{}, false, nil
}

func resolveDirectSavedAdd(ctx context.Context, app App, opts *options, source string, snapshot environmentSnapshot) (environment.Requirement, environment.LockedPackage, error) {
	spec, err := sourcespec.Parse(source)
	if err != nil {
		return environment.Requirement{}, environment.LockedPackage{}, err
	}
	requirement := directRequirement(spec)
	retained, pin, found, err := retainedDirectAdd(snapshot, requirement, opts.alias)
	if found || err != nil {
		return retained, pin, err
	}
	result, err := app.acquireDirectPackage(ctx, spec)
	if err != nil {
		return environment.Requirement{}, environment.LockedPackage{}, err
	}
	defer func() { _ = result.loaded.cleanup() }()
	requirement = result.requirement
	requirement.Alias = "candidate"
	retained, pin, found, err = retainedDirectAdd(snapshot, requirement, opts.alias)
	if found || err != nil {
		return retained, pin, err
	}
	alias := opts.alias
	if alias == "" {
		alias = dependencyManifestAlias(result.loaded.envelope.Manifest.Name)
	}
	requirement.Alias = alias
	if err := requirement.Validate(); err != nil {
		return requirement, environment.LockedPackage{}, err
	}
	if old, exists := snapshot.Requirements.Dependencies[alias]; exists && mustSourceKey(old) != mustSourceKey(requirement) {
		return requirement, environment.LockedPackage{}, errors.New("alias already belongs to another source; use --alias")
	}
	result.requirement = requirement
	pin = completeDirectPin(result, alias)
	if err := (dependencyResolver{app: app, dryRun: opts.dryRun}).finishLoaded(ctx, result.loaded, pin); err != nil {
		return requirement, pin, err
	}
	return requirement, pin, nil
}

func completeDirectPin(result directPackage, alias string) environment.LockedPackage {
	pin := result.pin
	pin.Alias, pin.SourceKey = alias, mustSourceKey(result.requirement)
	pin.Version = result.loaded.envelope.Manifest.Version
	pin.DigestAlgorithm, pin.TreeDigest = domain.TreeDigestAlgorithm, result.loaded.envelope.TreeDigest
	pin.ManifestDigest = result.loaded.envelope.ManifestDigest
	pin.ExecutableFiles = append([]string{}, result.loaded.envelope.ExecutableFiles...)
	return pin
}

func savedDirectSourceKey(raw string) string {
	spec, err := sourcespec.Parse(raw)
	if err != nil {
		return ""
	}
	requirement := directRequirement(spec)
	if requirement.Git == nil && requirement.Archive == nil {
		return ""
	}
	return mustSourceKey(requirement)
}

func directSourceRequested(source domain.SourceBinding) string {
	if source.SourceKind == "archive" {
		return source.SourceURL + "#sha256=" + source.ArchiveSHA256 + archivePathSelector(source.PackageSubpath)
	}
	value := source.SourceURL + "#" + source.ResolvedRevision
	if source.PackageSubpath != "" {
		value += "//" + source.PackageSubpath
	}
	return value
}
func archivePathSelector(packagePath string) string {
	if packagePath == "" {
		return ""
	}
	// Archive paths are already portable; the parser decodes fragment values.
	return "&path=" + url.QueryEscape(packagePath)
}

func localArchiveUpdateSource(source domain.SourceBinding) string {
	spec, err := sourcespec.Parse(source.RequestedSource)
	if err == nil && spec.Archive != nil && spec.Archive.SHA256 != "" {
		return source.SourceURL + "#sha256=" + spec.Archive.SHA256 + archivePathSelector(source.PackageSubpath)
	}
	return source.SourceURL
}
