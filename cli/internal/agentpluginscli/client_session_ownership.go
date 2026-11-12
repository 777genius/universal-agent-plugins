package agentpluginscli

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func sessionBindings(ctx context.Context, app App, client domain.DetectedClient) ([]domain.ClientBinding, error) {
	state, err := app.StateStore.Load()
	if err != nil {
		return nil, err
	}
	for _, receipt := range state.TransactionReceipts {
		if receipt.Phase != "committed" {
			return nil, fmt.Errorf("recover pending installation before client startup")
		}
	}
	if app.Lifecycle.Kernel.Directory.JournalDir != "" {
		pending, listErr := app.Lifecycle.Kernel.Directory.ListOpen()
		if listErr != nil {
			return nil, listErr
		}
		if len(pending) != 0 {
			return nil, fmt.Errorf("recover open installation journal before client startup")
		}
	}
	result := []domain.ClientBinding{}
	for _, installation := range state.Installations {
		for key, binding := range installation.Clients {
			if binding.ClientID != string(client.ClientID) {
				continue
			}
			if key != binding.ClientBindingID || binding.Scope != "project" || binding.PendingNativeIntent != nil {
				return nil, fmt.Errorf("client binding is stale, unscoped or pending")
			}
			if err := verifySessionBinding(ctx, app, installation, binding, client); err != nil {
				return nil, err
			}
			result = append(result, binding)
		}
	}
	return result, nil
}

func verifySessionBinding(ctx context.Context, app App, installation domain.Installation, binding domain.ClientBinding, client domain.DetectedClient) error {
	artifact := domain.ComputePhysicalArtifactID(installation.DeclaredName, installation.InstallationID)
	if binding.PhysicalArtifact != artifact || !hasReconciledOwnershipReceipt(binding, artifact, installation.DeclaredName) {
		return fmt.Errorf("client package ownership receipt is not reconciled")
	}
	observer, ok := app.Lifecycle.NativeObserver.(usecase.PreparedIdentityObserver)
	if !ok || app.Lifecycle.Targets == nil {
		return fmt.Errorf("process-inert client ownership verification unavailable")
	}
	target, err := app.Lifecycle.Targets.ResolveTarget(ctx, client, domain.ScopeProject, artifact)
	if err != nil {
		return err
	}
	if target.ActivePath != binding.TargetLocator {
		return fmt.Errorf("client target differs from its owned package")
	}
	plan := domain.DeliveryPlan{ClientID: client.ClientID, Scope: domain.ScopeProject,
		DeclaredName: installation.DeclaredName, PhysicalArtifactID: artifact, ActivePath: target.ActivePath,
		TargetRoot: target.TargetRoot, TargetAnchor: target.TargetAnchor, TargetContext: target.TargetContext,
		NativeRegistryRoot: target.NativeRegistryRoot, InstallIntent: binding.InstallIntent}
	observed, err := observer.ObservePreparedIdentity(ctx, client, plan, &binding)
	if err != nil {
		return err
	}
	if observed.State != domain.NativeIdentityManaged || !observed.ReceiptReconciled {
		return fmt.Errorf("client package or native components are not exactly owned")
	}
	return nil
}
