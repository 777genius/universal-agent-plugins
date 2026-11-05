package windsurf

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const DevinGlobalAction = "Devin Global files are installed in the selected original user profile; inspect Skills and stdio/HTTP MCP names with the genuine Devin CLI. Authentication, tool initialization, tools/list, tools/call and effective policy remain unobserved"

func preflightDevinGlobal(env clients.Env, request domain.ActivationRequest) error {
	root, err := devinGlobalPlanRoot(request.Client, request.Plan)
	if err != nil {
		return err
	}
	if err := validateDevinGlobalProjectionRoot(request.Plan.ActivePath, request.Plan); err != nil {
		return err
	}
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return err
	}
	if len(WindsurfObjects(request.PreviousNativeObjects)) != 0 ||
		len(WindsurfObjects(request.Delivery.NativeObjects)) != 0 {
		return fmt.Errorf("legacy Cascade receipt cannot be replayed as Devin Global")
	}
	if request.Delivery.ActivePath != request.Plan.ActivePath {
		return fmt.Errorf("Devin Global delivered package differs from selected managed root")
	}
	if request.ActivationComplete {
		return fmt.Errorf("Devin Global native discovery cannot be established by activation attestation")
	}
	if !shared.OnlyNativeComponents(request.Plan.Components) {
		return fmt.Errorf("Devin Global supports only selected Skills and MCP components")
	}
	layout := devinGlobalComponents(root, request.Plan.ActivePath)
	if err := layout.ValidateObjects(devinNativeObjects(request.PreviousNativeObjects)); err != nil {
		return err
	}
	if err := layout.ValidateObjects(devinNativeObjects(request.Delivery.NativeObjects)); err != nil {
		return err
	}
	return checkDevinProjectPolicy(root, request.Plan.Components, env.NativeConfig)
}

func (adapter *Adapter) activateDevinGlobal(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := preflightDevinGlobal(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	layout := devinGlobalComponents(request.Plan.NativeRegistryRoot, request.Plan.ActivePath)
	outcome, err := shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic: adapter.AutomaticallyActivates(env, request),
		UnavailableAction: "select only supported Devin Global Skills and MCP servers",
		RepairAction: "repair exact owned Devin Global Skills and MCP entries",
		RetryAction: "retry selected Devin Global native component installation",
		CompletedAction: DevinGlobalAction,
		Verify: func() error {
			return layout.Verify(devinNativeObjects(request.Delivery.NativeObjects), false, env.NativeConfig)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			servers, err := devinDesiredServersForKind(request.Delivery.ActivePath, devinNativeObjects(request.Delivery.NativeObjects), DevinGlobalMCPKind)
			if err != nil {
				return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
			}
			return (shared.ProjectComponentMutation{
				Layout: layout, ActivePath: request.Delivery.ActivePath,
				Previous: devinNativeObjects(request.PreviousNativeObjects),
				Desired: devinNativeObjects(request.Delivery.NativeObjects),
				Servers: servers, Kernel: env.NativeConfig,
			}).Apply(ctx)
		},
	})
	if err == nil && outcome.Verification == domain.VerificationInstalled {
		outcome.Activation, outcome.ActivationAttested = domain.ActivationPrepared, false
		outcome.UserActions = shared.AppendUnique(outcome.UserActions, DevinGlobalAction)
	}
	return outcome, err
}

// Global removal is receipt-bound and never invokes a native executable.
// It requires no currently available version/login, even after uninstall.
func deactivateDevinGlobal(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	root := request.NativeRegistryRoot
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Base(root) != "devin" ||
		root == request.Client.ConfigRoot {
		return outcome, fmt.Errorf("recorded Devin Global native root differs from legacy or is invalid")
	}
	if err := pathpolicy.ValidateLeafID(request.PhysicalArtifactID); err != nil {
		return outcome, err
	}
	if request.ManagedArtifactPath != "" && filepath.Base(request.ManagedArtifactPath) != request.PhysicalArtifactID {
		return outcome, fmt.Errorf("recorded Devin Global package differs from selected artifact")
	}
	layout := devinGlobalComponents(root, request.ManagedArtifactPath)
	objects := devinNativeObjects(request.NativeObjects)
	if !hasDevinGlobalObjects(objects) {
		return outcome, fmt.Errorf("Devin Global removal has no recorded native ownership")
	}
	if err := layout.ValidateObjects(objects); err != nil {
		return outcome, err
	}
	if err := env.NativeConfig.RequireFileIO(); err != nil {
		return outcome, err
	}
	if !request.Confirmed {
		outcome.UserActions = []string{"agentplugins will remove only exact owned Devin Global Skills and MCP entries"}
		return outcome, nil
	}
	if err := pathpolicy.RequireContainedChild(filepath.Dir(root), root); err != nil {
		return outcome, err
	}
	err := (shared.ProjectComponentMutation{Layout: layout, Previous: objects, Kernel: env.NativeConfig}).Apply(ctx)
	if err != nil && !shared.CommittedNativeDeactivationCleanup(&outcome, err) {
		return outcome, err
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
