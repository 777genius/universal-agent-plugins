package shared

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func componentUnchanged(err error) error {
	return &NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
}

func (mutation ProjectComponentMutation) Apply(ctx context.Context) error {
	requests, err := mutation.prepare(ctx)
	if err != nil {
		return componentUnchanged(err)
	}
	txn := newProjectSkillSwap(mutation)
	if err := txn.stage(ctx); err != nil {
		return componentUnchanged(errors.Join(err, txn.cleanup()))
	}
	return mutation.commit(ctx, requests, txn)
}

func (mutation ProjectComponentMutation) commit(ctx context.Context, requests []nativeconfig.Request, txn *projectSkillSwap) error {
	if err := txn.swap(ctx); err != nil {
		return rollbackProjectSkills(txn, err, true)
	}
	if err := mutation.Layout.Verify(projectSkillObjects(mutation), false, mutation.Kernel); err != nil {
		return rollbackProjectSkills(txn, err, true)
	}
	if err := ctx.Err(); err != nil {
		return rollbackProjectSkills(txn, err, true)
	}
	_, err := mutation.Kernel.ApplyBatch(requests)
	if err != nil && !nativeconfig.IsCommittedCleanup(err) {
		certain := errors.Is(err, nativeconfig.ErrCollision) || errors.Is(err, nativeconfig.ErrNotOwned)
		return rollbackProjectSkills(txn, err, certain)
	}
	if cleanupErr := txn.cleanup(); cleanupErr != nil {
		return &nativeconfig.CommittedCleanupError{Err: errors.Join(err, cleanupErr)}
	}
	return err
}

func projectSkillObjects(mutation ProjectComponentMutation) []domain.NativeObjectOwnership {
	objects := []domain.NativeObjectOwnership{}
	for _, object := range mutation.Desired {
		if object.Kind == mutation.Layout.SkillKind {
			objects = append(objects, object)
		}
	}
	return objects
}

func rollbackProjectSkills(txn *projectSkillSwap, cause error, unchanged bool) error {
	if err := txn.restore(); err != nil {
		return fmt.Errorf("%w; project skill rollback refused: %w; recovery retained at %q", cause, err, txn.root)
	}
	cleanupErr := txn.cleanup()
	result := errors.Join(cause, cleanupErr)
	if unchanged {
		return componentUnchanged(result)
	}
	return result
}
