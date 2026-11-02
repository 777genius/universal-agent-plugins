package kiro

import (
	"context"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const KiroProjectAction = "Open the selected Kiro workspace, review workspace trust and the active custom agent's includeMcpJson/resources, then inspect skills and MCP connections. Configuration delivery alone does not establish native discovery, IDE behavior, permissions or MCP initialization."

func preflightKiroProject(env clients.Env, request domain.ActivationRequest) error {
	root, err := kiroProjectPlanRoot(request.Client, request.Plan)
	if err != nil {
		return err
	}
	if request.Delivery.ActivePath != "" && request.Delivery.ActivePath != request.Plan.ActivePath {
		return fmt.Errorf("kiro delivered package differs from selected project")
	}
	if request.Plan.ClientID != request.Client.ClientID {
		return fmt.Errorf("kiro project plan client identity mismatch")
	}
	if request.Delivery.ClientID != "" {
		if err := shared.ActivationIdentityMismatch(request); err != nil {
			return err
		}
	}
	if request.ActivationComplete {
		return fmt.Errorf("kiro project native discovery cannot be established by activation attestation")
	}
	if !shared.OnlyNativeComponents(request.Plan.Components) {
		return fmt.Errorf("kiro project delivery requires supported skills and MCP servers only")
	}
	if err := validateKiroProjectObjects(root, request.PreviousNativeObjects); err != nil {
		return err
	}
	if err := validateKiroProjectObjects(root, request.Delivery.NativeObjects); err != nil {
		return err
	}
	return env.NativeConfig.RequireFileIO()
}

func activateKiroProject(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	outcome := shared.StartedActivation(request)
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return outcome, err
	}
	if request.Delivery.ActivePath != request.Plan.ActivePath {
		return outcome, fmt.Errorf("kiro delivered package differs from selected project")
	}
	outcome.NativeEffect, outcome.NativeObjects = domain.NativeEffectUnchanged, append([]domain.NativeObjectOwnership(nil), request.PreviousNativeObjects...)
	if err := preflightKiroProject(env, request); err != nil {
		return outcome, err
	}
	if err := ctx.Err(); err != nil {
		return outcome, err
	}
	root := request.Plan.NativeRegistryRoot
	if request.VerifyOnly {
		if err := VerifyNativeObjects(root, request.Delivery.NativeObjects, false); err != nil {
			return shared.FailedActivation(outcome, "repair managed Kiro workspace configuration", err)
		}
		outcome.NativeObjects = append([]domain.NativeObjectOwnership(nil), request.Delivery.NativeObjects...)
	} else {
		effect, err := applyKiroNativeMutationWithKernelAndOps(root, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, env.NativeConfig, shared.RenameDirectoryExclusive, os.RemoveAll)
		outcome.NativeEffect = effect
		outcome.NativeObjects = kiroProjectEffectObjects(root, effect, request)
		if err != nil && !shared.CommittedNativeCleanup(&outcome, err) {
			return shared.FailedActivation(outcome, "repair managed Kiro workspace configuration", err)
		}
	}
	outcome.Activation, outcome.Verification = domain.ActivationPrepared, domain.VerificationInstalled
	outcome.UserActions = shared.AppendUnique(outcome.UserActions, KiroProjectAction)
	return outcome, nil
}

func kiroProjectEffectObjects(root string, effect domain.NativeEffectState, request domain.ActivationRequest) []domain.NativeObjectOwnership {
	if effect == domain.NativeEffectCommitted {
		return append([]domain.NativeObjectOwnership(nil), request.Delivery.NativeObjects...)
	}
	if effect == domain.NativeEffectUnchanged {
		return append([]domain.NativeObjectOwnership(nil), request.PreviousNativeObjects...)
	}
	var retained []domain.NativeObjectOwnership
	for _, object := range NativeObjects(request.PreviousNativeObjects) {
		if VerifyNativeObjects(root, []domain.NativeObjectOwnership{object}, false) == nil {
			retained = append(retained, object)
		}
	}
	return retained
}

func deactivateKiroProject(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	root, err := kiroProjectRoot(request.Client, request.TargetContext, request.NativeRegistryRoot)
	if err != nil {
		return outcome, err
	}
	if err := validateKiroProjectObjects(root, request.NativeObjects); err != nil {
		return outcome, err
	}
	if err := env.NativeConfig.RequireFileIO(); err != nil {
		return outcome, err
	}
	if !request.Confirmed {
		outcome.UserActions = []string{"agentplugins will remove its exact owned Kiro workspace skill directories and MCP entries"}
		return outcome, nil
	}
	if err := ctx.Err(); err != nil {
		return outcome, err
	}
	_, err = applyKiroNativeMutationWithKernelAndOps(root, "", request.NativeObjects, nil, env.NativeConfig, shared.RenameDirectoryExclusive, os.RemoveAll)
	if err != nil && !shared.CommittedNativeDeactivationCleanup(&outcome, err) {
		return outcome, err
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
