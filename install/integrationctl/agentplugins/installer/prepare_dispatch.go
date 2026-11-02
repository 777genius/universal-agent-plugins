package installer

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// PreparedOperation owns a sealed source snapshot until Close or a terminal Apply.
type PreparedOperation struct {
	engine           *Engine
	mu               sync.Mutex
	closed           bool
	busy             bool
	applied          bool
	req              Request
	plan             Plan
	observationPlan  domain.DeliveryPlan
	snapshot         domain.PackageSnapshot
	snapshots        []domain.PackageSnapshot
	envelope         domain.PackageEnvelope
	envelopes        []domain.PackageEnvelope
	client           domain.DetectedClient
	clients          []domain.DetectedClient
	detected         map[domain.ClientID]domain.DetectedClient
	facts            BindingFacts
	openCodeHost     *OpenCodePreparedHost
	openCodeSiblings []domain.DetectedClient
	artifact         string
	deliveries       []preparedDeliveryPlan
	recorded         domain.StateFileV2
}

func (p *PreparedOperation) Plan() Plan {
	out := p.plan
	out.Scope = p.req.Scope
	out.TargetContext = domain.TargetContext{Scope: p.req.Scope, ProjectRoot: p.req.ProjectRoot, UserConfigRoot: p.client.ConfigRoot}
	out.ProfileAuthority = domain.CloneProfileAuthority(p.plan.ProfileAuthority)
	out.Client.ProfileAuthority = domain.CloneProfileAuthority(p.plan.Client.ProfileAuthority)
	if p.plan.OpenCodeProfile != nil {
		profile := p.plan.OpenCodeProfile.Clone()
		out.OpenCodeProfile = &profile
	}
	out.OpenCodeSelections = cloneOpenCodeSelections(p.plan.OpenCodeSelections)
	out.RequiredMissing = append([]string(nil), p.plan.RequiredMissing...)
	out.Delivery = cloneDeliveryPlan(p.plan.Delivery)
	out.Delivery.ProfileAuthority = domain.CloneProfileAuthority(p.plan.ProfileAuthority)
	out.Client.RequiredComponents = slices.Clone(p.plan.Client.RequiredComponents)
	if len(p.plan.Targets) > 0 {
		out.Targets = append([]PlanTarget(nil), p.plan.Targets...)
		for i := range out.Targets {
			out.Targets[i].ProfileAuthority = domain.CloneProfileAuthority(out.Targets[i].ProfileAuthority)
		}
	}
	return out
}

func (p *PreparedOperation) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.busy {
		return ErrHandleBusy
	}
	return p.closeLocked()
}

func (p *PreparedOperation) closeLocked() error {
	if p.closed {
		return nil
	}
	p.closed = true
	seen := map[string]bool{}
	var first error
	remove := func(snapshot domain.PackageSnapshot) {
		if snapshot.Root == "" || seen[snapshot.Root] {
			return
		}
		seen[snapshot.Root] = true
		if err := packagedigest.Remove(snapshot); err != nil && first == nil {
			first = err
		}
	}
	for _, snapshot := range p.snapshots {
		remove(snapshot)
	}
	remove(p.snapshot)
	return first
}

// Prepare captures a sealed snapshot for install or inspects owned state for remove.
// It does not mutate installed client config or the Notifications runtime ledger.
func (e *Engine) Prepare(ctx context.Context, req Request) (*PreparedOperation, error) {
	if err := validatePrepareInput(ctx, req); err != nil {
		return nil, err
	}
	if req.Scope == "" {
		req.Scope = domain.ScopeUser
	}
	if err := e.validatePrepareContext(req); err != nil {
		return nil, err
	}
	copied := clonePrepareRequest(req)
	if len(copied.Targets) == 1 {
		target := copied.Targets[0]
		copied.ClientID = target.ClientID
		copied.ClientConfigRoot = target.ClientConfigRoot
		copied.ClientExecutable = firstNonEmpty(target.ClientExecutable, copied.ClientExecutable)
		copied.PackageRoot = firstNonEmpty(target.PackageRoot, copied.PackageRoot)
		copied.ExternalUninstalled = target.ExternalUninstalled
		copied.Targets = nil
	}
	if len(copied.Targets) > 1 {
		if err := e.validateGroupTargets(copied); err != nil {
			return nil, err
		}
	}
	var authorityErr error
	copied.physical, authorityErr = e.captureRequestProfiles(ctx, copied)
	if authorityErr != nil {
		return nil, authorityErr
	}
	if len(copied.Targets) > 1 {
		return e.prepareGroup(ctx, copied)
	}
	switch copied.Operation {
	case OpInstall:
		return e.prepareInstall(ctx, copied)
	case OpUpdate:
		return e.prepareUpdate(ctx, copied)
	case OpRepair:
		return e.prepareRepair(ctx, copied)
	case OpRefreshProjection:
		return e.prepareRefreshProjection(ctx, copied)
	case OpRemove:
		return e.prepareRemove(ctx, copied)
	default:
		return nil, fmt.Errorf("%w: unknown operation", ErrInvalidRequest)
	}
}

func (e *Engine) prepareInstall(ctx context.Context, req Request) (*PreparedOperation, error) {
	return e.prepareMutatingPackage(ctx, req, OpInstall, false, func(svc usecase.Service, in usecase.AddInput) (usecase.AddResult, error) {
		return svc.Add(ctx, in)
	})
}

func (e *Engine) prepareUpdate(ctx context.Context, req Request) (*PreparedOperation, error) {
	if err := e.requireExistingBinding(req); err != nil {
		return nil, err
	}
	return e.prepareMutatingPackage(ctx, req, OpUpdate, true, func(svc usecase.Service, in usecase.AddInput) (usecase.AddResult, error) {
		return e.updateWithCompatibility(ctx, svc, req, in)
	})
}

func (e *Engine) prepareRepair(ctx context.Context, req Request) (*PreparedOperation, error) {
	if err := e.requireExistingBinding(req); err != nil {
		return nil, err
	}
	return e.prepareMutatingPackage(ctx, req, OpRepair, false, func(svc usecase.Service, in usecase.AddInput) (usecase.AddResult, error) {
		return svc.Repair(ctx, in)
	})
}

func (e *Engine) prepareRefreshProjection(ctx context.Context, req Request) (*PreparedOperation, error) {
	if err := e.requireExistingBinding(req); err != nil {
		return nil, err
	}
	return e.prepareMutatingPackage(ctx, req, OpRefreshProjection, false, func(svc usecase.Service, in usecase.AddInput) (usecase.AddResult, error) {
		return svc.RefreshProjection(ctx, in)
	})
}

func (e *Engine) requireExistingBinding(req Request) error {
	client, err := e.detectedClient(req)
	if err != nil {
		return err
	}
	if req.InstallationID == "" {
		return fmt.Errorf("%w: InstallationID is required", ErrInvalidRequest)
	}
	state, err := e.store.Load()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNotInstalled, err)
	}
	installation, ok := findInstall(state, req.InstallationID)
	if !ok {
		return fmt.Errorf("%w: installation %s", ErrNotInstalled, req.InstallationID)
	}
	_, _, ok = findBinding(installation, client.ClientID)
	if !ok {
		return fmt.Errorf("%w: client %s", ErrNotInstalled, req.ClientID)
	}
	return nil
}
