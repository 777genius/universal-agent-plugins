package agentpluginscli

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type dependencyApplicationResult struct {
	Results   []installer.Result
	Published bool
}

// applyPreparedDependencies is shared by CLI, picker and management. The caller
// already owns explicit acceptance; the existing environment lock and installer
// receipts remain the only authority for durable dependency/native effects.
func applyPreparedDependencies(ctx context.Context, app App, request *dependencySyncRequest, adapter *dependencyInstaller, check func(context.Context) error) (out dependencyApplicationResult, err error) {
	if adapter.beforeApply == nil {
		adapter.beforeApply = func() error { return request.checkInputs() }
	}
	if err = requireDependencyMutationReady(app); err != nil {
		return out, err
	}
	unlock, err := (processlock.Lock{Path: app.EnvironmentLockPath}).Acquire(ctx)
	if err != nil {
		return out, err
	}
	defer func() { _ = unlock() }()
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if err = requireDependencyMutationReady(app); err != nil {
		return out, err
	}
	if err = check(ctx); err != nil {
		return out, err
	}
	if !request.repairOnly {
		if err = ensureDependencyStore(ctx, app, request.selectedLock()); err != nil {
			return out, err
		}
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if err = adapter.beforeApply(); err != nil {
		return out, err
	}
	// A failed second publication is still a durable intent change. Preserve that
	// fact even when no binding reached the installer yet.
	manifest, lock := append([]byte(nil), request.manifestBytes...), append([]byte(nil), request.lockBytes...)
	defer func() {
		out.Published = !sameEnvironmentFile(request.manifestPath, manifest) || !sameEnvironmentFile(request.lockPath, lock)
	}()
	if err = request.publish(); err != nil {
		return out, err
	}
	out.Results, err = (environment.Coordinator{EnvironmentID: app.environmentID(), Mutator: adapter}).ApplySync(ctx, environment.PreparedSync{
		EnvironmentID: app.environmentID(), ManifestDigest: request.lock.ManifestDigest, Bindings: request.prepared,
	}, true)
	return out, err
}
