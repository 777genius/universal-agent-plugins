package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// CommandRunner is an alias of the port. It stays exported here so callers and
// the optional runner capabilities keep their current names until the rest of
// the providers package is split into client adapters.
type CommandRunner = ports.CommandRunner

type Activator struct {
	Runner       CommandRunner
	NativeConfig *nativeconfig.Kernel
	// Registry supplies the client adapters that own lifecycle. It is injected
	// by the composition root and never defaulted to "every client".
	Registry *clients.Registry
}

var errActivatorRegistryRequired = errors.New("activator registry is required")

func (activator Activator) requireRegistry() error {
	if activator.Registry == nil {
		return errActivatorRegistryRequired
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

func committedNativeCleanup(outcome *domain.ActivationOutcome, err error) bool {
	if !nativeconfig.IsCommittedCleanup(err) {
		return false
	}
	outcome.UserActions = append(outcome.UserActions, "native config was committed, but lock cleanup degraded; retry repair if the next operation reports a busy lock")
	return true
}

func committedNativeDeactivationCleanup(outcome *domain.DeactivationOutcome, err error) bool {
	if !nativeconfig.IsCommittedCleanup(err) {
		return false
	}
	outcome.UserActions = append(outcome.UserActions, "native config removal was committed, but lock cleanup degraded; retry removal if the next operation reports a busy lock")
	return true
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
	if automatic, ok := clients.As[clients.AutomaticActivator](activator.Registry, request.Client.ClientID); ok {
		return automatic.AutomaticallyActivates(activator.env(), request)
	}
	switch request.Client.ClientID {
	case domain.ClientCline:
		return strings.TrimSpace(request.Client.ConfigRoot) != "" && shared.OnlyNativeComponents(request.Plan.Components)
	case domain.ClientOpenCode:
		return strings.TrimSpace(request.Client.ConfigRoot) != "" && shared.OnlyNativeComponents(request.Plan.Components)
	case domain.ClientGemini:
		return strings.TrimSpace(request.Client.ConfigRoot) != "" && shared.OnlyNativeComponents(request.Plan.Components)
	case domain.ClientWindsurf:
		return strings.TrimSpace(request.Client.ConfigRoot) != "" && len(windsurf.WindsurfObjects(request.Delivery.NativeObjects)) > 0
	}
	if request.Client.ClientID == domain.ClientKiro {
		if strings.TrimSpace(request.Client.ConfigRoot) == "" || !shared.OnlyNativeComponents(request.Plan.Components) {
			return false
		}
		return !shared.HasSupportedMCP(request.Plan.Components) ||
			(strings.TrimSpace(request.BackendExecutable) != "" && isKiroCLI(request.BackendExecutable))
	}
	return activationObservable(request, activator.Runner)
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
	if request.Plan.InstallIntent == domain.InstallIntentPrepare {
		if request.Plan.Scope != domain.ScopeUser || strings.TrimSpace(request.Client.ConfigRoot) == "" || !shared.OnlyNativeComponents(request.Plan.Components) {
			return fmt.Errorf("Kiro preparation requires native configuration and supported components")
		}
		return nil
	}
	if !activator.AutomaticallyActivates(request) || request.Client.ClientID != domain.ClientKiro || !shared.HasSupportedMCP(request.Plan.Components) {
		return nil
	}
	runner, ok := activator.Runner.(ports.DuplexCapabilityRunner)
	if !ok {
		return fmt.Errorf("manual_activation_required: automatic native Kiro MCP lifecycle requires an ACP duplex process runner with capability preflight")
	}
	if err := runner.DuplexCapability(); err != nil {
		return fmt.Errorf("manual_activation_required: automatic native Kiro MCP lifecycle containment preflight failed: %w", err)
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
	outcome := domain.DeactivationOutcome{Activation: domain.ActivationNotRequired, ArtifactRemovalAllowed: true}
	switch request.Client.ClientID {
	case domain.ClientKiro:
		if len(kiro.NativeObjects(request.NativeObjects)) == 0 {
			return shared.RequireExternalUninstall(outcome, request.ExternalUninstalled, "remove the legacy custom Power in Kiro, then rerun remove with `--external-uninstalled`"), nil
		}
		if !request.Confirmed {
			outcome.UserActions = append(outcome.UserActions, "agentplugins will remove its managed Kiro skills and MCP entries automatically")
			return outcome, nil
		}
		if err := kiro.DeactivateNative(ctx, request); err != nil {
			return outcome, err
		}
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	case domain.ClientCline:
		if len(cline.ClineObjects(request.NativeObjects)) == 0 {
			return outcome, fmt.Errorf("managed Cline native ownership is missing")
		}
		if !request.Confirmed {
			outcome.UserActions = append(outcome.UserActions, "agentplugins will remove only its managed Cline skills and MCP entries")
			return outcome, nil
		}
		if err := cline.DeactivateClineNativeWithKernel(ctx, request, activator.nativeConfigKernel()); err != nil {
			if !committedNativeDeactivationCleanup(&outcome, err) {
				return outcome, err
			}
		}
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	case domain.ClientOpenCode:
		if !request.Confirmed {
			outcome.UserActions = append(outcome.UserActions, "agentplugins will remove its managed OpenCode skills and MCP entries automatically")
			return outcome, nil
		}
		if err := opencode.DeactivateOpenCodeNativeWithKernel(ctx, request, activator.nativeConfigKernel()); err != nil {
			if !committedNativeDeactivationCleanup(&outcome, err) {
				return outcome, err
			}
		}
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	case domain.ClientGemini:
		if !request.Confirmed {
			outcome.UserActions = append(outcome.UserActions, "agentplugins will remove its managed Gemini CLI skills and MCP entries automatically")
			return outcome, nil
		}
		if err := gemini.DeactivateGeminiNativeWithKernel(ctx, request, activator.nativeConfigKernel()); err != nil {
			if !committedNativeDeactivationCleanup(&outcome, err) {
				return outcome, err
			}
		}
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	case domain.ClientWindsurf:
		if len(windsurf.WindsurfObjects(request.NativeObjects)) == 0 {
			return shared.RequireExternalUninstall(outcome, request.ExternalUninstalled, "remove the prepared package manually, then rerun remove with `--external-uninstalled`"), nil
		}
		if strings.TrimSpace(request.Client.ConfigRoot) == "" {
			return shared.RequireExternalUninstall(outcome, request.ExternalUninstalled, "select exactly one installed Windsurf channel, then rerun remove"), nil
		}
		if !request.Confirmed {
			outcome.UserActions = append(outcome.UserActions, "agentplugins will remove only its owned Windsurf MCP entries")
			return outcome, nil
		}
		if err := windsurf.DeactivateWindsurfNativeWithKernel(ctx, request, activator.nativeConfigKernel()); err != nil {
			if !committedNativeDeactivationCleanup(&outcome, err) {
				return outcome, err
			}
		}
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	default:
		return domain.DeactivationOutcome{}, fmt.Errorf("unsupported deactivation client %q", request.Client.ClientID)
	}
}

func (activator Activator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := activator.requireRegistry(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if request.Plan.ClientID != request.Client.ClientID || request.Delivery.ClientID != request.Client.ClientID {
		return domain.ActivationOutcome{}, fmt.Errorf("activation client identity mismatch")
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
	outcome := domain.ActivationOutcome{
		Authentication: request.Plan.Authentication,
		Policy:         domain.PolicyAllowed,
		Verification:   domain.VerificationPackageValid,
	}
	if request.Plan.InstallIntent == domain.InstallIntentPrepare {
		if lifecycle, ok := clients.As[clients.Lifecycle](activator.Registry, request.Client.ClientID); ok {
			return lifecycle.Activate(ctx, activator.env(), request)
		}
		if request.VerifyOnly {
			err = kiro.VerifyNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false)
		} else {
			err = kiro.ActivateNative(ctx, request)
		}
		if err != nil {
			return shared.FailedActivation(outcome, "repair the managed Kiro native configuration", err)
		}
		outcome.Activation = domain.ActivationPrepared
		outcome.UserActions = append(outcome.UserActions, request.Plan.UserActions...)
		return outcome, nil
	}
	if request.ActivationComplete && !activator.AutomaticallyActivates(request) {
		outcome.Activation = domain.ActivationActive
		outcome.Verification = domain.VerificationInstalled
		outcome.ActivationAttested = true
		return outcome, nil
	}
	if lifecycle, ok := clients.As[clients.Lifecycle](activator.Registry, request.Client.ClientID); ok {
		return lifecycle.Activate(ctx, activator.env(), request)
	}
	switch request.Client.ClientID {
	case domain.ClientKiro:
		if !activator.AutomaticallyActivates(request) {
			outcome.Activation = domain.ActivationManual
			outcome.UserActions = append(outcome.UserActions, "install a current Kiro CLI and rerun add to register the package's skills and MCP servers")
			outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("Kiro native installation requires a writable config root and, for MCP packages, a complete Kiro CLI distribution at %s", request.Delivery.ActivePath))
			return outcome, nil
		}
		if request.VerifyOnly {
			if err := kiro.VerifyNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false); err != nil {
				return shared.FailedActivation(outcome, "repair the managed Kiro skills and MCP configuration", err)
			}
			if shared.HasSupportedMCP(request.Plan.Components) {
				if err := activator.verifyKiroMCP(ctx, request); err != nil {
					if errors.Is(err, kiro.ErrACPContractUnknown) {
						return manualKiroVerification(outcome, request), nil
					}
					return shared.FailedActivation(outcome, fmt.Sprintf("rerun structured Kiro ACP verification with `%s acp --agent-engine v3 --auth-method cli`", request.BackendExecutable), err)
				}
			}
			outcome.Activation = domain.ActivationActive
			outcome.Verification = domain.VerificationInstalled
			return outcome, nil
		}
		if err := kiro.ActivateNative(ctx, request); err != nil {
			return shared.FailedActivation(outcome, "retry the managed Kiro native installation", err)
		}
		if shared.HasSupportedMCP(request.Plan.Components) {
			if err := activator.verifyKiroMCP(ctx, request); err != nil {
				if errors.Is(err, kiro.ErrACPContractUnknown) {
					return manualKiroVerification(outcome, request), nil
				}
				return shared.FailedActivation(outcome, fmt.Sprintf("retry structured Kiro ACP verification with `%s acp --agent-engine v3 --auth-method cli`", request.BackendExecutable), err)
			}
		}
		outcome.Activation = domain.ActivationActive
		outcome.Verification = domain.VerificationInstalled
		return outcome, nil
	case domain.ClientCline:
		return activator.activateNativeConfigClient(ctx, request, outcome, nativeConfigActivation{
			unavailableAction: "install Cline and rerun add to register its skills and MCP servers",
			repairAction:      "repair the managed Cline skills and MCP configuration",
			retryAction:       "retry the managed Cline native installation",
			completedAction:   "reload the Cline MCP view in VS Code, or start a new Cline CLI process",
			verify: func() error {
				return cline.VerifyClineNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false)
			},
			activate: cline.ActivateClineNativeWithKernel,
		})
	case domain.ClientOpenCode:
		return activator.activateNativeConfigClient(ctx, request, outcome, nativeConfigActivation{
			unavailableAction: "rerun with a detected OpenCode config root",
			repairAction:      "repair the managed OpenCode skills and MCP configuration",
			retryAction:       "retry the managed OpenCode native installation",
			completedAction:   "restart OpenCode to load the installed plugin",
			verify: func() error {
				return opencode.VerifyOpenCodeNativeObjects(request.Client.ConfigRoot, request.Delivery.ActivePath, request.Delivery.NativeObjects)
			},
			activate: opencode.ActivateOpenCodeNativeWithKernel,
		})
	case domain.ClientGemini:
		return activator.activateNativeConfigClient(ctx, request, outcome, nativeConfigActivation{
			unavailableAction: "install Gemini CLI and rerun add with an isolated writable Gemini config root",
			repairAction:      "repair the managed Gemini CLI skills and MCP configuration",
			retryAction:       "retry the managed Gemini CLI native installation",
			completedAction:   "in a running Gemini CLI session use `/mcp reload` and `/skills reload`, or restart Gemini CLI",
			verify: func() error {
				return gemini.VerifyGeminiNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false)
			},
			activate: gemini.ActivateGeminiNativeWithKernel,
		})
	case domain.ClientWindsurf:
		if !activator.AutomaticallyActivates(request) {
			outcome.Activation = domain.ActivationManual
			if shared.HasSupportedMCP(request.Plan.Components) {
				outcome.UserActions = append(outcome.UserActions, "select one legacy Windsurf channel and add the prepared MCP servers manually")
				outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("Windsurf: import MCP servers from %s; Devin cloud-synced configuration is never changed automatically", filepath.Join(request.Delivery.ActivePath, "mcp.json")))
			} else {
				outcome.UserActions = append(outcome.UserActions, "use the prepared skills manually in Windsurf; agentplugins does not claim automatic skill activation")
				outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("Windsurf: prepared skills remain at %s; Devin cloud-synced configuration is never changed automatically", filepath.Join(request.Delivery.ActivePath, "skills")))
			}
			return outcome, nil
		}
		if request.VerifyOnly {
			if err := windsurf.VerifyWindsurfNativeObjects(request.Client.ConfigRoot, request.Delivery.ActivePath, request.Delivery.NativeObjects, false); err != nil {
				return shared.FailedActivation(outcome, "repair the managed Windsurf MCP configuration", err)
			}
			outcome.Activation = domain.ActivationActive
			outcome.Verification = domain.VerificationInstalled
			return outcome, nil
		}
		if err := windsurf.ActivateWindsurfNativeWithKernel(ctx, request, activator.nativeConfigKernel()); err != nil {
			if !committedNativeCleanup(&outcome, err) {
				return shared.FailedActivation(outcome, "retry the managed Windsurf MCP installation", err)
			}
		}
		outcome.Activation = domain.ActivationActive
		outcome.Verification = domain.VerificationInstalled
		outcome.UserActions = append(outcome.UserActions, "refresh MCP servers in Windsurf before first use")
		return outcome, nil
	default:
		return domain.ActivationOutcome{}, fmt.Errorf("unsupported activation client %q", request.Client.ClientID)
	}
}

func (activator Activator) verifyKiroMCP(ctx context.Context, request domain.ActivationRequest) error {
	runner, ok := activator.Runner.(ports.DuplexCommandRunner)
	if !ok {
		return fmt.Errorf("%w: the process runner does not support an ACP duplex exchange", kiro.ErrACPContractUnknown)
	}
	var servers []string
	for _, component := range request.Plan.Components {
		if component.Kind == domain.ComponentMCPServer && component.Support != domain.SupportUnsupported {
			servers = append(servers, component.Name)
		}
	}
	return kiro.VerifyACP(ctx, runner, request.BackendExecutable, request.Delivery.ActivePath, servers)
}

// nativeConfigActivation is the shape every client that installs through the
// native config kernel shares. Cline, OpenCode and Gemini differed only in
// their messages and in which pair of functions they called, so the control
// flow - including the committed-cleanup degradation path - lives in one place.
type nativeConfigActivation struct {
	unavailableAction string
	repairAction      string
	retryAction       string
	completedAction   string
	verify            func() error
	activate          func(ctx context.Context, request domain.ActivationRequest, kernel nativeconfig.Kernel) error
}

func (activator Activator) activateNativeConfigClient(
	ctx context.Context,
	request domain.ActivationRequest,
	outcome domain.ActivationOutcome,
	client nativeConfigActivation,
) (domain.ActivationOutcome, error) {
	if !activator.AutomaticallyActivates(request) {
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = append(outcome.UserActions, client.unavailableAction)
		return outcome, nil
	}
	if request.VerifyOnly {
		if err := client.verify(); err != nil {
			return shared.FailedActivation(outcome, client.repairAction, err)
		}
	} else if err := client.activate(ctx, request, activator.nativeConfigKernel()); err != nil {
		if !committedNativeCleanup(&outcome, err) {
			return shared.FailedActivation(outcome, client.retryAction, err)
		}
	}
	outcome.Activation = domain.ActivationActive
	outcome.Verification = domain.VerificationInstalled
	outcome.UserActions = append(outcome.UserActions, client.completedAction)
	return outcome, nil
}

func isKiroCLI(executable string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(executable)))
	return base == "kiro-cli" || base == "kiro-cli.exe" || base == "kiro" || base == "kiro.exe"
}

func activationObservable(request domain.ActivationRequest, runner CommandRunner) bool {
	if runner == nil || strings.TrimSpace(request.BackendExecutable) == "" {
		return false
	}
	if request.Client.ClientID != domain.ClientKiro {
		return false
	}
	if !shared.OnlyNativeComponents(request.Plan.Components) || !isKiroCLI(request.BackendExecutable) {
		return false
	}
	if !shared.HasSupportedMCP(request.Plan.Components) {
		return true
	}
	_, duplexAvailable := runner.(ports.DuplexCommandRunner)
	return duplexAvailable
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
