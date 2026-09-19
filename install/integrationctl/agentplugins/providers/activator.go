package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// CommandRunner is an alias of the port. It stays exported here so callers and
// the optional runner capabilities in kiro_acp.go and native_identity.go keep
// their current names until the providers package is split into client adapters.
type CommandRunner = ports.CommandRunner

type Activator struct {
	Runner       CommandRunner
	NativeConfig *nativeconfig.Kernel
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
	switch request.Client.ClientID {
	case domain.ClientCline:
		return strings.TrimSpace(request.Client.ConfigRoot) != "" && shared.OnlyNativeComponents(request.Plan.Components)
	case domain.ClientOpenCode:
		return strings.TrimSpace(request.Client.ConfigRoot) != "" && shared.OnlyNativeComponents(request.Plan.Components)
	case domain.ClientGemini:
		return strings.TrimSpace(request.Client.ConfigRoot) != "" && shared.OnlyNativeComponents(request.Plan.Components)
	case domain.ClientWindsurf:
		return strings.TrimSpace(request.Client.ConfigRoot) != "" && len(windsurfObjects(request.Delivery.NativeObjects)) > 0
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
	if request.Plan.InstallIntent == domain.InstallIntentPrepare {
		if request.Client.ClientID == domain.ClientChatGPT {
			if request.Plan.Scope != domain.ScopeUser || !request.Plan.PersonalChatGPTPreparation {
				return fmt.Errorf("ChatGPT preparation requires validated personal mapping")
			}
			return nil
		}
		if request.Plan.Scope != domain.ScopeUser || strings.TrimSpace(request.Client.ConfigRoot) == "" || !shared.OnlyNativeComponents(request.Plan.Components) {
			return fmt.Errorf("Kiro preparation requires native configuration and supported components")
		}
		return nil
	}
	if request.Client.ClientID == domain.ClientClaude {
		_, err := prepareClaudeActivationProbe(request)
		return err
	}
	if !activator.AutomaticallyActivates(request) || request.Client.ClientID != domain.ClientKiro || !shared.HasSupportedMCP(request.Plan.Components) {
		return nil
	}
	runner, ok := activator.Runner.(duplexCapabilityRunner)
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
	outcome := domain.DeactivationOutcome{Activation: domain.ActivationNotRequired, ArtifactRemovalAllowed: true}
	switch request.Client.ClientID {
	case domain.ClientCursor:
		return outcome, nil
	case domain.ClientClaude:
		// The official @skills-dir flow is removed by the transaction kernel's
		// owned-directory removal. A confirmed mutation first verifies the exact
		// id and installPath through the trusted Claude CLI; previews stay inert.
		if !request.Confirmed {
			outcome.UserActions = append(outcome.UserActions, "agentplugins will verify and remove its managed Claude Code @skills-dir plugin")
			return outcome, nil
		}
		if strings.TrimSpace(request.BackendExecutable) == "" || activator.Runner == nil {
			return outcome, fmt.Errorf("trusted Claude Code CLI is required for exact removal verification")
		}
		listed, err := activator.runClaudeListResult(ctx, request.BackendExecutable, request.Client.ConfigRoot, request.ManagedArtifactPath)
		if err != nil {
			return outcome, fmt.Errorf("verify Claude Code plugin before removal: %w", err)
		}
		switch claudePluginStatus(listed.Stdout, request.DeclaredName, request.ManagedArtifactPath) {
		case claudeStatusInstalled:
		case claudeStatusAbsent:
			if request.CurrentActivation == domain.ActivationActive {
				return outcome, fmt.Errorf("%w: managed Claude Code plugin is absent before removal", shared.ErrRecognizedNegativeEvidence)
			}
		default:
			return outcome, fmt.Errorf("Claude Code plugin identity is not exact before removal")
		}
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	case domain.ClientCodex:
		hasCLI := strings.TrimSpace(request.BackendExecutable) != "" && activator.Runner != nil
		if !hasCLI && !request.ExternalUninstalled {
			return shared.RequireExternalUninstall(outcome, false, "uninstall the plugin in Codex, then rerun remove with `--external-uninstalled` (also use the flag if it was never activated)"), nil
		}
		if !request.Confirmed {
			if hasCLI {
				outcome.UserActions = append(outcome.UserActions, "agentplugins will uninstall the managed Codex plugin and its marketplace")
				return outcome, nil
			}
			return shared.RequireExternalUninstall(outcome, true, ""), nil
		}
		if strings.TrimSpace(request.PhysicalArtifactID) == "" {
			return outcome, fmt.Errorf("managed Codex marketplace identity is missing")
		}
		marketplace := shared.ManagedMarketplaceName(request.PhysicalArtifactID)
		registered, err := managedCodexMarketplaceRegistered(request.Client.ConfigRoot, marketplace, request.ManagedArtifactPath)
		if err != nil {
			return outcome, err
		}
		pluginEntryPresent, err := managedCodexPluginEntryPresent(request.Client.ConfigRoot, request.DeclaredName, marketplace)
		if err != nil {
			return outcome, err
		}
		if strings.TrimSpace(request.BackendExecutable) == "" || activator.Runner == nil {
			// Block on either stale record, not just the marketplace: a
			// live [plugins."id"] entry with no CLI available to clear it
			// is exactly as dangerous as a registered marketplace with no
			// CLI -- both would otherwise let ExternalRemovalComplete=true
			// fall through below with nothing actually cleaned.
			if registered || pluginEntryPresent {
				outcome.Activation = domain.ActivationManual
				outcome.ArtifactRemovalAllowed = false
				outcome.UserActions = []string{fmt.Sprintf("run `codex plugin remove %s@%s --json`, then `codex plugin marketplace remove %s --json`, then retry removal", request.DeclaredName, marketplace, marketplace)}
				return outcome, nil
			}
		} else {
			// Remove the plugin's own registration whenever a live CLI is
			// available, independent of whether the marketplace source is
			// still registered: a stale `[plugins."id"] enabled = true`
			// config.toml entry can outlive the marketplace record (for
			// example after a user follows this same code's own earlier
			// manual-cleanup guidance and only runs `marketplace remove`),
			// and a freshly started Codex app-server treats that lingering
			// entry as still-enabled, silently re-materializing the
			// "removed" plugin from its original local source. Gating this
			// call behind `registered` reproduced exactly that bug.
			// `codex plugin remove` is confirmed idempotent and safe to call
			// even when the marketplace is already gone (verified by hand
			// against a real Codex 0.153.4 binary: exit 0, clears the
			// `[plugins."id"]` entry, no error either way).
			if err := activator.removeCodexPlugin(ctx, request.BackendExecutable, request.DeclaredName+"@"+marketplace); err != nil {
				return outcome, err
			}
			if registered {
				if err := activator.removeCodexMarketplace(ctx, request.BackendExecutable, marketplace); err != nil {
					return outcome, err
				}
			}
		}
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	case domain.ClientChatGPT:
		return shared.RequireExternalUninstall(outcome, request.ExternalUninstalled, "uninstall the plugin in ChatGPT Plugins, then rerun remove with `--external-uninstalled` (also use the flag if it was never activated)"), nil
	case domain.ClientKiro:
		if len(kiroObjects(request.NativeObjects)) == 0 {
			return shared.RequireExternalUninstall(outcome, request.ExternalUninstalled, "remove the legacy custom Power in Kiro, then rerun remove with `--external-uninstalled`"), nil
		}
		if !request.Confirmed {
			outcome.UserActions = append(outcome.UserActions, "agentplugins will remove its managed Kiro skills and MCP entries automatically")
			return outcome, nil
		}
		if err := deactivateKiroNative(ctx, request); err != nil {
			return outcome, err
		}
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	case domain.ClientCline:
		if len(clineObjects(request.NativeObjects)) == 0 {
			return outcome, fmt.Errorf("managed Cline native ownership is missing")
		}
		if !request.Confirmed {
			outcome.UserActions = append(outcome.UserActions, "agentplugins will remove only its managed Cline skills and MCP entries")
			return outcome, nil
		}
		if err := deactivateClineNativeWithKernel(ctx, request, activator.nativeConfigKernel()); err != nil {
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
		if err := deactivateOpenCodeNativeWithKernel(ctx, request, activator.nativeConfigKernel()); err != nil {
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
		if err := deactivateGeminiNativeWithKernel(ctx, request, activator.nativeConfigKernel()); err != nil {
			if !committedNativeDeactivationCleanup(&outcome, err) {
				return outcome, err
			}
		}
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	case domain.ClientWindsurf:
		if len(windsurfObjects(request.NativeObjects)) == 0 {
			return shared.RequireExternalUninstall(outcome, request.ExternalUninstalled, "remove the prepared package manually, then rerun remove with `--external-uninstalled`"), nil
		}
		if strings.TrimSpace(request.Client.ConfigRoot) == "" {
			return shared.RequireExternalUninstall(outcome, request.ExternalUninstalled, "select exactly one installed Windsurf channel, then rerun remove"), nil
		}
		if !request.Confirmed {
			outcome.UserActions = append(outcome.UserActions, "agentplugins will remove only its owned Windsurf MCP entries")
			return outcome, nil
		}
		if err := deactivateWindsurfNativeWithKernel(ctx, request, activator.nativeConfigKernel()); err != nil {
			if !committedNativeDeactivationCleanup(&outcome, err) {
				return outcome, err
			}
		}
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	case domain.ClientCopilot, domain.ClientVSCode:
		hasCLI := strings.TrimSpace(request.BackendExecutable) != "" && activator.Runner != nil
		if !hasCLI && !request.ExternalUninstalled {
			action := fmt.Sprintf("run `copilot plugin uninstall %s`, then rerun remove with `--external-uninstalled`", request.DeclaredName)
			if request.Client.ClientID == domain.ClientVSCode {
				action = "remove the plugin in VS Code, then rerun remove with `--external-uninstalled`"
			}
			return shared.RequireExternalUninstall(outcome, false, action), nil
		}
		if !request.Confirmed {
			if hasCLI {
				outcome.UserActions = append(outcome.UserActions, "agentplugins will uninstall the plugin from GitHub Copilot CLI and VS Code automatically")
				return outcome, nil
			}
			return shared.RequireExternalUninstall(outcome, true, ""), nil
		}
		if hasCLI {
			if err := activator.deactivateCopilot(ctx, request); err != nil {
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
		if request.Client.ClientID == domain.ClientChatGPT {
			outcome.Activation = domain.ActivationPrepared
			outcome.Authentication = domain.AuthenticationNotRequired
			outcome.LocalActions = append(outcome.LocalActions, domain.ChatGPTPreparedAction(request.Delivery.ActivePath, request.DeclaredName))
			outcome.UserActions = append(outcome.UserActions, request.Plan.UserActions...)
			return outcome, nil
		}
		if request.VerifyOnly {
			err = verifyKiroNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false)
		} else {
			err = activateKiroNative(ctx, request)
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
	switch request.Client.ClientID {
	case domain.ClientCursor:
		if request.VerifyOnly {
			outcome.Activation = domain.ActivationManual
			outcome.UserActions = []string{"confirm that the plugin is visible and enabled in Cursor"}
			outcome.LocalActions = []string{fmt.Sprintf("open Cursor and verify the plugin from %s is visible and enabled", request.Delivery.ActivePath)}
			return outcome, nil
		}
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = append(outcome.UserActions, "register the prepared package in Cursor and verify it is visible before using its components")
		outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("open Cursor and register %s, then reload Cursor and verify %s is visible", request.Delivery.ActivePath, request.DeclaredName))
		return outcome, nil
	case domain.ClientCodex:
		if strings.TrimSpace(request.BackendExecutable) == "" || activator.Runner == nil {
			outcome.Activation = domain.ActivationManual
			outcome.UserActions = append(outcome.UserActions, "install the prepared plugin in Codex Plugins, then verify it appears in Plugins > Personal")
			outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("in Codex, install %s from %s, then verify it appears in Plugins > Personal", request.DeclaredName, request.Delivery.ActivePath))
			return outcome, nil
		}
		if request.VerifyOnly {
			if err := activator.verifyCodex(ctx, request); err != nil {
				if errors.Is(err, errCodexListContractUnknown) {
					return manualCodexVerification(outcome, request), nil
				}
				return shared.FailedActivation(outcome, fmt.Sprintf("verify with `%s plugin list --json`", request.BackendExecutable), err)
			}
			outcome.Activation = domain.ActivationActive
			outcome.Verification = domain.VerificationInstalled
			return outcome, nil
		}
		if err := activator.activateCodex(ctx, request); err != nil {
			if errors.Is(err, errCodexListContractUnknown) {
				return manualCodexVerification(outcome, request), nil
			}
			return shared.FailedActivation(outcome, fmt.Sprintf("run Codex activation again for the prepared package at %s, then verify with `%s plugin list --json`", request.Delivery.ActivePath, request.BackendExecutable), err)
		}
		outcome.Activation = domain.ActivationActive
		outcome.Verification = domain.VerificationInstalled
		return outcome, nil
	case domain.ClientClaude:
		if strings.TrimSpace(request.BackendExecutable) == "" || activator.Runner == nil {
			return shared.FailedActivation(outcome, "install Claude Code CLI and retry exact @skills-dir verification", fmt.Errorf("trusted Claude Code CLI is required"))
		}
		if err := activator.verifyClaude(ctx, request); err != nil {
			return shared.FailedActivation(outcome, fmt.Sprintf("verify with `%s plugin list --json`", request.BackendExecutable), err)
		}
		outcome.Activation = domain.ActivationActive
		outcome.Verification = domain.VerificationInstalled
		return outcome, nil
	case domain.ClientChatGPT:
		outcome.Activation = domain.ActivationManual
		if shared.ComponentKindPresent(request.Plan.Components, domain.ComponentApp) {
			outcome.UserActions = append(outcome.UserActions, "in ChatGPT Developer Mode, verify the registered connection referenced by .app.json, install the plugin from Plugins, then confirm it is enabled in a new chat")
			outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("open ChatGPT Plugins and install %s from the prepared marketplace at %s; verify every .app.json connection before confirming activation", request.DeclaredName, request.Delivery.ActivePath))
		} else {
			outcome.UserActions = append(outcome.UserActions, "install the prepared skills-only plugin from ChatGPT Plugins, then confirm it is enabled in a new chat")
			outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("open ChatGPT Plugins and install %s from the prepared marketplace at %s, then confirm it is enabled in a new chat", request.DeclaredName, request.Delivery.ActivePath))
		}
		return outcome, nil
	case domain.ClientKiro:
		if !activator.AutomaticallyActivates(request) {
			outcome.Activation = domain.ActivationManual
			outcome.UserActions = append(outcome.UserActions, "install a current Kiro CLI and rerun add to register the package's skills and MCP servers")
			outcome.LocalActions = append(outcome.LocalActions, kiroManualCLIAction(request))
			return outcome, nil
		}
		if request.VerifyOnly {
			if err := verifyKiroNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false); err != nil {
				return shared.FailedActivation(outcome, "repair the managed Kiro skills and MCP configuration", err)
			}
			if shared.HasSupportedMCP(request.Plan.Components) {
				if err := activator.verifyKiroMCP(ctx, request); err != nil {
					if errors.Is(err, errKiroACPContractUnknown) {
						return manualKiroVerification(outcome, request), nil
					}
					return shared.FailedActivation(outcome, fmt.Sprintf("rerun structured Kiro ACP verification with `%s acp --agent-engine v3 --auth-method cli`", request.BackendExecutable), err)
				}
			}
			outcome.Activation = domain.ActivationActive
			outcome.Verification = domain.VerificationInstalled
			return outcome, nil
		}
		if err := activateKiroNative(ctx, request); err != nil {
			return shared.FailedActivation(outcome, "retry the managed Kiro native installation", err)
		}
		if shared.HasSupportedMCP(request.Plan.Components) {
			if err := activator.verifyKiroMCP(ctx, request); err != nil {
				if errors.Is(err, errKiroACPContractUnknown) {
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
				return verifyClineNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false)
			},
			activate: activateClineNativeWithKernel,
		})
	case domain.ClientOpenCode:
		return activator.activateNativeConfigClient(ctx, request, outcome, nativeConfigActivation{
			unavailableAction: "rerun with a detected OpenCode config root",
			repairAction:      "repair the managed OpenCode skills and MCP configuration",
			retryAction:       "retry the managed OpenCode native installation",
			completedAction:   "restart OpenCode to load the installed plugin",
			verify: func() error {
				return verifyOpenCodeNativeObjects(request.Client.ConfigRoot, request.Delivery.ActivePath, request.Delivery.NativeObjects)
			},
			activate: activateOpenCodeNativeWithKernel,
		})
	case domain.ClientGemini:
		return activator.activateNativeConfigClient(ctx, request, outcome, nativeConfigActivation{
			unavailableAction: "install Gemini CLI and rerun add with an isolated writable Gemini config root",
			repairAction:      "repair the managed Gemini CLI skills and MCP configuration",
			retryAction:       "retry the managed Gemini CLI native installation",
			completedAction:   "in a running Gemini CLI session use `/mcp reload` and `/skills reload`, or restart Gemini CLI",
			verify: func() error {
				return verifyGeminiNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false)
			},
			activate: activateGeminiNativeWithKernel,
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
			if err := verifyWindsurfNativeObjects(request.Client.ConfigRoot, request.Delivery.ActivePath, request.Delivery.NativeObjects, false); err != nil {
				return shared.FailedActivation(outcome, "repair the managed Windsurf MCP configuration", err)
			}
			return windsurfActivated(outcome, request), nil
		}
		if err := activateWindsurfNativeWithKernel(ctx, request, activator.nativeConfigKernel()); err != nil {
			if !committedNativeCleanup(&outcome, err) {
				return shared.FailedActivation(outcome, "retry the managed Windsurf MCP installation", err)
			}
		}
		return windsurfActivated(outcome, request), nil
	case domain.ClientCopilot, domain.ClientVSCode:
		if strings.TrimSpace(request.BackendExecutable) == "" || activator.Runner == nil {
			outcome.Activation = domain.ActivationManual
			if request.Client.ClientID == domain.ClientVSCode {
				outcome.UserActions = append(outcome.UserActions, "register the prepared local plugin in VS Code")
				outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf(
					"VS Code: add %q to the `chat.pluginLocations` setting with value `true`, then reload VS Code and verify %s appears in the Plugins view",
					request.Delivery.ActivePath,
					request.DeclaredName,
				))
			} else {
				outcome.UserActions = append(outcome.UserActions, "install GitHub Copilot CLI, then rerun `agentplugins update` for this plugin")
				outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("install GitHub Copilot CLI, rerun add for the prepared package at %s, then verify with `copilot plugin list`", request.Delivery.ActivePath))
			}
			return outcome, nil
		}
		if request.VerifyOnly {
			if err := activator.verifyCopilot(ctx, request); err != nil {
				if errors.Is(err, errCopilotListContractUnknown) {
					return manualCopilotVerification(outcome, request), nil
				}
				return shared.FailedActivation(outcome, fmt.Sprintf("verify with `%s plugin list`", request.BackendExecutable), err)
			}
			outcome.Activation = domain.ActivationActive
			outcome.Verification = domain.VerificationInstalled
			return outcome, nil
		}
		if err := activator.activateCopilot(ctx, request); err != nil {
			if errors.Is(err, errCopilotListContractUnknown) {
				return manualCopilotVerification(outcome, request), nil
			}
			return shared.FailedActivation(outcome, fmt.Sprintf("rerun add for the prepared package at %s, then verify with `%s plugin list`", request.Delivery.ActivePath, request.BackendExecutable), err)
		}
		outcome.Activation = domain.ActivationActive
		outcome.Verification = domain.VerificationInstalled
		return outcome, nil
	default:
		return domain.ActivationOutcome{}, fmt.Errorf("unsupported activation client %q", request.Client.ClientID)
	}
}

func (activator Activator) activateCopilot(ctx context.Context, request domain.ActivationRequest) error {
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	updated := false
	if request.Replacing {
		if err := activator.runCopilot(ctx, request.BackendExecutable, "plugin", "marketplace", "update", marketplace); err != nil {
			if fallbackErr := activator.runCopilot(ctx, request.BackendExecutable, "plugin", "marketplace", "add", request.Delivery.ActivePath); fallbackErr != nil {
				return fmt.Errorf("refresh managed Copilot marketplace: %v; fallback registration: %w", err, fallbackErr)
			}
		}
		updated = activator.runCopilot(ctx, request.BackendExecutable, "plugin", "update", request.DeclaredName+"@"+marketplace) == nil
	} else if err := activator.runCopilot(ctx, request.BackendExecutable, "plugin", "marketplace", "add", request.Delivery.ActivePath); err != nil {
		if fallbackErr := activator.runCopilot(ctx, request.BackendExecutable, "plugin", "marketplace", "update", marketplace); fallbackErr != nil {
			return fmt.Errorf("register managed Copilot marketplace: %v; fallback refresh: %w", err, fallbackErr)
		}
	}
	pluginSpec := request.DeclaredName + "@" + marketplace
	if !updated {
		if err := activator.runCopilot(ctx, request.BackendExecutable, "plugin", "install", pluginSpec); err != nil {
			if !request.Replacing {
				_ = activator.runCopilot(ctx, request.BackendExecutable, "plugin", "marketplace", "remove", marketplace)
			}
			return err
		}
	}
	return activator.verifyCopilot(ctx, request)
}

func (activator Activator) verifyCopilot(ctx context.Context, request domain.ActivationRequest) error {
	pluginSpec := request.DeclaredName + "@" + shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	listed, err := activator.runCopilotResult(ctx, request.BackendExecutable, "plugin", "list")
	if err != nil {
		return fmt.Errorf("verify Copilot plugin listing: %w", err)
	}
	switch copilotPluginStatus(listed.Stdout, pluginSpec, copilotMarketplaceVersion(request.Plan.DeclaredVersion), request.Delivery.ActivePath) {
	case copilotStatusInstalled:
		return nil
	case copilotStatusAbsent:
		return fmt.Errorf("%w: verify Copilot plugin listing: %s is not listed", shared.ErrRecognizedNegativeEvidence, pluginSpec)
	default:
		return fmt.Errorf("%w: verify Copilot plugin listing", errCopilotListContractUnknown)
	}
}

func (activator Activator) activateCodex(ctx context.Context, request domain.ActivationRequest) error {
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	if request.Replacing {
		if _, err := activator.runClientResult(ctx, "Codex CLI", request.BackendExecutable, "plugin", "marketplace", "update", marketplace, "--json"); err != nil {
			if _, fallbackErr := activator.runClientResult(ctx, "Codex CLI", request.BackendExecutable, "plugin", "marketplace", "add", request.Delivery.ActivePath, "--json"); fallbackErr != nil {
				return fmt.Errorf("refresh Codex marketplace: %v; fallback registration: %w", err, fallbackErr)
			}
		}
	} else if _, err := activator.runClientResult(ctx, "Codex CLI", request.BackendExecutable, "plugin", "marketplace", "add", request.Delivery.ActivePath, "--json"); err != nil {
		return fmt.Errorf("register Codex marketplace: %w", err)
	}
	pluginSpec := request.DeclaredName + "@" + marketplace
	if _, err := activator.runClientResult(ctx, "Codex CLI", request.BackendExecutable, "plugin", "add", pluginSpec, "--json"); err != nil {
		if !request.Replacing {
			_, _ = activator.runClientResult(ctx, "Codex CLI", request.BackendExecutable, "plugin", "marketplace", "remove", marketplace, "--json")
		}
		return fmt.Errorf("activate Codex plugin: %w", err)
	}
	return activator.verifyCodex(ctx, request)
}

func (activator Activator) verifyCodex(ctx context.Context, request domain.ActivationRequest) error {
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	pluginSpec := request.DeclaredName + "@" + marketplace
	listed, err := activator.runClientResult(ctx, "Codex CLI", request.BackendExecutable, "plugin", "list", "--json")
	if err != nil {
		return fmt.Errorf("verify Codex plugin listing: %w", err)
	}
	switch codexPluginStatus(listed.Stdout, request.DeclaredName, marketplace) {
	case codexStatusInstalled:
		return nil
	case codexStatusAbsent:
		return fmt.Errorf("%w: verify Codex plugin listing: %s is not installed and enabled", shared.ErrRecognizedNegativeEvidence, pluginSpec)
	default:
		return fmt.Errorf("%w: verify Codex plugin listing", errCodexListContractUnknown)
	}
}

func (activator Activator) verifyClaude(ctx context.Context, request domain.ActivationRequest) error {
	probe, err := prepareClaudeActivationProbe(request)
	if err != nil {
		return fmt.Errorf("prepare Claude Code plugin listing: %w", err)
	}
	listed, err := activator.runPreparedClaudeListResult(ctx, probe.command)
	if err != nil {
		return fmt.Errorf("verify Claude Code plugin listing: %w", err)
	}
	switch claudePluginStatus(listed.Stdout, request.DeclaredName, probe.activePath) {
	case claudeStatusInstalled:
		return nil
	case claudeStatusAbsent, claudeStatusCollision:
		return fmt.Errorf("%w: verify Claude Code plugin listing: %s@skills-dir is not enabled at the managed path", shared.ErrRecognizedNegativeEvidence, request.DeclaredName)
	default:
		return fmt.Errorf("Claude Code plugin list output is not recognized")
	}
}

func (activator Activator) verifyKiroMCP(ctx context.Context, request domain.ActivationRequest) error {
	runner, ok := activator.Runner.(duplexCommandRunner)
	if !ok {
		return fmt.Errorf("%w: the process runner does not support an ACP duplex exchange", errKiroACPContractUnknown)
	}
	var servers []string
	for _, component := range request.Plan.Components {
		if component.Kind == domain.ComponentMCPServer && component.Support != domain.SupportUnsupported {
			servers = append(servers, component.Name)
		}
	}
	return verifyKiroACP(ctx, runner, request.BackendExecutable, request.Delivery.ActivePath, servers)
}

func (activator Activator) deactivateCopilot(ctx context.Context, request domain.DeactivationRequest) error {
	if strings.TrimSpace(request.PhysicalArtifactID) == "" {
		return fmt.Errorf("managed Copilot marketplace identity is missing")
	}
	marketplace := shared.ManagedMarketplaceName(request.PhysicalArtifactID)
	uninstall, err := activator.runCopilotResult(ctx, request.BackendExecutable, "plugin", "uninstall", request.DeclaredName+"@"+marketplace)
	if err != nil && !shared.CommandOutputContains(uninstall, "is not installed") {
		return err
	}
	remove, err := activator.runCopilotResult(ctx, request.BackendExecutable, "plugin", "marketplace", "remove", marketplace)
	if err != nil && !shared.CommandOutputContains(remove, "is not registered") {
		return err
	}
	return nil
}

func (activator Activator) removeCodexMarketplace(ctx context.Context, executable, marketplace string) error {
	remove, err := activator.runClientResult(ctx, "Codex CLI", executable, "plugin", "marketplace", "remove", marketplace, "--json")
	if err != nil && !shared.CommandOutputContains(remove, "not configured or installed") {
		return fmt.Errorf("remove managed Codex marketplace %s: %w", marketplace, err)
	}
	return nil
}

// removeCodexPlugin uninstalls the plugin itself (distinct from its
// marketplace source): it is what clears Codex's own per-plugin
// config.toml enablement entry and native cache, not just the marketplace
// registration. Already-absent is treated as success for idempotent retries.
func (activator Activator) removeCodexPlugin(ctx context.Context, executable, pluginSpec string) error {
	remove, err := activator.runClientResult(ctx, "Codex CLI", executable, "plugin", "remove", pluginSpec, "--json")
	if err != nil && !shared.CommandOutputContains(remove, "not configured or installed") {
		return fmt.Errorf("remove managed Codex plugin %s: %w", pluginSpec, err)
	}
	return nil
}

func (activator Activator) runCopilot(ctx context.Context, executable string, args ...string) error {
	_, err := activator.runCopilotResult(ctx, executable, args...)
	return err
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

func (activator Activator) runCopilotResult(ctx context.Context, executable string, args ...string) (legacyports.CommandResult, error) {
	return activator.runClientResult(ctx, "GitHub Copilot CLI", executable, args...)
}

func (activator Activator) runClientResult(ctx context.Context, client, executable string, args ...string) (legacyports.CommandResult, error) {
	return shared.RunClientCommand(ctx, activator.Runner, client, executable, args...)
}

func (activator Activator) runClaudeListResult(ctx context.Context, executable, configRoot, activePath string) (legacyports.CommandResult, error) {
	if activator.Runner == nil {
		return legacyports.CommandResult{}, fmt.Errorf("Claude Code CLI runner is unavailable")
	}
	command, err := claudeListCommand(executable, configRoot, activePath)
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	return activator.runPreparedClaudeListResult(ctx, command)
}

func (activator Activator) runPreparedClaudeListResult(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	result, err := runClaudeListCommand(ctx, activator.Runner, command)
	if err != nil {
		return result, fmt.Errorf("start Claude Code CLI: %w", err)
	}
	if result.ExitCode != 0 {
		return result, fmt.Errorf("Claude Code CLI command failed with exit code %d", result.ExitCode)
	}
	return result, nil
}

func isKiroCLI(executable string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(executable)))
	return base == "kiro-cli" || base == "kiro-cli.exe" || base == "kiro" || base == "kiro.exe"
}

type codexStatus int

const (
	codexStatusUnknown codexStatus = iota
	codexStatusInstalled
	codexStatusAbsent
)

var errCodexListContractUnknown = errors.New("Codex plugin list output is not recognized")

func codexPluginStatus(body []byte, name, marketplace string) codexStatus {
	if len(body) == 0 {
		return codexStatusUnknown
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	parsed, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return codexStatusUnknown
	}
	if _, tokenErr := decoder.Token(); tokenErr == nil || !errors.Is(tokenErr, io.EOF) {
		return codexStatusUnknown
	}
	document, ok := parsed.(map[string]any)
	if !ok {
		return codexStatusUnknown
	}
	installedValue, ok := document["installed"]
	if !ok {
		return codexStatusUnknown
	}
	entries, ok := installedValue.([]any)
	if !ok {
		return codexStatusUnknown
	}
	expectedID := name + "@" + marketplace
	identities := make(map[string]struct{}, len(entries))
	foundExpected := false
	expectedActive := false
	required := []string{"pluginId", "name", "marketplaceName", "installed", "enabled"}
	for _, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok {
			return codexStatusUnknown
		}
		for _, field := range required {
			if _, present := entry[field]; !present {
				return codexStatusUnknown
			}
		}
		pluginID, pluginIDOK := entry["pluginId"].(string)
		entryName, nameOK := entry["name"].(string)
		marketplaceName, marketplaceOK := entry["marketplaceName"].(string)
		installed, installedOK := entry["installed"].(bool)
		enabled, enabledOK := entry["enabled"].(bool)
		if !pluginIDOK || !nameOK || !marketplaceOK || !installedOK || !enabledOK ||
			pluginID == "" || entryName == "" || marketplaceName == "" ||
			pluginID != entryName+"@"+marketplaceName {
			return codexStatusUnknown
		}
		if _, duplicate := identities[pluginID]; duplicate {
			return codexStatusUnknown
		}
		identities[pluginID] = struct{}{}
		if pluginID == expectedID {
			foundExpected = true
			expectedActive = installed && enabled
		}
	}
	if foundExpected && expectedActive {
		return codexStatusInstalled
	}
	return codexStatusAbsent
}

type claudeStatus int

const (
	claudeStatusUnknown claudeStatus = iota
	claudeStatusInstalled
	claudeStatusAbsent
	claudeStatusCollision
)

// claudePluginStatus verifies the official in-place plugin slot: the unique
// list entry must be name@skills-dir at installPath == ActivePath with user
// scope. Claude 2.1.275 lets an enabled same-name marketplace install win:
// the skills-dir copy is listed as folder@skills-dir with an empty path, so
// that leftover is a collision rather than a free slot.
func claudePluginStatus(body []byte, name, activePath string) claudeStatus {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	parsed, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return claudeStatusUnknown
	}
	if _, tokenErr := decoder.Token(); !errors.Is(tokenErr, io.EOF) {
		return claudeStatusUnknown
	}
	entries, ok := parsed.([]any)
	if !ok {
		return claudeStatusUnknown
	}
	expectedID := name + "@skills-dir"
	seen := map[string]struct{}{}
	found := false
	foreignClaim := false
	for _, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok {
			return claudeStatusUnknown
		}
		id, idOK := entry["id"].(string)
		scope, scopeOK := entry["scope"].(string)
		enabled, enabledOK := entry["enabled"].(bool)
		installPath, pathOK := entry["installPath"].(string)
		if !idOK || !scopeOK || !enabledOK || !pathOK || id == "" || scope == "" {
			return claudeStatusUnknown
		}
		// Claude 2.1.275 lists a same-name skills-dir loser as folder@skills-dir
		// with enabled=false, empty installPath, and an errors array. Skip only
		// failed loads, not an enabled plugin that happens to carry warnings.
		if !enabled && (strings.TrimSpace(installPath) == "" || entry["errors"] != nil) {
			continue
		}
		if !filepath.IsAbs(installPath) {
			return claudeStatusUnknown
		}
		identity := id + "\x00" + scope + "\x00" + filepath.Clean(installPath)
		if _, duplicate := seen[identity]; duplicate {
			return claudeStatusUnknown
		}
		seen[identity] = struct{}{}
		if id == expectedID {
			if scope != "user" || !equivalentLocalPath(installPath, activePath) {
				return claudeStatusCollision
			}
			if found {
				return claudeStatusUnknown
			}
			found = enabled
			continue
		}
		pluginName, namespace, ok := splitClaudePluginID(id)
		if !ok {
			return claudeStatusUnknown
		}
		if enabled && pluginName == name && namespace != "skills-dir" {
			foreignClaim = true
		}
	}
	if found {
		return claudeStatusInstalled
	}
	if foreignClaim {
		return claudeStatusCollision
	}
	return claudeStatusAbsent
}

func splitClaudePluginID(id string) (name, namespace string, ok bool) {
	at := strings.LastIndex(id, "@")
	if at <= 0 || at == len(id)-1 {
		return "", "", false
	}
	return id[:at], id[at+1:], true
}

var copilotInstalledEntry = regexp.MustCompile(`^[ \t]+•[ \t]+([A-Za-z0-9][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9._-]*)[ \t]+\(v([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?)\)[ \t]*$`)
var copilotLiveEntry = regexp.MustCompile(`^  • ([A-Za-z0-9][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9._-]*) \(v([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?)\) \(([A-Za-z0-9_-]+)\)$`)

const copilotLiveHeader = "Live Plugins (loaded from a local marketplace directory, never copied):"

type copilotStatus int

const (
	copilotStatusUnknown copilotStatus = iota
	copilotStatusInstalled
	copilotStatusAbsent
)

var errCopilotListContractUnknown = errors.New("Copilot plugin list output is not recognized")

func copilotLivePluginStatus(stdout []byte, expected, expectedVersion, expectedPath string) (copilotStatus, bool) {
	document := strings.TrimSuffix(strings.ReplaceAll(string(stdout), "\r\n", "\n"), "\n")
	lines := strings.Split(document, "\n")
	if len(lines) == 0 || lines[0] != copilotLiveHeader {
		return copilotStatusUnknown, false
	}
	if len(lines) < 3 || (len(lines)-1)%2 != 0 || strings.TrimSpace(expectedVersion) == "" || strings.TrimSpace(expectedPath) == "" {
		return copilotStatusUnknown, true
	}
	seen := make(map[string]bool, (len(lines)-1)/2)
	matches := 0
	for index := 1; index < len(lines); index += 2 {
		entry := copilotLiveEntry.FindStringSubmatch(lines[index])
		if len(entry) != 4 || seen[entry[1]] || (entry[3] != "enabled" && entry[3] != "disabled") {
			return copilotStatusUnknown, true
		}
		seen[entry[1]] = true
		const pathPrefix = "      from "
		if !strings.HasPrefix(lines[index+1], pathPrefix) {
			return copilotStatusUnknown, true
		}
		listedPath := strings.TrimPrefix(lines[index+1], pathPrefix)
		if listedPath == "" || !filepath.IsAbs(listedPath) || listedPath != filepath.Clean(listedPath) {
			return copilotStatusUnknown, true
		}
		if entry[1] != expected {
			continue
		}
		if entry[2] != expectedVersion || entry[3] != "enabled" || expectedPath != filepath.Clean(expectedPath) || listedPath != expectedPath {
			return copilotStatusUnknown, true
		}
		matches++
	}
	if matches == 1 {
		return copilotStatusInstalled, true
	}
	if matches > 1 {
		return copilotStatusUnknown, true
	}
	return copilotStatusAbsent, true
}

func copilotPluginStatus(stdout []byte, expected, expectedVersion, expectedPath string) copilotStatus {
	if status, recognized := copilotLivePluginStatus(stdout, expected, expectedVersion, expectedPath); recognized {
		return status
	}
	inInstalledSection := false
	recognizedSection := false
	recognizedEntry := false
	matches := 0
	for _, rawLine := range strings.Split(strings.ReplaceAll(string(stdout), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(rawLine) == "Installed plugins:" {
			if inInstalledSection {
				return copilotStatusUnknown
			}
			inInstalledSection = true
			recognizedSection = true
			continue
		}
		if !inInstalledSection {
			continue
		}
		if rawLine != "" && rawLine[0] != ' ' && rawLine[0] != '\t' {
			inInstalledSection = false
			continue
		}
		entry := copilotInstalledEntry.FindStringSubmatch(rawLine)
		if len(entry) == 3 {
			recognizedEntry = true
			if entry[1] == expected {
				if entry[2] != expectedVersion {
					return copilotStatusUnknown
				}
				matches++
			}
			continue
		}
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}
		if trimmed == "No plugins installed." || trimmed == "No plugins installed" {
			recognizedEntry = true
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(trimmed, expected) {
			for _, state := range []string{"pending", "disconnected", "disabled", "auth-required", "auth required", "authentication required", "error", "failed", "failure"} {
				if strings.Contains(lower, state) {
					return copilotStatusAbsent
				}
			}
			return copilotStatusUnknown
		}
	}
	if matches == 1 {
		return copilotStatusInstalled
	}
	if matches > 1 {
		return copilotStatusUnknown
	}
	if recognizedSection && recognizedEntry {
		return copilotStatusAbsent
	}
	return copilotStatusUnknown
}

func activationObservable(request domain.ActivationRequest, runner CommandRunner) bool {
	if runner == nil || strings.TrimSpace(request.BackendExecutable) == "" {
		return false
	}
	switch request.Client.ClientID {
	case domain.ClientCodex, domain.ClientClaude, domain.ClientCopilot, domain.ClientVSCode:
		return true
	case domain.ClientKiro:
		if !shared.OnlyNativeComponents(request.Plan.Components) || !isKiroCLI(request.BackendExecutable) {
			return false
		}
		if !shared.HasSupportedMCP(request.Plan.Components) {
			return true
		}
		_, duplexAvailable := runner.(duplexCommandRunner)
		return duplexAvailable
	default:
		return false
	}
}

func kiroManualCLIAction(request domain.ActivationRequest) string {
	cli := strings.TrimSpace(request.BackendExecutable)
	if cli == "" {
		return fmt.Sprintf("Kiro native installation requires a writable config root and, for MCP packages, a trusted kiro-cli or kiro executable. Prepared package: %s", request.Delivery.ActivePath)
	}
	return fmt.Sprintf("Kiro native installation requires a writable config root and, for MCP packages, a trusted Kiro CLI at %s. Prepared package: %s", cli, request.Delivery.ActivePath)
}

func windsurfActivated(outcome domain.ActivationOutcome, request domain.ActivationRequest) domain.ActivationOutcome {
	outcome.Activation = domain.ActivationActive
	outcome.Verification = domain.VerificationInstalled
	outcome.UserActions = append(outcome.UserActions, "refresh MCP servers in Windsurf before first use")
	if shared.ComponentKindPresent(request.Plan.Components, domain.ComponentSkill) {
		outcome.UserActions = shared.AppendUnique(outcome.UserActions, "Windsurf skills remain in the prepared package and are not claimed as activated")
	}
	return outcome
}

func manualCodexVerification(outcome domain.ActivationOutcome, request domain.ActivationRequest) domain.ActivationOutcome {
	if attested, ok := shared.AttestedUnknownVerification(outcome, request); ok {
		return attested
	}
	outcome.Activation = domain.ActivationManual
	outcome.UserActions = []string{"confirm the managed plugin is installed and enabled in Codex"}
	outcome.LocalActions = []string{fmt.Sprintf("the `%s plugin list --json` output contract was not recognized; inspect %s manually", request.BackendExecutable, request.DeclaredName)}
	return outcome
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

func manualCopilotVerification(outcome domain.ActivationOutcome, request domain.ActivationRequest) domain.ActivationOutcome {
	if attested, ok := shared.AttestedUnknownVerification(outcome, request); ok {
		return attested
	}
	outcome.Activation = domain.ActivationManual
	outcome.UserActions = []string{"confirm the managed plugin is installed and enabled in GitHub Copilot"}
	outcome.LocalActions = []string{fmt.Sprintf("the `%s plugin list` output contract was not recognized; inspect %s manually", request.BackendExecutable, request.DeclaredName)}
	return outcome
}
