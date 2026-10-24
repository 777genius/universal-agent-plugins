package agentpluginscli

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Validate the original graph before converge can repair any client projection.
// Only an explicit lock operation may rebuild an incomplete recorded graph.
func resolveDependencySyncCandidate(ctx context.Context, app App, opts *options, requirements environment.Requirements, previous environment.Lock, lockedOnly bool) (environment.Requirements, environment.Lock, error) {
	digest, err := environment.ManifestDigest(requirements)
	if err != nil {
		return requirements, environment.Lock{}, err
	}
	if lockedOnly || previous.LockVersion != 0 && previous.ManifestDigest == digest {
		if err := previous.Validate(requirements); err != nil {
			return requirements, environment.Lock{}, fmt.Errorf("exact current uap.lock required; run agentplugins deps lock: %w", err)
		}
		if err := verifyDependencyDeclarations(ctx, app, previous); err != nil {
			return requirements, environment.Lock{}, err
		}
		return requirements, previous, nil
	}
	requirements, err = app.canonicalizeDependencyRequirements(ctx, requirements)
	if err != nil {
		return requirements, environment.Lock{}, err
	}
	lock, err := (environment.Coordinator{Resolver: dependencyResolver{app: app, requirements: requirements, dryRun: opts.dryRun}}).Resolve(ctx, requirements, previous, "")
	if err == nil {
		err = verifyDependencyDeclarations(ctx, app, lock)
	}
	return requirements, lock, err
}

func verifyDependencyDeclarations(ctx context.Context, app App, lock environment.Lock) error {
	bySource := make(map[string]environment.LockedPackage, len(lock.Packages))
	for _, pin := range lock.Packages {
		bySource[pin.SourceKey] = pin
	}
	for _, pin := range lock.Packages {
		resolved, err := readReplayDependency(ctx, app, pin)
		if err != nil {
			return fmt.Errorf("verify %s pinned manifest; run agentplugins deps lock: %w", pin.Alias, err)
		}
		if pin.Version != "" && pin.Version != resolved.Package.Version {
			return fmt.Errorf("%s version differs from pinned manifest; run agentplugins deps lock", pin.Alias)
		}
		if err := verifyDeclaredEdges(pin.Alias, resolved.Dependencies, bySource); err != nil {
			return err
		}
	}
	return nil
}

func readReplayDependency(ctx context.Context, app App, pin environment.LockedPackage) (environment.ResolvedPackage, error) {
	_, err := app.dependencyStore().Get(ctx, pin.DigestAlgorithm, pin.TreeDigest)
	if err == nil {
		return describePinnedDependency(ctx, app, pin)
	}
	if !errors.Is(err, environmentstore.ErrMissing) {
		return environment.ResolvedPackage{}, err
	}
	loaded, err := materializeLockedPackage(ctx, app, pin, false, nil)
	if err != nil {
		return environment.ResolvedPackage{}, err
	}
	if loaded.cleanup != nil {
		defer func() { _ = loaded.cleanup() }()
	}
	pin.Version = loaded.envelope.Manifest.Version
	facts, err := packageDependencyFacts(loaded.envelope)
	if err != nil {
		return environment.ResolvedPackage{}, err
	}
	return environment.ResolvedPackage{Package: pin, Dependencies: facts}, nil
}

func verifyDeclaredEdges(parent string, declarations map[string]environment.Requirement, bySource map[string]environment.LockedPackage) error {
	expected := make(map[string]environment.Selector, len(declarations))
	for _, requirement := range declarations {
		key, err := requirement.SourceKey()
		if err != nil {
			return err
		}
		selector := requirement.RequestedSelector()
		expected[key] = selector
	}
	for key, pin := range bySource {
		for _, edge := range pin.RequiredBy {
			if edge.Alias != parent {
				continue
			}
			selector, exists := expected[key]
			if !exists || selector != edge.Selector {
				return fmt.Errorf("%s has an unexpected dependency edge to %s; run agentplugins deps lock", parent, pin.Alias)
			}
			delete(expected, key)
		}
	}
	if len(expected) != 0 {
		return fmt.Errorf("%s pinned manifest declares missing dependency edges; run agentplugins deps lock", parent)
	}
	return nil
}
