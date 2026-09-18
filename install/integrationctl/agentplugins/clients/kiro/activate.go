package kiro

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// AutomaticallyActivates reports that Activate will write managed Kiro native
// objects for this exact request. MCP packages also need a Kiro CLI name.
func (*Adapter) AutomaticallyActivates(_ clients.Env, request domain.ActivationRequest) bool {
	return automaticallyActivates(request)
}

func automaticallyActivates(request domain.ActivationRequest) bool {
	if request.Plan.InstallIntent != domain.InstallIntentAutomatic {
		return false
	}
	if strings.TrimSpace(request.Client.ConfigRoot) == "" || !shared.OnlyNativeComponents(request.Plan.Components) {
		return false
	}
	return !shared.HasSupportedMCP(request.Plan.Components) ||
		(strings.TrimSpace(request.BackendExecutable) != "" && isKiroCLI(request.BackendExecutable))
}

// VerifierAvailable reports that an exact Kiro native/ACP observation can be
// made for this plan. The executable name is part of the evidence: a backend
// that does not look like Kiro cannot prove Kiro state.
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, backendExecutable string) bool {
	if strings.TrimSpace(backendExecutable) == "" {
		return false
	}
	if !strings.Contains(strings.ToLower(backendExecutable), "kiro") || len(plan.Components) == 0 {
		return false
	}
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if component.Kind != domain.ComponentSkill && component.Kind != domain.ComponentMCPServer {
			return false
		}
	}
	return true
}

// PreflightActivation rejects a prepare request that cannot own native objects,
// and rejects automatic MCP activation whose runner cannot speak ACP duplex.
func (*Adapter) PreflightActivation(env clients.Env, request domain.ActivationRequest) error {
	if request.Plan.InstallIntent == domain.InstallIntentPrepare {
		if request.Plan.Scope != domain.ScopeUser || strings.TrimSpace(request.Client.ConfigRoot) == "" || !shared.OnlyNativeComponents(request.Plan.Components) {
			return kiroErrorf("Kiro preparation requires native configuration and supported components")
		}
		return nil
	}
	if !automaticallyActivates(request) || !shared.HasSupportedMCP(request.Plan.Components) {
		return nil
	}
	runner, ok := env.Runner.(ports.DuplexCapabilityRunner)
	if !ok {
		return fmt.Errorf("manual_activation_required: automatic native Kiro MCP lifecycle requires an ACP duplex process runner with capability preflight")
	}
	if err := runner.DuplexCapability(); err != nil {
		return fmt.Errorf("manual_activation_required: automatic native Kiro MCP lifecycle containment preflight failed: %w", err)
	}
	return nil
}

// Activate prepares, verifies or installs managed Kiro native objects, then
// observes MCP through ACP when the package selected any.
func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	if request.Plan.InstallIntent == domain.InstallIntentPrepare {
		return activatePrepared(ctx, request, outcome)
	}
	if !automaticallyActivates(request) {
		return manualKiroInstall(request, outcome), nil
	}
	if request.VerifyOnly {
		return verifyKiroInstall(ctx, env, request, outcome)
	}
	return activateAutomatic(ctx, env, request, outcome)
}

func activatePrepared(ctx context.Context, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	var err error
	if request.VerifyOnly {
		err = VerifyNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false)
	} else {
		err = ActivateNative(ctx, request)
	}
	if err != nil {
		return shared.FailedActivation(outcome, "repair the managed Kiro native configuration", err)
	}
	outcome.Activation = domain.ActivationPrepared
	outcome.UserActions = append(outcome.UserActions, request.Plan.UserActions...)
	return outcome, nil
}

func manualKiroInstall(request domain.ActivationRequest, outcome domain.ActivationOutcome) domain.ActivationOutcome {
	outcome.Activation = domain.ActivationManual
	outcome.UserActions = append(outcome.UserActions, "install a current Kiro CLI and rerun add to register the package's skills and MCP servers")
	outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("Kiro native installation requires a writable config root and, for MCP packages, a complete Kiro CLI distribution at %s", request.Delivery.ActivePath))
	return outcome
}

func verifyKiroInstall(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if err := VerifyNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false); err != nil {
		return shared.FailedActivation(outcome, "repair the managed Kiro skills and MCP configuration", err)
	}
	if err := verifyKiroMCP(ctx, env, request); err != nil {
		return kiroACPFailure(outcome, request, err, "rerun structured Kiro ACP verification with `%s acp --agent-engine v3 --auth-method cli`")
	}
	outcome.Activation = domain.ActivationActive
	outcome.Verification = domain.VerificationInstalled
	return outcome, nil
}

func activateAutomatic(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if err := ActivateNative(ctx, request); err != nil {
		return shared.FailedActivation(outcome, "retry the managed Kiro native installation", err)
	}
	if err := verifyKiroMCP(ctx, env, request); err != nil {
		return kiroACPFailure(outcome, request, err, "retry structured Kiro ACP verification with `%s acp --agent-engine v3 --auth-method cli`")
	}
	outcome.Activation = domain.ActivationActive
	outcome.Verification = domain.VerificationInstalled
	return outcome, nil
}

func kiroACPFailure(outcome domain.ActivationOutcome, request domain.ActivationRequest, err error, next string) (domain.ActivationOutcome, error) {
	if err == nil {
		return outcome, nil
	}
	if errors.Is(err, ErrACPContractUnknown) {
		return manualKiroVerification(outcome, request), nil
	}
	return shared.FailedActivation(outcome, fmt.Sprintf(next, request.BackendExecutable), err)
}

func verifyKiroMCP(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	if !shared.HasSupportedMCP(request.Plan.Components) {
		return nil
	}
	runner, ok := env.Runner.(ports.DuplexCommandRunner)
	if !ok {
		return fmt.Errorf("%w: the process runner does not support an ACP duplex exchange", ErrACPContractUnknown)
	}
	return VerifyACP(ctx, runner, request.BackendExecutable, request.Delivery.ActivePath, shared.SupportedMCPNames(request.Plan))
}

func manualKiroVerification(outcome domain.ActivationOutcome, request domain.ActivationRequest) domain.ActivationOutcome {
	if attested, ok := shared.AttestedUnknownVerification(outcome, request); ok {
		return attested
	}
	outcome.Activation = domain.ActivationManual
	outcome.UserActions = []string{"confirm each imported MCP server is connected in Kiro"}
	outcome.LocalActions = []string{fmt.Sprintf("Kiro structured ACP verification via `%s acp --agent-engine v3 --auth-method cli` was unavailable or unrecognized; ensure the companion kiro-cli-chat is installed, then inspect each imported server manually", request.BackendExecutable)}
	return outcome
}

// Deactivate removes managed Kiro native objects, or asks the operator to
// finish a leftover custom Power first.
func (*Adapter) Deactivate(ctx context.Context, _ clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if len(NativeObjects(request.NativeObjects)) == 0 {
		return shared.RequireExternalUninstall(outcome, request.ExternalUninstalled, "remove the legacy custom Power in Kiro, then rerun remove with `--external-uninstalled`"), nil
	}
	if !request.Confirmed {
		outcome.UserActions = append(outcome.UserActions, "agentplugins will remove its managed Kiro skills and MCP entries automatically")
		return outcome, nil
	}
	if err := DeactivateNative(ctx, request); err != nil {
		return outcome, err
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}

func isKiroCLI(executable string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(executable)))
	return base == "kiro-cli" || base == "kiro-cli.exe" || base == "kiro" || base == "kiro.exe"
}
