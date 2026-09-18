package providers

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type Activator struct {
	Runner       ports.CommandRunner
	NativeConfig *nativeconfig.Kernel
	// Registry supplies the client adapters that own lifecycle. It is injected
	// by the composition root and never defaulted to "every client".
	Registry *clients.Registry
}

func (activator Activator) requireRegistry() error {
	if activator.Registry == nil {
		return clients.ErrRegistryRequired
	}
	return nil
}

func (activator Activator) env() clients.Env {
	return clients.Env{Runner: activator.Runner, NativeConfig: activator.nativeConfigKernel()}
}

func (activator Activator) nativeConfigKernel() nativeconfig.Kernel {
	if activator.NativeConfig != nil {
		return *activator.NativeConfig
	}
	return nativeconfig.New()
}

// AutomaticallyActivates reports whether Activate will use a managed client
// CLI for this exact request. Runtime preflight consumes this same predicate so
// it cannot drift from the provider's activation paths.
func (activator Activator) AutomaticallyActivates(request domain.ActivationRequest) bool {
	if request.Plan.InstallIntent != domain.InstallIntentAutomatic {
		return false
	}
	if activator.requireRegistry() != nil {
		return false
	}
	automatic, ok := clients.As[clients.AutomaticActivator](activator.Registry, request.Client.ClientID)
	if !ok {
		return false
	}
	return automatic.AutomaticallyActivates(activator.env(), request)
}

// PreflightActivation rejects lifecycle configurations that would otherwise
// discover a missing required capability only after native client mutation.
func (activator Activator) PreflightActivation(request domain.ActivationRequest) error {
	if err := request.Plan.InstallIntent.Validate(request.Client.ClientID); err != nil {
		return err
	}
	if err := activator.requireRegistry(); err != nil {
		return err
	}
	if preflight, ok := clients.As[clients.ActivationPreflighter](activator.Registry, request.Client.ClientID); ok {
		return preflight.PreflightActivation(activator.env(), request)
	}
	return nil
}

func (activator Activator) Deactivate(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return domain.DeactivationOutcome{}, err
	}
	if err := activator.requireRegistry(); err != nil {
		return domain.DeactivationOutcome{}, err
	}
	if lifecycle, ok := clients.As[clients.Lifecycle](activator.Registry, request.Client.ClientID); ok {
		return lifecycle.Deactivate(ctx, activator.env(), request)
	}
	return domain.DeactivationOutcome{}, fmt.Errorf("unsupported deactivation client %q", request.Client.ClientID)
}

func (activator Activator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := activator.requireRegistry(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if request.Plan.ActivePath != request.Delivery.ActivePath {
		return domain.ActivationOutcome{}, fmt.Errorf("activation artifact path mismatch")
	}
	if strings.TrimSpace(request.DeclaredName) == "" {
		return domain.ActivationOutcome{}, fmt.Errorf("activation plugin name is required")
	}
	if err := activator.PreflightActivation(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := pathpolicy.RequireContainedChild(request.Delivery.OwnedBase, request.Delivery.ActivePath); err != nil {
		return domain.ActivationOutcome{}, fmt.Errorf("unsafe activation artifact: %w", err)
	}
	info, err := os.Lstat(request.Delivery.ActivePath)
	if err != nil {
		return domain.ActivationOutcome{}, fmt.Errorf("inspect activation artifact: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return domain.ActivationOutcome{}, fmt.Errorf("activation artifact must be a real directory")
	}
	if request.Plan.InstallIntent == domain.InstallIntentPrepare {
		return activator.dispatchActivate(ctx, request)
	}
	if request.ActivationComplete && !activator.AutomaticallyActivates(request) {
		return domain.ActivationOutcome{
			Authentication:     request.Plan.Authentication,
			Policy:             domain.PolicyAllowed,
			Activation:         domain.ActivationActive,
			Verification:       domain.VerificationInstalled,
			ActivationAttested: true,
		}, nil
	}
	return activator.dispatchActivate(ctx, request)
}

func (activator Activator) dispatchActivate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if lifecycle, ok := clients.As[clients.Lifecycle](activator.Registry, request.Client.ClientID); ok {
		return lifecycle.Activate(ctx, activator.env(), request)
	}
	return domain.ActivationOutcome{}, fmt.Errorf("unsupported activation client %q", request.Client.ClientID)
}
