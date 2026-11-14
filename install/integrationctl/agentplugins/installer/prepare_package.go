package installer

import (
	"context"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) prepareMutatingPackage(ctx context.Context, req Request, op Operation, allowDigestRewrite bool, dry func(usecase.Service, usecase.AddInput) (usecase.AddResult, error)) (*PreparedOperation, error) {
	client, detected, err := e.preparedClients(req)
	if err != nil {
		return nil, err
	}
	if err := e.validatePackageRoots(req); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(e.cfg.TempRoot, 0700); err != nil {
		return nil, err
	}
	e.report(ProgressPrepare)
	if err := e.checkRequestProfiles(ctx, req); err != nil {
		return nil, err
	}
	snapshot, err := snapshotRequestPackage(ctx, e.cfg.TempRoot, req)
	if err != nil {
		return nil, err
	}
	if req.SourceRoot != "" {
		snapshot.Source.RequestedSource = req.SourceRoot
		snapshot.Source.CanonicalSource = req.SourceRoot
	}
	handle := &PreparedOperation{engine: e, req: req, snapshot: snapshot, client: client, detected: detected}
	if err := e.loadMutatingPackage(ctx, handle, op, allowDigestRewrite); err != nil {
		_ = handle.closeLocked()
		return nil, err
	}
	handle.recorded, err = e.store.Load()
	if err != nil {
		_ = handle.closeLocked()
		return nil, err
	}
	if err := e.prepareOpenCodeHost(ctx, handle); err != nil {
		_ = handle.closeLocked()
		return nil, err
	}
	helper, _ := e.helper()
	previewFacts := BindingFacts{}
	if installation, ok := findInstall(handle.recorded, req.InstallationID); ok {
		previewFacts.InstallationID = installation.InstallationID
	}
	svc := confirmationLifecycle(handle, e.lifecycle(helper, previewFacts, handle.detected), true)
	var hints domain.CompatibilityHints
	if req.AcquiredSnapshot != nil {
		hints = req.AcquiredSnapshot.Compatibility
	}
	preview, err := e.previewHostedPackage(ctx, handle, svc, usecase.AddInput{
		Envelope: handle.envelope, Client: handle.client, Scope: e.scope(), DryRun: true, Confirmed: false,
		PersistAuthoritativeObservations: e.persistObservations,
		InstallationID:                   req.InstallationID, OperationID: req.OperationID, BackendExecutable: req.ClientExecutable,
		OriginMode: snapshot.OriginMode, DirectoryResolution: cloneAcquiredDirectory(snapshot.Directory), Hints: hints,
		EnvironmentAuthority: req.EnvironmentAuthority,
		ConfirmPluginEnable:  req.ConfirmPluginEnable,
		ActivationComplete:   req.ActivationComplete, AuthComplete: req.AuthComplete,
	}, dry)
	if err != nil {
		_ = handle.closeLocked()
		return nil, wrapLifecycleError(err)
	}
	return e.planMutatingPackage(handle, op, preview)
}

// A saved install uses the same registered preparation capability as the
// imperative CLI. Only a fresh automatic User preflight may select it; the
// sealed package and original recorded state remain the confirmation authority.
func (e *Engine) previewHostedPackage(ctx context.Context, handle *PreparedOperation, svc usecase.Service, input usecase.AddInput, dry func(usecase.Service, usecase.AddInput) (usecase.AddResult, error)) (usecase.AddResult, error) {
	captured := len(handle.deliveries)
	preview, originalErr := dry(svc, input)
	if err := ctx.Err(); err != nil {
		return preview, err
	}
	if originalErr == nil || !freshHostedPreparation(handle, input, preview) {
		return preview, originalErr
	}
	preflighter, ok := svc.Activator.(ports.ActivationPreflighter)
	if !ok {
		return preview, originalErr
	}
	preflightErr := preflighter.PreflightActivation(domain.ActivationRequest{
		Client: input.Client, Plan: cloneObservedDeliveryPlan(preview.Plan), BackendExecutable: input.BackendExecutable,
		VerifyOnly: true, ActivationComplete: input.ActivationComplete,
	})
	if err := ctx.Err(); err != nil {
		return preview, err
	}
	if preflightErr == nil {
		return preview, originalErr
	}
	live, err := e.store.Load()
	if err != nil {
		return preview, err
	}
	if err := confirmFreshHostedPreparation(handle, live, preview.InstallationID); err != nil {
		return preview, err
	}
	// Discard only the failed preview's unconfirmed plan captures. Replanning
	// reruns every source, scope, component and ownership guard with the same IID.
	handle.deliveries = handle.deliveries[:captured]
	input.InstallationID, input.InstallIntent = preview.InstallationID, domain.InstallIntentPrepare
	return dry(svc, input)
}

func freshHostedPreparation(handle *PreparedOperation, input usecase.AddInput, preview usecase.AddResult) bool {
	plan := preview.Plan
	if handle.req.Operation != OpInstall || input.Scope != domain.ScopeUser || input.ActivationComplete || input.AuthComplete ||
		plan.ClientID != input.Client.ClientID || plan.Scope != domain.ScopeUser || plan.TargetContext.ProjectRoot != "" ||
		plan.TargetContext.Scope == domain.ScopeProject || plan.Status != domain.PlanReady || plan.InstallIntent != domain.InstallIntentAutomatic ||
		!plan.SelectedDelivery.IsZero() || preview.InstallationID == "" {
		return false
	}
	traits := plan.SelectedDelivery.EffectiveTraitsForScope(plan.ClientID, plan.Scope)
	_, recorded := findInstall(handle.recorded, preview.InstallationID)
	return !recorded && traits.Allows(domain.InstallIntentPrepare) && !traits.RequiresPersonalMappingForPrepare
}

func confirmFreshHostedPreparation(handle *PreparedOperation, live domain.StateFileV2, installationID string) error {
	_, recorded := findInstall(handle.recorded, installationID)
	_, appeared := findInstall(live, installationID)
	if !recorded && appeared {
		return fmt.Errorf("%w: installation appeared after fresh preparation was selected", ErrPlanChanged)
	}
	return nil
}

func confirmHostedPreparation(handle *PreparedOperation, live domain.StateFileV2) error {
	if handle.req.Operation == OpInstall && handle.observationPlan.InstallIntent == domain.InstallIntentPrepare {
		return confirmFreshHostedPreparation(handle, live, handle.plan.InstallationID)
	}
	return nil
}
