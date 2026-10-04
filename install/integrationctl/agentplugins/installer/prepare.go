package installer

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// PreparedOperation owns a sealed source snapshot until Close or a terminal Apply.
type PreparedOperation struct {
	engine       *Engine
	mu           sync.Mutex
	closed       bool
	busy         bool
	applied      bool
	req          Request
	plan         Plan
	snapshot     domain.PackageSnapshot
	snapshots    []domain.PackageSnapshot
	envelope     domain.PackageEnvelope
	envelopes    []domain.PackageEnvelope
	client       domain.DetectedClient
	clients      []domain.DetectedClient
	detected     map[domain.ClientID]domain.DetectedClient
	facts        BindingFacts
	openCodeHost *OpenCodePreparedHost
	artifact     string
	deliveries   []preparedDeliveryPlan
	recorded     domain.StateFileV2
}

func (p *PreparedOperation) Plan() Plan {
	out := p.plan
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
	if ctx == nil {
		return nil, fmt.Errorf("%w: context is required", ErrInvalidRequest)
	}
	if req.ExecutableFiles != nil && len(req.Targets) > 0 {
		return nil, fmt.Errorf("%w: explicit snapshot executable inventory requires a single target", ErrInvalidRequest)
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
	preview, err := dry(svc, usecase.AddInput{
		Envelope: handle.envelope, Client: handle.client, Scope: domain.ScopeUser, DryRun: true, Confirmed: false,
		PersistAuthoritativeObservations: e.persistObservations,
		InstallationID:                   req.InstallationID, OperationID: req.OperationID, BackendExecutable: req.ClientExecutable,
	})
	if err != nil {
		_ = handle.closeLocked()
		return nil, wrapLifecycleError(err)
	}
	return e.planMutatingPackage(handle, op, preview)
}

func (e *Engine) prepareRemove(ctx context.Context, req Request) (*PreparedOperation, error) {
	client, err := e.detectedClient(req)
	if err != nil {
		return nil, err
	}
	detected, err := e.detectedClients(req)
	if err != nil {
		return nil, err
	}
	client = physicalClient(req, client)
	detected = physicalDetected(req, detected)
	selector := req.Selector
	if selector == "" {
		selector = req.InstallationID
	}
	if selector == "" {
		return nil, fmt.Errorf("%w: remove requires InstallationID or Selector", ErrInvalidRequest)
	}
	state, err := e.store.Load()
	if err != nil {
		return nil, err
	}
	installation, ok := findInstall(state, selector)
	if !ok {
		return nil, fmt.Errorf("%w: installation %s is not installed", ErrInvalidRequest, selector)
	}
	binding, receipt, ok := findBinding(installation, client.ClientID)
	if !ok {
		e.report(ProgressPrepare)
		e.report(ProgressPreflight)
		handle := &PreparedOperation{engine: e, req: req, client: client, detected: detected, recorded: state}
		return e.prepareAbsentRemoval(handle, installation.InstallationID), nil
	}
	if err := e.removalPreflight(ctx, client, binding, receipt); err != nil {
		return nil, err
	}
	handle := &PreparedOperation{engine: e, req: req, client: client, detected: detected, recorded: state}
	helperVersion, helperDigest := e.helperIdentity()
	handle.plan = Plan{ProfileAuthority: domain.CloneProfileAuthority(client.ProfileAuthority),
		Operation: OpRemove, ClientID: string(client.ClientID), ConfigRoot: client.ConfigRoot,
		TargetPath: binding.TargetLocator, InstallationID: installation.InstallationID,
		BindingID: binding.ClientBindingID, HelperVersion: helperVersion, HelperDigest: helperDigest,
		SelectedDelivery: binding.SelectedDelivery, TreeDigest: recordedBindingDigest(binding, installation.Source.TreeDigest),
		Client: liveClientResult(binding, req.RequiredComponents, installation.Source.TreeDigest),
	}
	handle.facts = BindingFacts{ProfileAuthority: domain.CloneProfileAuthority(handle.client.ProfileAuthority),
		InstallationID: installation.InstallationID, ClientID: string(client.ClientID),
		BindingID: binding.ClientBindingID, Scope: binding.Scope, TargetPath: binding.TargetLocator,
		DataRoot: receipt.Locator, DataReceiptID: binding.DataReceiptID, OperationID: req.OperationID,
		SelectedDelivery: binding.SelectedDelivery, TreeDigest: recordedBindingDigest(binding, installation.Source.TreeDigest),
	}
	handle.artifact = binding.PhysicalArtifact
	e.report(ProgressPrepare)
	e.report(ProgressPreflight)
	return handle, nil
}

func (e *Engine) prepareAbsentRemoval(handle *PreparedOperation, installationID string) *PreparedOperation {
	helperVersion, helperDigest := e.helperIdentity()
	handle.plan = Plan{ProfileAuthority: domain.CloneProfileAuthority(handle.client.ProfileAuthority),
		Operation: OpRemove, ClientID: string(handle.client.ClientID), ConfigRoot: handle.client.ConfigRoot,
		InstallationID: installationID, HelperVersion: helperVersion,
		HelperDigest: helperDigest, NoChange: true,
	}
	handle.facts = BindingFacts{ProfileAuthority: domain.CloneProfileAuthority(handle.client.ProfileAuthority),
		InstallationID: installationID, ClientID: string(handle.client.ClientID),
		OperationID: handle.req.OperationID,
	}
	return handle
}

// removalPreflight is the §5.5.2 read-only check: exact target, persisted path,
// managed artifact digest, and owned PLUGIN_DATA. It does not lock, recover,
// EnsureData, invoke a helper, or deactivate the client. Apply repeats it
// before UAP Remove.
func (e *Engine) removalPreflight(ctx context.Context, client domain.DetectedClient, binding domain.ClientBinding, receipt domain.DataReceipt) error {
	if err := e.validateRemovalBinding(client, binding); err != nil {
		return err
	}
	if err := e.confirmSelectedNativeEntry(ctx, client, binding); err != nil {
		return err
	}
	digest := managedPackageDigest(binding)
	if digest == "" {
		return fmt.Errorf("%w: managed package digest is missing; refusing removal", ErrInvalidRequest)
	}
	target, err := e.planner().ResolveTarget(ctx, client, domain.ScopeUser, binding.PhysicalArtifact)
	if err != nil {
		return fmt.Errorf("%w: resolve managed removal target: %w", ErrInvalidRequest, err)
	}
	if err := pathpolicy.RequireExactPath(target.ActivePath, binding.TargetLocator); err != nil {
		return fmt.Errorf("%w: refuse removal from untrusted persisted target: %w", ErrInvalidRequest, err)
	}
	if err := (providers.Stager{}).Verify(ctx, binding.TargetLocator, digest); err != nil {
		return fmt.Errorf("%w: managed package was changed or is missing; refusing silent removal: %w", ErrInvalidRequest, err)
	}
	if receipt.DataReceiptID == "" {
		return nil
	}
	if err := (providers.PluginDataManager{Base: e.cfg.PluginDataBase}).ValidateData(ctx, receipt); err != nil {
		return fmt.Errorf("%w: required PLUGIN_DATA is missing or unreadable; refusing silent removal: %w", ErrInvalidRequest, err)
	}
	return nil
}

func managedPackageDigest(client domain.ClientBinding) string {
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" && object.ManagedDigest != "" {
			return object.ManagedDigest
		}
	}
	return ""
}

func (e *Engine) validatePackageRoots(req Request) error {
	if req.PackageRoot == "" || !validRoot(req.PackageRoot) {
		return fmt.Errorf("%w: PackageRoot must be an explicit absolute clean path", ErrInvalidRequest)
	}
	if req.SourceRoot != "" && !validRoot(req.SourceRoot) {
		return fmt.Errorf("%w: SourceRoot must be an explicit absolute clean path", ErrInvalidRequest)
	}
	if overlappingRoots(e.cfg.TempRoot, req.PackageRoot) {
		return fmt.Errorf("%w: TempRoot must not overlap PackageRoot", ErrInvalidRequest)
	}
	return nil
}

func (e *Engine) loadMutatingPackage(ctx context.Context, handle *PreparedOperation, op Operation, allowDigestRewrite bool) error {
	req, snapshot, client := handle.req, handle.snapshot, handle.client
	if err := e.assessSnapshot(ctx, snapshot, req.Assessment); err != nil {
		return err
	}
	if op == OpInstall {
		if err := e.refuseRecordedDigestRewrite(req.InstallationID, snapshot.TreeDigest); err != nil {
			return err
		}
	}
	if op == OpRepair || op == OpRefreshProjection {
		if err := e.refuseRepairRevisionRewrite(req.InstallationID, string(client.ClientID), snapshot.TreeDigest); err != nil {
			return err
		}
	}
	e.reuseMatchingSourceIdentity(req.InstallationID, &snapshot, allowDigestRewrite)
	handle.snapshot = snapshot
	ldr, err := newLoader()
	if err != nil {
		return err
	}
	envelope, err := ldr.Load(ctx, domain.LoadInput{
		SnapshotRoot: snapshot.Root, TreeDigest: snapshot.TreeDigest,
		ExecutableFiles: snapshot.ExecutableFiles, Source: snapshot.Source,
	})
	if err != nil {
		return err
	}
	handle.envelope = envelope
	return nil
}

func (e *Engine) planMutatingPackage(handle *PreparedOperation, op Operation, preview usecase.AddResult) (*PreparedOperation, error) {
	req, snapshot, client, envelope := handle.req, handle.snapshot, handle.client, handle.envelope
	missing := missingRequired(envelope, req.RequiredComponents)
	helperVersion, helperDigest := e.helperIdentity()
	handle.plan = Plan{ProfileAuthority: domain.CloneProfileAuthority(client.ProfileAuthority),
		Operation: op, SourceRoot: firstNonEmpty(req.SourceRoot, req.PackageRoot), TreeDigest: snapshot.TreeDigest,
		DigestAlgorithm: snapshot.DigestAlgorithm, ClientID: string(client.ClientID),
		ConfigRoot: client.ConfigRoot, TargetPath: preview.Plan.ActivePath,
		InstallationID: firstNonEmpty(req.InstallationID, preview.InstallationID),
		BindingID:      domain.ComputeClientBindingID(firstNonEmpty(req.InstallationID, preview.InstallationID), string(preview.Plan.ClientID), string(preview.Plan.Scope), preview.Plan.ActivePath),
		HelperVersion:  helperVersion, HelperDigest: helperDigest,
		RequiredMissing: missing, NoChange: preview.NoChange,
		Delivery: deliveryPlan(preview.Plan), SelectedDelivery: preview.Plan.SelectedDelivery,
		Client: ClientResult{ClientID: string(client.ClientID), Activation: string(preview.Activation.Activation),
			Authentication: string(preview.Activation.Authentication), Policy: string(preview.Activation.Policy), Verification: string(preview.Activation.Verification)},
		RequiresConfirmation: preview.RequiresConfirmation,
	}
	if handle.openCodeHost != nil {
		profile := handle.openCodeHost.Profile()
		handle.plan.OpenCodeProfile = &profile
		handle.plan.OpenCodeSelections = handle.openCodeHost.Selections()
	}
	handle.facts = BindingFacts{ProfileAuthority: domain.CloneProfileAuthority(handle.client.ProfileAuthority),
		InstallationID: handle.plan.InstallationID, ClientID: handle.plan.ClientID,
		BindingID: handle.plan.BindingID, Scope: string(preview.Plan.Scope),
		SelectedDelivery: preview.Plan.SelectedDelivery,
		TargetPath:       handle.plan.TargetPath, OperationID: req.OperationID, TreeDigest: snapshot.TreeDigest,
	}
	handle.req.InstallationID = handle.plan.InstallationID
	handle.artifact = preview.Plan.PhysicalArtifactID
	if len(missing) != 0 {
		_ = handle.closeLocked()
		return nil, fmt.Errorf("%w: %v", ErrIncomplete, missing)
	}
	return handle, nil
}

func clonePrepareRequest(req Request) Request {
	copied := req
	copied.RequiredComponents = append([]string(nil), req.RequiredComponents...)
	copied.ExecutableFiles = slices.Clone(req.ExecutableFiles)
	copied.Targets = append([]ClientTarget(nil), req.Targets...)
	copied.KnownTargets = append([]TargetFacts(nil), req.KnownTargets...)
	if req.Assessment != nil {
		assessment := *req.Assessment
		copied.Assessment = &assessment
	}
	return copied
}
