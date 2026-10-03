package installer

import (
	"context"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// preparedDeliveryPlan retains the actual adapter output before the use case
// adds a persisted or staged projection digest. Both revisions matter: copying
// the persisted digest over a changed adapter digest must not hide drift.
type preparedDeliveryPlan struct {
	request domain.PlanRequest
	plan    domain.DeliveryPlan
}

type confirmationPlanner struct {
	inner    ports.DeliveryPlanner
	prepared *PreparedOperation
	capture  bool
	store    interface {
		Load() (domain.StateFileV2, error)
	}
}

func (p confirmationPlanner) Plan(ctx context.Context, request domain.PlanRequest) (domain.DeliveryPlan, error) {
	if !p.capture {
		state, err := p.store.Load()
		if err != nil {
			return domain.DeliveryPlan{}, err
		}
		if err := confirmRecordedDeliveries(p.prepared, state); err != nil {
			return domain.DeliveryPlan{}, err
		}
	}
	plan, err := p.inner.Plan(ctx, request)
	if err != nil {
		if !p.capture {
			return plan, fmt.Errorf("%w: regenerate selected plan: %w", ErrPlanChanged, err)
		}
		return plan, err
	}
	if p.capture {
		p.prepared.deliveries = append(p.prepared.deliveries, preparedDeliveryPlan{request: request, plan: plan})
		return plan, nil
	}
	matched := false
	for _, frozen := range p.prepared.deliveries {
		if frozen.request.Client.ClientID == request.Client.ClientID && frozen.request.Scope == request.Scope {
			matched = true
			if err := confirmDeliveryPlan(frozen.plan, plan); err != nil {
				return plan, err
			}
		}
	}
	if !matched {
		return plan, fmt.Errorf("%w: delivery was not prepared", ErrPlanChanged)
	}
	return plan, nil
}

func confirmDeliveryPlan(frozen, current domain.DeliveryPlan) error {
	// SameSelection/SameProfile intentionally discard revision fields. Use the
	// complete-value semantics of usecase group coalescing at this boundary too.
	if frozen.ActivePath != current.ActivePath || !reflect.DeepEqual(frozen.SelectedDelivery, current.SelectedDelivery) {
		return fmt.Errorf("%w: selected delivery differs from confirmed plan", ErrPlanChanged)
	}
	return nil
}

func confirmationLifecycle(prepared *PreparedOperation, svc usecase.Service, capture bool) usecase.Service {
	svc.Planner = confirmationPlanner{inner: svc.Planner, prepared: prepared, capture: capture, store: svc.StateStore}
	return svc
}

func (e *Engine) confirmDeliveries(ctx context.Context, prepared *PreparedOperation) error {
	if prepared.req.Operation == OpRemove {
		return e.confirmRemoveBindings(ctx, prepared)
	}
	if len(prepared.deliveries) == 0 {
		return nil
	}
	state, err := e.store.Load()
	if err != nil {
		return err
	}
	if err := confirmRecordedDeliveries(prepared, state); err != nil {
		return err
	}
	for _, frozen := range prepared.deliveries {
		current, err := e.planner().Plan(ctx, frozen.request)
		if err != nil {
			return fmt.Errorf("%w: regenerate selected plan: %w", ErrPlanChanged, err)
		}
		if err := confirmDeliveryPlan(frozen.plan, current); err != nil {
			return err
		}
	}
	return nil
}

// A newly planned revision may intentionally differ from the old binding (for
// update/refresh). Freeze the old committed authority independently so a later
// receipt edit cannot disappear behind SameSelection or projection sealing.
func confirmRecordedDeliveries(prepared *PreparedOperation, current domain.StateFileV2) error {
	before, _ := findInstall(prepared.recorded, prepared.plan.InstallationID)
	after, _ := findInstall(current, prepared.plan.InstallationID)
	for _, delivery := range prepared.deliveries {
		id := delivery.request.Client.ClientID
		old, _, oldOK := findBinding(before, id)
		live, _, liveOK := findBinding(after, id)
		if delivery.plan.SelectedDelivery.IsZero() && old.SelectedDelivery.IsZero() && live.SelectedDelivery.IsZero() {
			continue
		}
		if oldOK != liveOK || old.ClientBindingID != live.ClientBindingID || !reflect.DeepEqual(old.SelectedDelivery, live.SelectedDelivery) {
			return fmt.Errorf("%w: recorded delivery differs from prepared binding", ErrPlanChanged)
		}
	}
	return nil
}
