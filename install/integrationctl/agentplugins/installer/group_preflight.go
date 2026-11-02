package installer

import (
	"context"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) requireGroupBindings(req Request, clients []domain.DetectedClient) error {
	if req.Operation == OpUpdate || req.Operation == OpRepair {
		for _, client := range clients {
			if err := e.requireExistingBinding(Request{
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
	handle.plan = Plan{ProfileAuthority: domain.CloneProfileAuthority(clients[0].ProfileAuthority),
		Operation: req.Operation, SourceRoot: roots[0], TreeDigest: envelopes[0].TreeDigest,
		DigestAlgorithm: handle.snapshot.DigestAlgorithm, ClientID: string(clients[0].ClientID),
		ConfigRoot: clients[0].ConfigRoot, InstallationID: firstNonEmpty(req.InstallationID, preview.InstallationID),
		HelperVersion: helperVersion, HelperDigest: helperDigest, RequiredMissing: missing,
		NoChange: groupPreviewUnchanged(preview),
	}
	if len(preview.Targets) > 0 {
		handle.plan.SelectedDelivery = preview.Targets[0].Plan.SelectedDelivery
		handle.plan.Delivery = deliveryPlan(preview.Targets[0].Plan)
		handle.plan.TargetPath = preview.Targets[0].Plan.ActivePath
		handle.plan.BindingID = domain.ComputeClientBindingID(handle.plan.InstallationID, string(preview.Targets[0].Plan.ClientID), string(preview.Targets[0].Plan.Scope), preview.Targets[0].Plan.ActivePath)
		handle.artifact = preview.Targets[0].Plan.PhysicalArtifactID
	}
	for i, target := range preview.Targets {
		client := clients[i]
		handle.plan.Targets = append(handle.plan.Targets, PlanTarget{ProfileAuthority: domain.CloneProfileAuthority(client.ProfileAuthority),
			ClientID: string(client.ClientID), ConfigRoot: client.ConfigRoot,
			TargetPath: target.Plan.ActivePath, TreeDigest: envelopes[i].TreeDigest,
			BindingID: domain.ComputeClientBindingID(handle.plan.InstallationID, string(target.Plan.ClientID), string(target.Plan.Scope), target.Plan.ActivePath),
			NoChange:  target.NoChange, SelectedDelivery: target.Plan.SelectedDelivery,
		})
	}
	handle.req.InstallationID = handle.plan.InstallationID
	handle.facts = BindingFacts{ProfileAuthority: domain.CloneProfileAuthority(handle.client.ProfileAuthority),
		SelectedDelivery: handle.plan.SelectedDelivery,
		InstallationID:   handle.plan.InstallationID, ClientID: handle.plan.ClientID,
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
	if req.AcquiredSnapshot == nil {
		e.reuseMatchingSourceIdentity(req.InstallationID, &snapshot, req.Operation == OpUpdate)
	}
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
