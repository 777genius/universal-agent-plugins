package usecase

import (
	"context"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func (service Service) persistNativeRemovalDecision(ctx context.Context, installationID, bindingID string, before, desired domain.StateFileV2) error {
	recorder, ok := service.Kernel.NativeRecovery.(ports.NativeRemovalDecisionRecorder)
	if !ok {
		return service.persistLifecycleState(desired)
	}
	// Keep the core's target isolated even from a callback holding the same
	// in-memory state store. Reuse the existing persisted snapshot boundary.
	target, err := freezeNativeRemovalAttempt(before, desired, "")
	if err != nil {
		return err
	}
	input, err := freezeNativeRemovalAttempt(target.before, target.attempted, "")
	if err != nil {
		return err
	}
	handled, err := recorder.PrepareNativeRemovalDecision(ctx, installationID, bindingID, input.before, input.attempted)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(input.before, target.before) || !reflect.DeepEqual(input.attempted, target.attempted) {
		return fmt.Errorf("native removal decision callback changed frozen state")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	observed, err := service.StateStore.Load()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(observed, target.before) {
		return fmt.Errorf("native removal decision callback changed durable predecessor")
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	if err := kernel.PersistStateDecision(target.before, target.attempted); err != nil {
		return err
	}
	if handled {
		return service.Kernel.NativeRecovery.Recover(ctx)
	}
	return nil
}
