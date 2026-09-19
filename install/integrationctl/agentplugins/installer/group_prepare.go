package installer

import (
	"context"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) prepareMutatingGroup(ctx context.Context, req Request) (*PreparedOperation, error) {
	detected, err := e.detectedClients(req)
	if err != nil {
		return nil, err
	}
	roots, err := e.groupPackageRoots(req)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(e.cfg.TempRoot, 0700); err != nil {
		return nil, err
	}
	e.report(ProgressPrepare)
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
	if err := e.requireGroupBindings(req, clients); err != nil {
		_ = handle.closeLocked()
		return nil, err
	}
	helper, _ := e.helper()
	svc := e.lifecycle(helper, BindingFacts{}, handle.detected)
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

func (e *Engine) requireGroupBindings(req Request, clients []domain.DetectedClient) error {
	if req.Operation == OpUpdate || req.Operation == OpRepair {
		for _, client := range clients {
			if _, _, err := e.requireExistingBinding(Request{
				InstallationID: req.InstallationID, ClientID: string(client.ClientID),
				ClientConfigRoot: client.ConfigRoot, ClientExecutable: client.ExecutablePath,
				Operation: req.Operation,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Engine) planMutatingGroup(handle *PreparedOperation, roots []string, envelopes []domain.PackageEnvelope, clients []domain.DetectedClient, preview usecase.GroupResult) (*PreparedOperation, error) {
	req := handle.req
	missing := missingRequired(envelopes[0], req.RequiredComponents)
	for _, envelope := range envelopes[1:] {
		missing = append(missing, missingRequired(envelope, req.RequiredComponents)...)
	}
	helperVersion, helperDigest := e.helperIdentity()
	handle.clients = clients
	handle.client = clients[0]
	handle.plan = Plan{
		Operation: req.Operation, SourceRoot: roots[0], TreeDigest: envelopes[0].TreeDigest,
		DigestAlgorithm: handle.snapshot.DigestAlgorithm, ClientID: string(clients[0].ClientID),
		ConfigRoot: clients[0].ConfigRoot, InstallationID: firstNonEmpty(req.InstallationID, preview.InstallationID),
		HelperVersion: helperVersion, HelperDigest: helperDigest, RequiredMissing: missing,
		NoChange: groupPreviewUnchanged(preview),
	}
	if len(preview.Targets) > 0 {
		handle.plan.TargetPath = preview.Targets[0].Plan.ActivePath
		handle.plan.BindingID = domain.ComputeClientBindingID(handle.plan.InstallationID, string(preview.Targets[0].Plan.ClientID), string(preview.Targets[0].Plan.Scope), preview.Targets[0].Plan.ActivePath)
		handle.artifact = preview.Targets[0].Plan.PhysicalArtifactID
	}
	for i, target := range preview.Targets {
		client := clients[i]
		handle.plan.Targets = append(handle.plan.Targets, PlanTarget{
			ClientID: string(client.ClientID), ConfigRoot: client.ConfigRoot,
			TargetPath: target.Plan.ActivePath, TreeDigest: envelopes[i].TreeDigest,
			BindingID: domain.ComputeClientBindingID(handle.plan.InstallationID, string(target.Plan.ClientID), string(target.Plan.Scope), target.Plan.ActivePath),
			NoChange:  target.NoChange,
		})
	}
	handle.facts = BindingFacts{
		InstallationID: handle.plan.InstallationID, ClientID: handle.plan.ClientID,
		BindingID: handle.plan.BindingID, TargetPath: handle.plan.TargetPath,
		OperationID: req.OperationID, TreeDigest: envelopes[0].TreeDigest,
	}
	if len(missing) != 0 {
		_ = handle.closeLocked()
		return nil, fmt.Errorf("%w: %v", ErrIncomplete, missing)
	}
	return handle, nil
}

func (e *Engine) loadGroupSnapshot(ctx context.Context, handle *PreparedOperation, ldr loader.Loader, root string, i int) (domain.PackageEnvelope, string, error) {
	req := handle.req
	snapshot, err := snapshotLocalPackage(ctx, e.cfg.TempRoot, root)
	if err != nil {
		_ = handle.closeLocked()
		return domain.PackageEnvelope{}, "", err
	}
	handle.snapshots = append(handle.snapshots, snapshot)
	if handle.snapshot.Root == "" {
		handle.snapshot = snapshot
	}
	if err := e.assessSnapshot(ctx, snapshot, req.Assessment); err != nil {
		_ = handle.closeLocked()
		return domain.PackageEnvelope{}, "", err
	}
	if req.Operation == OpInstall {
		if err := e.refuseRecordedDigestRewrite(req.InstallationID, snapshot.TreeDigest); err != nil {
			_ = handle.closeLocked()
			return domain.PackageEnvelope{}, "", err
		}
	}
	e.reuseMatchingSourceIdentity(req.InstallationID, &snapshot, req.Operation == OpUpdate)
	handle.snapshots[len(handle.snapshots)-1] = snapshot
	if i == 0 {
		handle.snapshot = snapshot
	}
	envelope, err := ldr.Load(ctx, domain.LoadInput{
		SnapshotRoot: snapshot.Root, TreeDigest: snapshot.TreeDigest,
		ExecutableFiles: snapshot.ExecutableFiles, Source: snapshot.Source,
	})
	if err != nil {
		_ = handle.closeLocked()
		return domain.PackageEnvelope{}, "", err
	}
	return envelope, snapshot.TreeDigest, nil
}
