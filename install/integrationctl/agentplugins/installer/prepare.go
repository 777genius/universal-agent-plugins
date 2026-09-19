package installer

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// PreparedOperation owns a sealed source snapshot until Close or a terminal Apply.
type PreparedOperation struct {
	engine    *Engine
	mu        sync.Mutex
	closed    bool
	busy      bool
	applied   bool
	req       Request
	plan      Plan
	snapshot  domain.PackageSnapshot
	snapshots []domain.PackageSnapshot
	envelope  domain.PackageEnvelope
	envelopes []domain.PackageEnvelope
	client    domain.DetectedClient
	clients   []domain.DetectedClient
	detected  map[domain.ClientID]domain.DetectedClient
	facts     BindingFacts
	artifact  string
}

func (p *PreparedOperation) Plan() Plan {
	out := p.plan
	out.RequiredMissing = append([]string(nil), p.plan.RequiredMissing...)
	if len(p.plan.Targets) > 0 {
		out.Targets = append([]PlanTarget(nil), p.plan.Targets...)
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
	copied := req
	copied.RequiredComponents = append([]string(nil), req.RequiredComponents...)
	copied.Targets = append([]ClientTarget(nil), req.Targets...)
	copied.KnownTargets = append([]TargetFacts(nil), req.KnownTargets...)
	if req.Assessment != nil {
		assessment := *req.Assessment
		copied.Assessment = &assessment
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
	if _, _, err := e.requireExistingBinding(req); err != nil {
		return nil, err
	}
	return e.prepareMutatingPackage(ctx, req, OpUpdate, true, func(svc usecase.Service, in usecase.AddInput) (usecase.AddResult, error) {
		return e.updateWithCompatibility(ctx, svc, req, in)
	})
}

func (e *Engine) prepareRepair(ctx context.Context, req Request) (*PreparedOperation, error) {
	if _, _, err := e.requireExistingBinding(req); err != nil {
		return nil, err
	}
	return e.prepareMutatingPackage(ctx, req, OpRepair, false, func(svc usecase.Service, in usecase.AddInput) (usecase.AddResult, error) {
		return svc.Repair(ctx, in)
	})
}

func (e *Engine) requireExistingBinding(req Request) (domain.Installation, domain.ClientBinding, error) {
	client, err := e.detectedClient(req)
	if err != nil {
		return domain.Installation{}, domain.ClientBinding{}, err
	}
	if req.InstallationID == "" {
		return domain.Installation{}, domain.ClientBinding{}, fmt.Errorf("%w: InstallationID is required", ErrInvalidRequest)
	}
	state, err := e.store.Load()
	if err != nil {
		return domain.Installation{}, domain.ClientBinding{}, fmt.Errorf("%w: %v", ErrNotInstalled, err)
	}
	installation, ok := findInstall(state, req.InstallationID)
	if !ok {
		return domain.Installation{}, domain.ClientBinding{}, fmt.Errorf("%w: installation %s", ErrNotInstalled, req.InstallationID)
	}
	binding, _, ok := findBinding(installation, client.ClientID)
	if !ok {
		return domain.Installation{}, domain.ClientBinding{}, fmt.Errorf("%w: client %s", ErrNotInstalled, req.ClientID)
	}
	return installation, binding, nil
}

func (e *Engine) prepareMutatingPackage(ctx context.Context, req Request, op Operation, allowDigestRewrite bool, dry func(usecase.Service, usecase.AddInput) (usecase.AddResult, error)) (*PreparedOperation, error) {
	client, err := e.detectedClient(req)
	if err != nil {
		return nil, err
	}
	detected, err := e.detectedClients(req)
	if err != nil {
		return nil, err
	}
	if req.PackageRoot == "" || !validRoot(req.PackageRoot) {
		return nil, fmt.Errorf("%w: PackageRoot must be an explicit absolute clean path", ErrInvalidRequest)
	}
	if req.SourceRoot != "" && !validRoot(req.SourceRoot) {
		return nil, fmt.Errorf("%w: SourceRoot must be an explicit absolute clean path", ErrInvalidRequest)
	}
	if overlappingRoots(e.cfg.TempRoot, req.PackageRoot) {
		return nil, fmt.Errorf("%w: TempRoot must not overlap PackageRoot", ErrInvalidRequest)
	}
	if err := os.MkdirAll(e.cfg.TempRoot, 0700); err != nil {
		return nil, err
	}
	e.report(ProgressPrepare)
	snapshot, err := snapshotLocalPackage(ctx, e.cfg.TempRoot, req.PackageRoot)
	if err != nil {
		return nil, err
	}
	if req.SourceRoot != "" {
		snapshot.Source.RequestedSource = req.SourceRoot
		snapshot.Source.CanonicalSource = req.SourceRoot
	}
	handle := &PreparedOperation{engine: e, req: req, snapshot: snapshot, client: client, detected: detected}
	if err := e.assessSnapshot(ctx, snapshot, req.Assessment); err != nil {
		_ = handle.closeLocked()
		return nil, err
	}
	if op == OpInstall {
		if err := e.refuseRecordedDigestRewrite(req.InstallationID, snapshot.TreeDigest); err != nil {
			_ = handle.closeLocked()
			return nil, err
		}
	}
	if op == OpRepair {
		if err := e.refuseRepairRevisionRewrite(req.InstallationID, string(client.ClientID), snapshot.TreeDigest); err != nil {
			_ = handle.closeLocked()
			return nil, err
		}
	}
	e.reuseMatchingSourceIdentity(req.InstallationID, &snapshot, allowDigestRewrite)
	handle.snapshot = snapshot
	ldr, err := newLoader()
	if err != nil {
		_ = handle.closeLocked()
		return nil, err
	}
	envelope, err := ldr.Load(ctx, domain.LoadInput{
		SnapshotRoot: snapshot.Root, TreeDigest: snapshot.TreeDigest,
		ExecutableFiles: snapshot.ExecutableFiles, Source: snapshot.Source,
	})
	if err != nil {
		_ = handle.closeLocked()
		return nil, err
	}
	handle.envelope = envelope
	helper, _ := e.helper()
	svc := e.lifecycle(helper, BindingFacts{}, handle.detected)
	preview, err := dry(svc, usecase.AddInput{
		Envelope: envelope, Client: client, Scope: domain.ScopeUser, DryRun: true, Confirmed: false,
		PersistAuthoritativeObservations: e.persistObservations,
		InstallationID:                   req.InstallationID, OperationID: req.OperationID, BackendExecutable: req.ClientExecutable,
	})
	if err != nil {
		_ = handle.closeLocked()
		return nil, wrapLifecycleError(err)
	}
	missing := missingRequired(envelope, req.RequiredComponents)
	helperVersion, helperDigest := e.helperIdentity()
	handle.plan = Plan{
		Operation: op, SourceRoot: firstNonEmpty(req.SourceRoot, req.PackageRoot), TreeDigest: snapshot.TreeDigest,
		DigestAlgorithm: snapshot.DigestAlgorithm, ClientID: string(client.ClientID),
		ConfigRoot: req.ClientConfigRoot, TargetPath: preview.Plan.ActivePath,
		InstallationID: firstNonEmpty(req.InstallationID, preview.InstallationID),
		BindingID:      domain.ComputeClientBindingID(firstNonEmpty(req.InstallationID, preview.InstallationID), string(preview.Plan.ClientID), string(preview.Plan.Scope), preview.Plan.ActivePath),
		HelperVersion:  helperVersion, HelperDigest: helperDigest,
		RequiredMissing: missing, NoChange: preview.NoChange,
	}
	handle.facts = BindingFacts{
		InstallationID: handle.plan.InstallationID, ClientID: handle.plan.ClientID,
		BindingID: handle.plan.BindingID, Scope: string(preview.Plan.Scope),
		TargetPath: handle.plan.TargetPath, OperationID: req.OperationID, TreeDigest: snapshot.TreeDigest,
	}
	handle.artifact = preview.Plan.PhysicalArtifactID
	if len(missing) != 0 {
		_ = handle.closeLocked()
		return nil, fmt.Errorf("%w: %v", ErrIncomplete, missing)
	}
	return handle, nil
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
		handle := &PreparedOperation{engine: e, req: req, client: client, detected: detected}
		helperVersion, helperDigest := e.helperIdentity()
		handle.plan = Plan{
			Operation: OpRemove, ClientID: string(client.ClientID), ConfigRoot: req.ClientConfigRoot,
			InstallationID: installation.InstallationID, HelperVersion: helperVersion,
			HelperDigest: helperDigest, NoChange: true,
		}
		handle.facts = BindingFacts{
			InstallationID: installation.InstallationID, ClientID: string(client.ClientID),
			OperationID: req.OperationID,
		}
		return handle, nil
	}
	e.report(ProgressPrepare)
	e.report(ProgressPreflight)
	if err := e.removalPreflight(ctx, client, binding, receipt); err != nil {
		return nil, err
	}
	handle := &PreparedOperation{engine: e, req: req, client: client, detected: detected}
	helperVersion, helperDigest := e.helperIdentity()
	handle.plan = Plan{
		Operation: OpRemove, ClientID: string(client.ClientID), ConfigRoot: req.ClientConfigRoot,
		TargetPath: binding.TargetLocator, InstallationID: installation.InstallationID,
		BindingID: binding.ClientBindingID, HelperVersion: helperVersion, HelperDigest: helperDigest,
	}
	handle.facts = BindingFacts{
		InstallationID: installation.InstallationID, ClientID: string(client.ClientID),
		BindingID: binding.ClientBindingID, Scope: binding.Scope, TargetPath: binding.TargetLocator,
		DataRoot: receipt.Locator, DataReceiptID: binding.DataReceiptID, OperationID: req.OperationID,
	}
	handle.artifact = binding.PhysicalArtifact
	return handle, nil
}

// removalPreflight is the §5.5.2 read-only check: exact target, persisted path,
// managed artifact digest, and owned PLUGIN_DATA. It does not lock, recover,
// EnsureData, invoke a helper, or deactivate the client. Apply repeats it
// before UAP Remove.
func (e *Engine) removalPreflight(ctx context.Context, client domain.DetectedClient, binding domain.ClientBinding, receipt domain.DataReceipt) error {
	digest := managedPackageDigest(binding)
	if digest == "" {
		return fmt.Errorf("%w: managed package digest is missing; refusing removal", ErrInvalidRequest)
	}
	target, err := e.planner().ResolveTarget(ctx, client, domain.ScopeUser, binding.PhysicalArtifact)
	if err != nil {
		return fmt.Errorf("%w: resolve managed removal target: %v", ErrInvalidRequest, err)
	}
	if err := pathpolicy.RequireExactPath(target.ActivePath, binding.TargetLocator); err != nil {
		return fmt.Errorf("%w: refuse removal from untrusted persisted target: %v", ErrInvalidRequest, err)
	}
	if err := (providers.Stager{}).Verify(ctx, binding.TargetLocator, digest); err != nil {
		return fmt.Errorf("%w: managed package was changed or is missing; refusing silent removal: %v", ErrInvalidRequest, err)
	}
	if receipt.DataReceiptID == "" {
		return nil
	}
	if err := (providers.PluginDataManager{Base: e.cfg.PluginDataBase}).ValidateData(ctx, receipt); err != nil {
		return fmt.Errorf("%w: required PLUGIN_DATA is missing or unreadable; refusing silent removal: %v", ErrInvalidRequest, err)
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
