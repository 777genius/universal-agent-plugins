package installer

import (
	"context"
	"errors"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// VerifyPreparedBinding observes complete native materialization for a retained
// single-binding repair preparation. The handle, not a presentation Plan, owns
// the source, profile and delivery authority. No repair or receipt write occurs.
// Callers needing an atomic environment proof hold their existing mutation lock;
// state and captured profiles are also checked around observational callbacks.
func (e *Engine) VerifyPreparedBinding(ctx context.Context, prepared *PreparedOperation) (bool, error) {
	if ctx == nil || prepared == nil || prepared.engine != e {
		return false, ErrInvalidRequest
	}
	prepared.mu.Lock()
	defer prepared.mu.Unlock()
	if prepared.closed || prepared.busy || prepared.applied || prepared.req.Operation != OpRepair || len(prepared.req.Targets) > 0 {
		return false, ErrInvalidRequest
	}
	if err := e.checkNativeObservation(ctx, prepared); err != nil {
		return false, err
	}
	installation, ok := findInstall(prepared.recorded, prepared.plan.InstallationID)
	binding, found := installation.Clients[prepared.plan.BindingID]
	if !ok || !found || binding.ClientID != prepared.plan.ClientID || binding.TargetLocator != prepared.plan.TargetPath {
		return false, ErrNotInstalled
	}
	helper, err := e.preparedHelper(prepared)
	if err != nil {
		return false, err
	}
	service := e.lifecycle(helper, prepared.facts, prepared.detected)
	input := usecase.AddInput{Envelope: prepared.envelope, Client: prepared.client, Scope: domain.ScopeUser, BackendExecutable: prepared.req.ClientExecutable, EnvironmentAuthority: prepared.req.EnvironmentAuthority}
	coherent, observedErr := service.InspectOwnedBinding(ctx, input, cloneObservedDeliveryPlan(prepared.observationPlan), installation, binding)
	if err := e.checkNativeObservation(ctx, prepared); err != nil {
		return false, errors.Join(ErrPlanChanged, observedErr, err)
	}
	return coherent, observedErr
}

func (e *Engine) checkNativeObservation(ctx context.Context, prepared *PreparedOperation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.checkPreparedProfiles(ctx, prepared); err != nil {
		return err
	}
	if err := e.revalidateOpenCodeHost(ctx, prepared); err != nil {
		return err
	}
	state, err := e.store.Load()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(state, prepared.recorded) {
		return ErrPlanChanged
	}
	return nil
}
