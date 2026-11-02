package installer

import (
	"context"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
	preview, err := dry(svc, usecase.AddInput{
		Envelope: handle.envelope, Client: handle.client, Scope: e.scope(), DryRun: true, Confirmed: false,
		PersistAuthoritativeObservations: e.persistObservations,
		InstallationID:                   req.InstallationID, OperationID: req.OperationID, BackendExecutable: req.ClientExecutable,
		OriginMode: snapshot.OriginMode, DirectoryResolution: cloneAcquiredDirectory(snapshot.Directory), Hints: hints,
		EnvironmentAuthority: req.EnvironmentAuthority,
		ActivationComplete:   req.ActivationComplete, AuthComplete: req.AuthComplete,
	})
	if err != nil {
		_ = handle.closeLocked()
		return nil, wrapLifecycleError(err)
	}
	return e.planMutatingPackage(handle, op, preview)
}
