package environmentapp

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type MutationReadiness interface {
	RequireMutationReady() error
}

type EnvironmentLock interface {
	Acquire(context.Context) (func() error, error)
}

// PackageMaterializer retains the host's strict source, provenance and security
// checks when making the selected immutable packages available for application.
type PackageMaterializer interface {
	Ensure(context.Context, environment.Lock) error
}

type Service struct {
	Readiness MutationReadiness
	Lock      EnvironmentLock
	Packages  PackageMaterializer
	Files     IntentFiles
	Mutator   environment.BindingMutator
}

// Change is an already-accepted review. Recheck verifies the retained review
// under the environment lock. CheckInputs optionally replaces the default
// intent check with a host guard that also checks its captured context; that
// guard must include Intent.Check and observes the same advancing Intent.
// Neither callback is a confirmation or presentation hook.
type Change struct {
	Intent      *Intent
	Plan        environment.PreparedSync
	Lock        environment.Lock
	Aliases     []string
	RepairOnly  bool
	Recheck     func(context.Context) error
	CheckInputs func() error
}

type Result struct {
	Results   []installer.Result
	Published bool
}

// Apply owns the application ordering around the existing coordinator. It never
// resolves intent, asks for confirmation or replaces installer transactions.
func (service Service) Apply(ctx context.Context, change Change) (out Result, err error) {
	if change.Intent == nil || change.Recheck == nil {
		return out, errors.New("reviewed environment intent and recheck are required")
	}
	checkInputs := change.CheckInputs
	if checkInputs == nil {
		checkInputs = func() error { return change.Intent.Check(service.Files) }
	}
	if err = service.Readiness.RequireMutationReady(); err != nil {
		return out, err
	}
	unlock, err := service.Lock.Acquire(ctx)
	if err != nil {
		return out, err
	}
	defer func() { _ = unlock() }()
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if err = service.Readiness.RequireMutationReady(); err != nil {
		return out, err
	}
	if err = change.Recheck(ctx); err != nil {
		return out, err
	}
	if !change.RepairOnly {
		if err = service.Packages.Ensure(ctx, SelectedLock(change.Lock, change.Aliases)); err != nil {
			return out, err
		}
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if err = checkInputs(); err != nil {
		return out, err
	}
	// Observe durable effects even when an atomic replacement succeeds before its
	// parent-directory sync reports an error, or only the manifest was saved.
	before := change.Intent.snapshot()
	defer func() { out.Published = before.Check(service.Files) != nil }()
	if err = change.Intent.Publish(service.Files); err != nil {
		return out, err
	}
	out.Results, err = (environment.Coordinator{
		EnvironmentID: change.Plan.EnvironmentID,
		Mutator:       guardedMutator{inner: service.Mutator, check: checkInputs},
	}).ApplySync(ctx, change.Plan, true)
	return out, err
}

// Both preparation and application must see the current expected intent bytes,
// including the successful writes performed immediately before the first bind.
type guardedMutator struct {
	inner environment.BindingMutator
	check func() error
}

func (mutator guardedMutator) Prepare(ctx context.Context, binding environment.PreparedBinding) (*installer.PreparedOperation, error) {
	if err := mutator.check(); err != nil {
		return nil, err
	}
	return mutator.inner.Prepare(ctx, binding)
}

func (mutator guardedMutator) Apply(ctx context.Context, prepared *installer.PreparedOperation) (installer.Result, error) {
	if err := mutator.check(); err != nil {
		return installer.Result{}, err
	}
	return mutator.inner.Apply(ctx, prepared)
}
