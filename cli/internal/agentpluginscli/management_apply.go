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
	// A failed second publication is still a durable intent change. Preserve that
	// fact even when no binding reached the installer yet.
	manifest, lock := append([]byte(nil), request.manifestBytes...), append([]byte(nil), request.lockBytes...)
	defer func() {
		out.Published = !sameEnvironmentFile(request.manifestPath, manifest) || !sameEnvironmentFile(request.lockPath, lock)
	}()
	if err = request.publish(); err != nil {
		return out, err
	}
	adapter.beforeApply = request.checkInputs
	out.Results, err = (environment.Coordinator{Mutator: adapter}).ApplySync(ctx, environment.PreparedSync{
		EnvironmentID: environment.EnvironmentUserDefault, ManifestDigest: request.lock.ManifestDigest, Bindings: request.prepared,
	}, true)
	return out, err
}
