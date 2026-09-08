package usecase

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

// Repair replaces a missing or modified managed directory from a freshly
// resolved package. It never uses a persisted target until that target has
// been matched to the target resolver's current safe result.
func (service Service) Repair(ctx context.Context, input AddInput) (AddResult, error) {
	if input.OriginMode == "" && input.DirectoryResolution != nil {
		input.OriginMode = domain.OriginModeDirectory
	}
	if err := validateOperationOrigin(input.OriginMode, input.DirectoryResolution); err != nil {
		return AddResult{}, err
	}
	if service.StateStore == nil || service.Planner == nil || service.Targets == nil || service.Stager == nil {
		return AddResult{}, fmt.Errorf("agentplugins repair dependencies are incomplete")
	}
	if input.ReleaseRevoked && normalizedOriginMode(input.OriginMode) == domain.OriginModeDirectory {
		return AddResult{}, fmt.Errorf("revoked release cannot be repaired or rematerialized; remove or update to a safe release")
	}
	release, err := service.beginMutation(ctx, input.DryRun, input.Confirmed)
	if err != nil {
		return AddResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	state, err := service.StateStore.Load()
	if err != nil {
		return AddResult{}, err
	}
	index, existing := findSourceInstallation(state, domain.ComputeSourceBindingID(input.Envelope.Source))
	if !existing {
		return AddResult{}, fmt.Errorf("resolved source is not bound to an installation")
	}
	installation := state.Installations[index]
	if installation.InstallationID != input.InstallationID || installation.DeclaredName != input.Envelope.Manifest.Name {
		return AddResult{}, fmt.Errorf("resolved repair source identity does not match the selected installation")
	}
	physicalID := domain.ComputePhysicalArtifactID(installation.DeclaredName, installation.InstallationID)
	plan, err := service.Planner.Plan(ctx, input.Envelope, input.Client, input.Scope, physicalID)
	if err != nil {
		return AddResult{}, err
	}
	result := AddResult{InstallationID: installation.InstallationID, Plan: plan}
	if err := service.preflightTargetComponents(ctx, input, &plan, &installation, true, false); err != nil {
		result.Plan = plan
		return result, err
	}
	result.Plan = plan
	clientKey := domain.ComputeClientBindingID(installation.InstallationID, string(input.Client.ClientID), string(input.Scope), plan.ActivePath)
	client, ok := installation.Clients[clientKey]
	if !ok && sameNativeBackend(input.Client.ClientID, domain.ClientCopilot) {
		for key, binding := range installation.Clients {
			if binding.Scope != string(input.Scope) || binding.Materialization == domain.MaterializationAbsent ||
				binding.PhysicalArtifact != plan.PhysicalArtifactID || !sameNativeBackend(domain.ClientID(binding.ClientID), input.Client.ClientID) {
				continue
			}
			clientKey, client, ok = key, binding, true
			plan.ActivePath = binding.TargetLocator
			plan.TargetRoot = filepath.Dir(binding.TargetLocator)
			result.Plan = plan
			break
		}
	}
	if !ok || (client.Materialization != domain.MaterializationMaterialized && client.Materialization != domain.MaterializationDegraded) {
		return result, fmt.Errorf("plugin is not materialized for %s", input.Client.ClientID)
	}
	if client.ClientBindingID != clientKey || !sameNativeBackend(domain.ClientID(client.ClientID), input.Client.ClientID) ||
		client.Scope != string(input.Scope) || client.PhysicalArtifact != physicalID {
		return result, fmt.Errorf("managed repair target identity does not match the selected binding")
	}
	result.Activation = lifecycleOutcome(client)
	if !repairPackageRevisionMatches(client.PackageRevision, input.Envelope, installation.OriginMode == domain.OriginModeDirectory) {
		return result, fmt.Errorf("resolved repair package differs from the installed revision; use update")
	}
	if installation.OriginMode == domain.OriginModeDirectory {
		if installation.Directory == nil || input.DirectoryResolution == nil || client.PackageRevision == nil ||
			client.PackageRevision.DistributionID != installation.Directory.DistributionID || client.PackageRevision.ReleaseSequence < 1 ||
			input.DirectoryResolution.DistributionID != client.PackageRevision.DistributionID || input.DirectoryResolution.DesiredReleaseSequence != client.PackageRevision.ReleaseSequence {
			return result, fmt.Errorf("repair requires the exact recorded Directory release sequence")
		}
	}
	if client.PackageRevision == nil || strings.TrimSpace(client.PackageRevision.ResolvedRevision) != strings.TrimSpace(input.Envelope.Source.ResolvedRevision) {
		return result, fmt.Errorf("resolved repair package does not match the exact installed revision")
	}
	targetClient := input.Client
	targetClient.ClientID = domain.ClientID(client.ClientID)
	target, err := service.Targets.ResolveTarget(ctx, targetClient, input.Scope, client.PhysicalArtifact)
	if err != nil {
		return result, fmt.Errorf("resolve managed repair target: %w", err)
	}
	if err := pathpolicy.RequireExactPath(target.ActivePath, client.TargetLocator); err != nil {
		return result, fmt.Errorf("refuse repair of untrusted persisted target: %w", err)
	}
	expectedDigest := managedDigest(client)
	if expectedDigest == "" {
		return result, fmt.Errorf("managed package digest is missing; refusing repair")
	}
	verifyErr := service.Stager.Verify(ctx, target.ActivePath, expectedDigest)
	if verifyErr == nil {
		verified, clientVerifyErr := service.verifyClientReadOnly(ctx, input, result, client)
		if clientVerifyErr != nil {
			if !nativeLifecycleClient(input.Client.ClientID) {
				return result, clientVerifyErr
			}
			// The package bytes are intact but an owned native projection is not.
			// A confirmed repair may reconstruct an absent exact-owned object. The
			// provider still observes the live object before any effect and rejects
			// foreign or tampered state.
			if input.DryRun {
				return result, nil
			}
			if !input.Confirmed {
				result.RequiresConfirmation = true
				return result, nil
			}
			delivery := domain.StagedDelivery{
				ClientID: input.Client.ClientID, OwnedBase: result.Plan.TargetRoot,
				ActivePath: client.TargetLocator, ArtifactDigest: expectedDigest,
				NativeObjects: append([]domain.NativeObjectOwnership(nil), client.NativeObjects...),
			}
			outcome, activationErr := service.Activator.Activate(ctx, domain.ActivationRequest{
				Client: input.Client, Plan: result.Plan, Delivery: delivery,
				DeclaredName: input.Envelope.Manifest.Name, Replacing: true,
				BackendExecutable:     input.BackendExecutable,
				PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), client.NativeObjects...),
			})
			outcome = preserveManagedAuthentication(outcome, client.Authentication)
			result.Activation = outcome
			if activationErr != nil {
				return result, fmt.Errorf("repair managed native state: %w", activationErr)
			}
			repairedClient := client
			repairedClient.Materialization = domain.MaterializationMaterialized
			repairedClient.Activation = outcome.Activation
			repairedClient.Authentication = outcome.Authentication
			repairedClient.Policy = outcome.Policy
			repairedClient.Verification = outcome.Verification
			repairedClient.UpdatedAt = service.now().Format("2006-01-02T15:04:05.999999999Z07:00")
			installation.Clients[clientKey] = repairedClient
			installation.UpdatedAt = repairedClient.UpdatedAt
			state.Installations[index] = installation
			if saveErr := service.StateStore.Save(state); saveErr != nil {
				return result, fmt.Errorf("persist repaired native lifecycle: %w", saveErr)
			}
			result.Mutated = true
			return result, nil
		}
		corrected := client
		corrected.Materialization = domain.MaterializationMaterialized
		if corrected.Verification == domain.VerificationFailed {
			corrected.Verification = domain.VerificationPackageValid
		}
		if verified.Activation != "" {
			corrected.Activation = verified.Activation
		}
		if verified.Authentication != "" {
			corrected.Authentication = verified.Authentication
		}
		if verified.Policy != "" {
			corrected.Policy = verified.Policy
		}
		if verified.Verification != "" {
			corrected.Verification = verified.Verification
		}
		result.Activation = lifecycleOutcome(corrected)
		if corrected.Materialization == client.Materialization && sameLifecycleOutcome(lifecycleOutcome(corrected), lifecycleOutcome(client)) {
			result.NoChange = true
			return result, nil
		}
		if input.DryRun {
			return result, nil
		}
		if !input.Confirmed {
			result.RequiresConfirmation = true
			return result, nil
		}
		corrected.UpdatedAt = service.now().Format("2006-01-02T15:04:05.999999999Z07:00")
		installation.Clients[clientKey] = corrected
		installation.UpdatedAt = corrected.UpdatedAt
		state.Installations[index] = installation
		if err := service.StateStore.Save(state); err != nil {
			return result, fmt.Errorf("persist corrected repair verification state: %w", err)
		}
		result.Mutated = true
		return result, nil
	}
	var verification *ports.VerificationError
	if !errors.As(verifyErr, &verification) || (verification.Kind != ports.VerificationAbsent && verification.Kind != ports.VerificationDigestMismatch) {
		return result, fmt.Errorf("managed package integrity could not be determined; refusing repair: %w", verifyErr)
	}
	beforeDigest := ""
	if verification.Kind == ports.VerificationDigestMismatch {
		if strings.TrimSpace(verification.ActualDigest) == "" {
			return result, fmt.Errorf("managed package verifier reported a digest mismatch without the actual digest; refusing repair")
		}
		beforeDigest = verification.ActualDigest
	}
	if input.DryRun {
		return result, nil
	}
	if !input.Confirmed {
		result.RequiresConfirmation = true
		return result, nil
	}
	operationID := strings.TrimSpace(input.OperationID)
	if operationID == "" {
		operationID, err = newOperationID()
		if err != nil {
			return result, err
		}
	}
	dataPath := ""
	if packageNeedsPluginData(input.Envelope, plan) {
		if service.PluginData == nil {
			return result, fmt.Errorf("PLUGIN_DATA manager is required for stdio repair")
		}
		receipt, ok := installation.DataReceipts[client.DataReceiptID]
		if !ok || receipt.DataReceiptID == "" {
			return result, fmt.Errorf("exact repair is missing its owned PLUGIN_DATA receipt; run reviewed state recovery")
		}
		if err := service.PluginData.ValidateData(ctx, receipt); err != nil {
			return result, fmt.Errorf("validate repair PLUGIN_DATA ownership: %w", err)
		}
		dataPath = receipt.Locator
	}
	delivery, err := service.stagePackage(ctx, input.Envelope, plan, operationID, input.Hints, dataPath)
	if err != nil {
		return result, err
	}
	defer func() { _ = service.Stager.Discard(context.Background(), delivery) }()
	delivery, err = bindStagedDeliveryToPhysicalOwner(delivery, plan, &client)
	if err != nil {
		return result, err
	}
	if delivery.ActivePath != target.ActivePath {
		return result, fmt.Errorf("stager returned an unexpected repair target")
	}
	if delivery.ArtifactDigest != expectedDigest {
		return result, fmt.Errorf("resolved repair projection digest differs from the originally managed package")
	}
	// The user or client can change the native object while staging runs. Repair
	// may replace the exact absent/digest-mismatched object reviewed above, but
	// never a different object that appeared after preflight.
	if err := service.verifyRepairPrecondition(ctx, target.ActivePath, expectedDigest, verification.Kind, beforeDigest); err != nil {
		return result, err
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	verifiedState := lifecycleOutcome(client)
	verifiedState.Verification = domain.VerificationPackageValid
	desiredClient := client
	desiredClient.Materialization = domain.MaterializationMaterialized
	desiredClient.Verification = domain.VerificationPackageValid
	desiredClient.NativeObjects = append([]domain.NativeObjectOwnership(nil), delivery.NativeObjects...)
	desiredClient.UpdatedAt = service.now().Format("2006-01-02T15:04:05.999999999Z07:00")
	installation.Clients[clientKey] = desiredClient
	installation.UpdatedAt = desiredClient.UpdatedAt
	state.Installations[index] = installation
	receipt, err := kernel.ApplyDirectory(ctx, transaction.DirectoryMutation{
		OperationID: operationID, InstallationID: installation.InstallationID, ClientBindingID: clientKey,
		Sequence: nextSequence(client), OwnedBase: delivery.OwnedBase, ActivePath: target.ActivePath,
		StagingPath: delivery.StagingPath, BeforeDigest: beforeDigest, AfterDigest: delivery.ArtifactDigest,
		NativeObjects: delivery.NativeObjects, Activation: verifiedState.Activation, Authentication: verifiedState.Authentication,
		Policy: verifiedState.Policy, Verification: verifiedState.Verification, DesiredState: state,
		Verify: func(verifyContext context.Context, activePath string) error {
			return service.Stager.Verify(verifyContext, activePath, delivery.ArtifactDigest)
		},
	})
	result.Receipt = receipt
	if err != nil {
		return result, err
	}
	result.Mutated = true
	result.Activation = verifiedState
	if nativeLifecycleClient(input.Client.ClientID) {
		outcome, activationErr := service.Activator.Activate(ctx, domain.ActivationRequest{
			Client: input.Client, Plan: plan,
			Delivery: domain.StagedDelivery{
				ClientID: delivery.ClientID, OwnedBase: delivery.OwnedBase,
				ActivePath: delivery.ActivePath, ArtifactDigest: delivery.ArtifactDigest,
				NativeObjects: append([]domain.NativeObjectOwnership(nil), delivery.NativeObjects...),
			},
			DeclaredName: input.Envelope.Manifest.Name, Replacing: true,
			BackendExecutable:     input.BackendExecutable,
			PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), client.NativeObjects...),
		})
		outcome = preserveManagedAuthentication(outcome, client.Authentication)
		result.Activation = outcome
		if _, updateErr := service.updateLifecycle(installation.InstallationID, clientKey, outcome); updateErr != nil {
			if activationErr != nil {
				return result, fmt.Errorf("reapply repaired native state: %v; persist verification state: %w", activationErr, updateErr)
			}
			return result, updateErr
		}
		if activationErr != nil {
			return result, fmt.Errorf("reapply repaired native state: %w", activationErr)
		}
	} else if input.Client.ClientID == domain.ClientClaude {
		outcome, activationErr := service.Activator.Activate(ctx, domain.ActivationRequest{
			Client: input.Client, Plan: plan,
			Delivery:     domain.StagedDelivery{ClientID: delivery.ClientID, OwnedBase: delivery.OwnedBase, ActivePath: delivery.ActivePath, ArtifactDigest: delivery.ArtifactDigest, NativeObjects: delivery.NativeObjects},
			DeclaredName: input.Envelope.Manifest.Name, Replacing: true, BackendExecutable: input.BackendExecutable, VerifyOnly: true,
		})
		outcome = preserveManagedAuthentication(outcome, client.Authentication)
		result.Activation = outcome
		if _, updateErr := service.updateLifecycle(installation.InstallationID, clientKey, outcome); updateErr != nil {
			if activationErr != nil {
				return result, fmt.Errorf("verify repaired Claude Code plugin: %v; persist verification state: %w", activationErr, updateErr)
			}
			return result, updateErr
		}
		if activationErr != nil {
			return result, activationErr
		}
	}
	return result, nil
}

func nativeLifecycleClient(clientID domain.ClientID) bool {
	switch clientID {
	case domain.ClientGemini, domain.ClientOpenCode, domain.ClientCline, domain.ClientWindsurf:
		return true
	default:
		return false
	}
}

func (service Service) verifyRepairPrecondition(ctx context.Context, activePath, managedDigest string, reviewedKind ports.VerificationKind, reviewedDigest string) error {
	err := service.Stager.Verify(ctx, activePath, managedDigest)
	var observed *ports.VerificationError
	if !errors.As(err, &observed) {
		if err == nil {
			return fmt.Errorf("managed native object changed after repair preflight; rerun repair")
		}
		return fmt.Errorf("revalidate managed native object before repair commit: %w", err)
	}
	if observed.Kind != reviewedKind {
		return fmt.Errorf("managed native object changed after repair preflight; rerun repair")
	}
	if reviewedKind == ports.VerificationDigestMismatch && (reviewedDigest == "" || observed.ActualDigest != reviewedDigest) {
		return fmt.Errorf("managed native object digest changed after repair preflight; rerun repair")
	}
	return nil
}

func sameLifecycleOutcome(left, right domain.ActivationOutcome) bool {
	return left.Activation == right.Activation && left.Authentication == right.Authentication &&
		left.Policy == right.Policy && left.Verification == right.Verification
}
