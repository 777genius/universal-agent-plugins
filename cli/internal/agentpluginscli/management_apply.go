package agentpluginscli

import (
	"context"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environmentapp"
)

type dependencyApplicationResult = environmentapp.Result

// applyPreparedDependencies maps CLI, picker and management preparation to the
// transport-independent application service. The caller owns acceptance.
func applyPreparedDependencies(ctx context.Context, app App, request *dependencySyncRequest, adapter *dependencyInstaller, check func(context.Context) error) (dependencyApplicationResult, error) {
	// The service invokes this same guard before publication and before every
	// binding Prepare/Apply. Retain custom guards such as the Skills context check
	// while avoiding a second invocation inside the concrete installer adapter.
	guard := adapter.beforeApply
	adapter.beforeApply = nil
	defer func() { adapter.beforeApply = guard }()
	return (environmentapp.Service{
		Readiness: dependencyMutationReadiness{app: app},
		Lock:      dependencyEnvironmentLock{path: app.EnvironmentLockPath},
		Packages:  dependencyPackageMaterializer{app: app},
		Files:     dependencyIntentFiles{},
		Mutator:   adapter,
	}).Apply(ctx, environmentapp.Change{
		Intent: &request.intent,
		Plan: environment.PreparedSync{EnvironmentID: app.environmentID(),
			ManifestDigest: request.lock.ManifestDigest, Bindings: request.prepared},
		Lock: request.lock, Aliases: request.aliases, RepairOnly: request.repairOnly,
		Recheck: check, CheckInputs: guard,
	})
}

type dependencyMutationReadiness struct{ app App }

func (gate dependencyMutationReadiness) RequireMutationReady() error {
	return requireDependencyMutationReady(gate.app)
}

type dependencyPackageMaterializer struct{ app App }

func (materializer dependencyPackageMaterializer) Ensure(ctx context.Context, lock environment.Lock) error {
	return ensureDependencyStore(ctx, materializer.app, lock)
}

type dependencyEnvironmentLock struct{ path string }

func (lock dependencyEnvironmentLock) Acquire(ctx context.Context) (func() error, error) {
	unlock, err := (processlock.Lock{Path: lock.path}).Acquire(ctx)
	return unlock, err
}

type dependencyIntentFiles struct{}

func (dependencyIntentFiles) Read(path string) ([]byte, error) { return os.ReadFile(path) }
func (dependencyIntentFiles) Write(path string, body []byte) error {
	return publishEnvironmentFile(path, body)
}
