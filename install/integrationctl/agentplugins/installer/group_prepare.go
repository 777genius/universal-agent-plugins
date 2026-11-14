package installer

import (
	"context"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (e *Engine) prepareMutatingGroup(ctx context.Context, req Request) (*PreparedOperation, error) {
	detected, err := e.detectedClients(req)
	if err != nil {
		return nil, err
	}
	detected = physicalDetected(req, detected)
	roots, err := e.groupPackageRoots(req)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(e.cfg.TempRoot, 0700); err != nil {
		return nil, err
	}
	e.report(ProgressPrepare)
	if err := e.checkRequestProfiles(ctx, req); err != nil {
		return nil, err
	}
	handle := &PreparedOperation{engine: e, req: req, detected: detected}
	envelopes, err := e.loadGroupPackages(ctx, handle, roots)
	if err != nil {
		return nil, err
	}
	inputs, clients, err := e.groupAddInputs(req, envelopes, true, false)
	if err != nil {
		_ = handle.closeLocked()
		return nil, err
	}
	for i := range clients {
		clients[i] = physicalClient(req, clients[i])
		inputs[i].Client = clients[i]
	}
	if err := e.requireGroupBindings(req, clients); err != nil {
		_ = handle.closeLocked()
		return nil, err
	}
	handle.recorded, err = e.store.Load()
	if err != nil {
		_ = handle.closeLocked()
		return nil, err
	}
	helper, _ := e.helper()
	previewFacts := BindingFacts{}
	if installation, ok := findInstall(handle.recorded, req.InstallationID); ok {
		previewFacts.InstallationID = installation.InstallationID
	}
	svc := confirmationLifecycle(handle, e.lifecycle(helper, previewFacts, handle.detected), true)
	preview, err := e.previewGroup(ctx, svc, req, inputs)
	if err != nil {
		_ = handle.closeLocked()
		return nil, wrapLifecycleError(err)
	}
	return e.planMutatingGroup(handle, roots, envelopes, clients, preview)
}

func (e *Engine) groupPackageRoots(req Request) ([]string, error) {
	roots := make([]string, len(req.Targets))
	for i, target := range req.Targets {
		root := firstNonEmpty(target.PackageRoot, req.PackageRoot)
		if root == "" || !validRoot(root) {
			return nil, fmt.Errorf("%w: PackageRoot must be an explicit absolute clean path", ErrInvalidRequest)
		}
		if overlappingRoots(e.cfg.TempRoot, root) {
			return nil, fmt.Errorf("%w: TempRoot must not overlap PackageRoot", ErrInvalidRequest)
		}
		roots[i] = root
	}
	mixed := false
	for _, root := range roots[1:] {
		if root != roots[0] {
			mixed = true
			break
		}
	}
	if mixed && req.Operation != OpRepair {
		return nil, fmt.Errorf("%w: mixed package roots in one group are unpublished", ErrUnsupported)
	}
	return roots, nil
}

func (e *Engine) loadGroupPackages(ctx context.Context, handle *PreparedOperation, roots []string) ([]domain.PackageEnvelope, error) {
	req := handle.req
	ldr, err := newLoader()
	if err != nil {
		return nil, err
	}
	cachedEnvelope := map[string]domain.PackageEnvelope{}
	cachedDigest := map[string]string{}
	envelopes := make([]domain.PackageEnvelope, len(req.Targets))
	for i, root := range roots {
		if envelope, ok := cachedEnvelope[root]; ok {
			envelopes[i] = envelope
		} else {
			envelope, digest, err := e.loadGroupSnapshot(ctx, handle, ldr, root, i)
			if err != nil {
				return nil, err
			}
			cachedEnvelope[root] = envelope
			cachedDigest[root] = digest
			envelopes[i] = envelope
		}
		if req.Operation == OpRepair {
			if err := e.refuseRepairRevisionRewrite(req.InstallationID, req.Targets[i].ClientID, cachedDigest[root]); err != nil {
				_ = handle.closeLocked()
				return nil, err
			}
		}
	}
	handle.envelope = envelopes[0]
	handle.envelopes = envelopes
	return envelopes, nil
}
