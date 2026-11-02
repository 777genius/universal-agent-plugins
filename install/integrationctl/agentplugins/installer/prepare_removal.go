package installer

import (
	"context"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

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
	if err := e.removalPreflight(ctx, client, installation.DeclaredName, binding, receipt); err != nil {
		return nil, err
	}
	handle := &PreparedOperation{engine: e, req: req, client: client, detected: detected, recorded: state}
	helperVersion, helperDigest := e.helperIdentity()
	handle.plan = Plan{ProfileAuthority: domain.CloneProfileAuthority(client.ProfileAuthority),
		Operation: OpRemove, ClientID: string(client.ClientID), ConfigRoot: client.ConfigRoot,
		TargetPath: binding.TargetLocator, InstallationID: installation.InstallationID,
		BindingID: binding.ClientBindingID, HelperVersion: helperVersion, HelperDigest: helperDigest,
		SelectedDelivery: binding.SelectedDelivery, TreeDigest: recordedBindingDigest(binding, installation.Source.TreeDigest),
		Client:                      liveClientResult(binding, req.RequiredComponents, installation.Source.TreeDigest),
		RequiresExternalUninstalled: e.removalRequiresExternalUninstalled(client, installation, binding, req.ClientExecutable),
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
func (e *Engine) removalPreflight(ctx context.Context, client domain.DetectedClient, declaredName string, binding domain.ClientBinding, receipt domain.DataReceipt) error {
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
	target, err := e.planner().ResolveTarget(ctx, client, e.scope(), binding.PhysicalArtifact)
	if err != nil {
		return fmt.Errorf("%w: resolve managed removal target: %w", ErrInvalidRequest, err)
	}
	if err := pathpolicy.RequireExactPath(target.ActivePath, binding.TargetLocator); err != nil {
		return fmt.Errorf("%w: refuse removal from untrusted persisted target: %w", ErrInvalidRequest, err)
	}
	if err := (providers.Stager{}).Verify(ctx, binding.TargetLocator, digest); err != nil {
		return fmt.Errorf("%w: managed package was changed or is missing; refusing silent removal: %w", ErrInvalidRequest, err)
	}
	if err := e.confirmProjectRemovalFiles(ctx, client, declaredName, binding, target, digest); err != nil {
		return err
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
