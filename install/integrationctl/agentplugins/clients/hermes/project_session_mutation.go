package hermes

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Retain the published root through the entire component transaction. Entry
// digests alone cannot authorize an identical config in a replacement root.
// The borrowed kernel uses this original directory for locks, reads, writes and
// compensation; it never follows a replacement pathname after admission.
func (adapter *Adapter) applyComponents(ctx context.Context, env clients.Env, plan domain.DeliveryPlan, objects []domain.NativeObjectOwnership, mutation shared.ProjectComponentMutation) (resultErr error) {
	selected := false
	for _, object := range objects {
		selected = selected || object.Kind == projectSessionRootKind
	}
	if plan.Scope != domain.ScopeProject || !selected {
		return mutation.Apply(ctx)
	}
	if err := adapter.verifyProjectRoot(ctx, env, plan, objects); err != nil {
		return err
	}
	directory, err := openSessionDirectory(projectSessionRoot(plan.TargetContext.ProjectRoot))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.close()) }()
	revalidate := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := directory.rejoin(); err != nil {
			return err
		}
		if err := adapter.verifyProjectRoot(ctx, env, plan, objects); err != nil {
			return err
		}
		return directory.rejoin()
	}
	mutation.Kernel, err = mutation.Kernel.WithDirectory(directory.root, directory.path, revalidate)
	if err != nil {
		return err
	}
	mutation.Revalidate = revalidate
	applyErr, authorityErr := mutation.Apply(ctx), revalidate()
	if authorityErr != nil {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUncertain, Err: errors.Join(applyErr, authorityErr)}
	}
	return applyErr
}
