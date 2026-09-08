package agentpluginscli

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func reconcileInstalledInfo(ctx context.Context, app App, installation domain.Installation, targetOption string, public *publicInstallation) error {
	targets, err := parseTargetOption(targetOption)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}
	selected := make(map[domain.ClientID]bool, len(targets))
	for _, target := range targets {
		selected[target] = true
	}
	clients, err := detectClientsForInfoReconciliation(ctx, app.Detector, targets)
	if err != nil {
		return err
	}
	detected := make(map[domain.ClientID]domain.DetectedClient, len(clients))
	for _, client := range clients {
		detected[client.ClientID] = client
	}

	filtered := make([]publicClient, 0, len(public.Clients))
	for _, view := range public.Clients {
		binding, bindingOK := installation.Clients[view.BindingID]
		if !bindingMatchesSelectedSurface(view, binding, bindingOK, selected) {
			continue
		}
		bindingClient, observerClient, detectedOK := reconciliationClientsForBinding(binding, detected)
		if !bindingOK || binding.ClientBindingID != view.BindingID || binding.ClientID != view.ClientID || binding.Scope != view.Scope {
			detectedOK = false
		}
		result := reconcileClientIdentity(ctx, app, installation, binding, bindingClient, observerClient, detectedOK)
		view.ReceiptReconciled = boolPointer(result.receiptReconciled)
		view.NativeDiscoveryReconciled = boolPointer(result.nativeDiscoveryReconciled)
		view.NativeIdentityState = result.state
		view.ClientVersion = result.clientVersion
		view.NativeDiscoveryEvidence = result.evidence
		filtered = append(filtered, view)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].ClientID != filtered[j].ClientID {
			return filtered[i].ClientID < filtered[j].ClientID
		}
		if filtered[i].Scope != filtered[j].Scope {
			return filtered[i].Scope < filtered[j].Scope
		}
		return filtered[i].BindingID < filtered[j].BindingID
	})
	public.Clients = filtered
	return nil
}

func bindingMatchesSelectedSurface(view publicClient, binding domain.ClientBinding, bindingOK bool, selected map[domain.ClientID]bool) bool {
	if selected[domain.ClientID(view.ClientID)] {
		return true
	}
	if bindingOK {
		for _, surface := range bindingSurfaceTargets(binding) {
			if selected[surface] {
				return true
			}
		}
		return false
	}
	for _, surface := range view.AffectedSurfaces {
		if selected[domain.ClientID(surface)] {
			return true
		}
	}
	return false
}

func reconciliationClientsForBinding(binding domain.ClientBinding, detected map[domain.ClientID]domain.DetectedClient) (domain.DetectedClient, domain.DetectedClient, bool) {
	owner := domain.ClientID(binding.ClientID)
	if owner != domain.ClientCopilot && owner != domain.ClientVSCode {
		client, ok := detected[owner]
		return client, client, ok
	}
	copilot, ok := detected[domain.ClientCopilot]
	if !ok || copilot.Status != domain.DetectionDetected || strings.TrimSpace(copilot.ExecutablePath) == "" {
		return domain.DetectedClient{ClientID: owner}, domain.DetectedClient{ClientID: owner}, false
	}
	// The persisted owner selects the exact managed path while Copilot CLI is
	// the authoritative native registry for both Copilot and VS Code surfaces.
	copilot.ClientID = owner
	return copilot, copilot, true
}

type clientIdentityReconciliation struct {
	receiptReconciled         bool
	nativeDiscoveryReconciled bool
	state                     domain.NativeIdentityState
	clientVersion             string
	evidence                  *publicNativeDiscoveryEvidence
}

func detectClientsForInfoReconciliation(ctx context.Context, detector ports.ClientDetector, targets []domain.ClientID) ([]domain.DetectedClient, error) {
	probeSet := make(map[domain.ClientID]struct{}, len(targets))
	for _, target := range targets {
		if target == domain.ClientVSCode {
			target = domain.ClientCopilot
		}
		probeSet[target] = struct{}{}
	}
	probeTargets := make([]domain.ClientID, 0, len(probeSet))
	for target := range probeSet {
		probeTargets = append(probeTargets, target)
	}
	sort.Slice(probeTargets, func(i, j int) bool { return probeTargets[i] < probeTargets[j] })
	if targeted, ok := detector.(ports.TargetedVersionProbingClientDetector); ok {
		return targeted.DetectTargetsWithVersionProbe(ctx, probeTargets)
	}
	return detector.Detect(ctx)
}

func reconcileClientIdentity(ctx context.Context, app App, installation domain.Installation, binding domain.ClientBinding, bindingClient, observerClient domain.DetectedClient, detected bool) clientIdentityReconciliation {
	result := clientIdentityReconciliation{state: domain.NativeIdentityIndeterminate}
	if detected {
		result.clientVersion = strings.TrimSpace(observerClient.Version)
	}
	if !detected || bindingClient.Status != domain.DetectionDetected || app.Lifecycle.Targets == nil || app.Lifecycle.NativeObserver == nil {
		return result
	}
	if strings.TrimSpace(observerClient.Version) == "" {
		return result
	}
	expectedArtifact := domain.ComputePhysicalArtifactID(installation.DeclaredName, installation.InstallationID)
	if strings.TrimSpace(binding.PhysicalArtifact) != expectedArtifact {
		return result
	}
	if !hasReconciledOwnershipReceipt(binding, expectedArtifact, installation.DeclaredName) {
		return result
	}
	declaredVersion := installation.Package.Version
	if binding.PackageRevision != nil {
		declaredVersion = binding.PackageRevision.Version
	}
	target, err := app.Lifecycle.Targets.ResolveTarget(ctx, bindingClient, domain.InstallScope(binding.Scope), expectedArtifact)
	if err != nil || filepath.Clean(target.ActivePath) != filepath.Clean(binding.TargetLocator) {
		return result
	}
	plan := domain.DeliveryPlan{
		ClientID: bindingClient.ClientID, Scope: domain.InstallScope(binding.Scope), DeclaredName: installation.DeclaredName,
		DeclaredVersion:    declaredVersion,
		PhysicalArtifactID: expectedArtifact, TargetAnchor: target.TargetAnchor, TargetRoot: target.TargetRoot, ActivePath: target.ActivePath,
		NativeRegistryRoot: observerClient.ConfigRoot, NativeRegistryExecutable: observerClient.ExecutablePath,
	}
	observation, observeErr := app.Lifecycle.NativeObserver.ObserveNativeIdentity(ctx, observerClient, plan, &binding)
	result.receiptReconciled = observation.ReceiptReconciled
	result.nativeDiscoveryReconciled = observation.NativeDiscoveryReconciled
	if observation.State != "" {
		result.state = observation.State
	}
	if observeErr != nil {
		return result
	}
	if observation.NativeDiscoveryAttempted {
		result.evidence = nativeCommandEvidence(observerClient, installation.DeclaredName, expectedArtifact, result.clientVersion, result.nativeDiscoveryReconciled)
	}
	if result.state == domain.NativeIdentityManaged && !result.nativeDiscoveryReconciled && observation.NativeDiscoveryState != "" {
		result.state = observation.NativeDiscoveryState
	}
	return result
}

func hasReconciledOwnershipReceipt(binding domain.ClientBinding, physicalArtifact, declaredName string) bool {
	digest := ""
	expectedObjectID := "package:" + binding.ClientID + ":" + physicalArtifact
	for _, object := range binding.NativeObjects {
		if object.Kind != "managed_package_directory" {
			continue
		}
		if digest != "" || object.ObjectID != expectedObjectID || object.LogicalName != declaredName || strings.TrimSpace(object.ManagedDigest) == "" {
			return false
		}
		digest = object.ManagedDigest
	}
	if digest == "" {
		return false
	}
	latestSequence := 0
	latestDigest := ""
	latestPhase := ""
	sequences := make(map[int]struct{}, len(binding.Receipts))
	for _, receipt := range binding.Receipts {
		if receipt.ClientBindingID != binding.ClientBindingID || receipt.Sequence < 1 || strings.TrimSpace(receipt.OperationID) == "" {
			return false
		}
		if _, duplicate := sequences[receipt.Sequence]; duplicate {
			return false
		}
		sequences[receipt.Sequence] = struct{}{}
		if receipt.Sequence > latestSequence {
			latestSequence = receipt.Sequence
			latestDigest = strings.TrimSpace(receipt.AfterDigest)
			latestPhase = strings.TrimSpace(receipt.Phase)
		}
	}
	return latestSequence > 0 && latestDigest == digest && latestPhase == "committed"
}

func nativeCommandEvidence(client domain.DetectedClient, declaredName, physicalArtifact, version string, discovered bool) *publicNativeDiscoveryEvidence {
	if (client.ClientID != domain.ClientCopilot && client.ClientID != domain.ClientVSCode) ||
		strings.TrimSpace(client.ExecutablePath) == "" || strings.TrimSpace(physicalArtifact) == "" {
		return nil
	}
	return &publicNativeDiscoveryEvidence{
		Basis: "native_client_command",
		VersionOperation: publicVersionOperation{
			Argv: []string{"copilot", "--version"}, ObservedClientVersion: version,
		},
		DiscoveryOperation: publicDiscoveryOperation{
			Argv: []string{"copilot", "plugin", "list"}, Discovered: discovered,
			ProductID: strings.TrimSpace(declaredName) + "@" + providers.ManagedMarketplaceName(physicalArtifact),
		},
	}
}

func boolPointer(value bool) *bool { return &value }
