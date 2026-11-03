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
	lock, err := (environment.Coordinator{EnvironmentID: app.environmentID(), Resolver: dependencyResolver{app: app, requirements: requirements, dryRun: opts.dryRun}}).Resolve(ctx, requirements, previous, "")
	if err == nil {
		err = verifyDependencyDeclarations(ctx, app, lock)
	}
	return requirements, lock, err
}

func verifyDependencyDeclarations(ctx context.Context, app App, lock environment.Lock) error {
	index, err := indexDependencyDeclarations(ctx, lock)
	if err != nil {
		return err
	}
	for _, pin := range lock.Packages {
		if err := ctx.Err(); err != nil {
			return err
		}
		resolved, err := readReplayDependency(ctx, app, pin)
		if err != nil {
			return fmt.Errorf("verify %s pinned manifest; run agentplugins deps lock: %w", pin.Alias, err)
		}
		if pin.Version != "" && pin.Version != resolved.Package.Version {
			return fmt.Errorf("%s version differs from pinned manifest; run agentplugins deps lock", pin.Alias)
		}
		if err := index.verifyDeclaredEdges(ctx, pin.Alias, resolved.Dependencies); err != nil {
			return err
		}
	}
	return nil
}

func readReplayDependency(ctx context.Context, app App, pin environment.LockedPackage) (environment.ResolvedPackage, error) {
	resolved, err := describePinnedDependency(ctx, app, pin)
	if err == nil {
		return resolved, nil
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
