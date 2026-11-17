package claude

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var _ clients.InterruptedAttemptConfirmer = (*Adapter)(nil)

// ConfirmsInterruptedAttempt covers the project marketplace lifecycle, whose
// native commands run without a persisted intent.
func (*Adapter) ConfirmsInterruptedAttempt(scope domain.InstallScope) bool {
	return scope == domain.ScopeProject
}

// ConfirmInterruptedAttempt settles a project attempt whose agentplugins
// process was killed while a Claude Code command ran. A command that survived
// it could still change Claude's registry, so any such live process refuses.
// Otherwise the registry is read once, exactly as after an interrupted command
// whose process tree was reaped: an exact install of the installed package is
// owned, a state in which every recorded object is still exact keeps that
// ownership, and anything else stays uncertain. The outcome never claims
// activation; the ordinary repair or removal still decides the binding.
func (*Adapter) ConfirmInterruptedAttempt(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	outcome := shared.StartedActivation(request)
	outcome.Activation, outcome.Verification = domain.ActivationFailed, domain.VerificationFailed
	outcome.NativeEffect = domain.NativeEffectUncertain
	outcome.NativeObjects = append([]domain.NativeObjectOwnership(nil), request.PreviousNativeObjects...)
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return outcome, err
	}
	probe, err := PrepareClaudeActivationProbe(request)
	if err != nil {
		return outcome, err
	}
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		return outcome, fmt.Errorf("the Claude Code CLI is required to read its plugin state")
	}
	if err := requireNoSurvivingClaudeCommand(ctx, env, request.BackendExecutable, request.Client.ConfigRoot); err != nil {
		return outcome, err
	}
	request, err = installedClaudeProjectDelivery(ctx, request)
	if err != nil {
		return outcome, err
	}
	exact, _, err := readSettledClaudeProjectInstall(ctx, env, request, probe.Command)
	if err != nil {
		return outcome, fmt.Errorf("the Claude Code plugin state does not match the recorded ownership: %w", err)
	}
	outcome.NativeEffect = domain.NativeEffectCommitted
	if exact {
		outcome.NativeObjects = request.Delivery.NativeObjects
	}
	return outcome, nil
}

// installedClaudeProjectDelivery completes the request with the cache receipt
// Claude derives from the installed marketplace bundle. The bundle manifest
// always carries the version that names the cache.
func installedClaudeProjectDelivery(ctx context.Context, request domain.ActivationRequest) (domain.ActivationRequest, error) {
	root := request.Plan.ActivePath
	marketplace, err := hasClaudeProjectMarketplace(root, request.DeclaredName, request.Plan.PhysicalArtifactID)
	if err != nil {
		return request, err
	}
	if !marketplace {
		return request, fmt.Errorf("the installed package is not a Claude Code project marketplace")
	}
	manifest, err := readClaudeJSONObject(root, filepath.Join(claudeProjectBundleDirectory, ".claude-plugin", "plugin.json"))
	if err != nil {
		return request, err
	}
	version, _ := manifest["version"].(string)
	if pathpolicy.ValidateLeafID(version) != nil {
		return request, fmt.Errorf("the installed Claude Code project bundle has no exact version")
	}
	request.Plan.DeclaredVersion = version
	request.Delivery.NativeObjects, err = claudeProjectCacheProjection(ctx, root, request.Plan)
	return request, err
}

// requireNoSurvivingClaudeCommand refuses while a process that may still
// change Claude Code's plugins for this profile is alive.
func requireNoSurvivingClaudeCommand(ctx context.Context, env clients.Env, executable, configRoot string) error {
	census, ok := env.Runner.(ports.SurvivingCommandCensus)
	if !ok {
		return fmt.Errorf("cannot verify that no Claude Code command from the interrupted run is still running")
	}
	processes, err := census.SurvivingCommands(ctx)
	if err != nil {
		return fmt.Errorf("cannot verify that no Claude Code command from the interrupted run is still running: %w", err)
	}
	if process, found := survivingClaudeCommand(processes, executable, configRoot); found {
		command := strings.Join(process.Argv, " ")
		if len(command) > 200 {
			command = command[:200] + "..."
		}
		return fmt.Errorf("a Claude Code process that can still change its plugins is running (pid %d: %s); wait for it to finish or stop it, then retry", process.PID, command)
	}
	return nil
}

// survivingClaudeCommand finds a Claude Code process running a `plugin`
// command, or one bound to this profile through CLAUDE_CONFIG_DIR, which every
// native command of this adapter carries even if it rewrote its arguments.
// A wrapper script counts as Claude Code through its own name.
func survivingClaudeCommand(processes []legacyports.LiveProcess, executable, configRoot string) (legacyports.LiveProcess, bool) {
	names := map[string]bool{"claude": true, claudeCommandName(executable): true}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		names[claudeCommandName(resolved)] = true
	}
	delete(names, "")
	profile := "CLAUDE_CONFIG_DIR=" + filepath.Clean(configRoot)
	for _, process := range processes {
		claude, plugin := names[claudeCommandName(process.Executable)], false
		for i, arg := range process.Argv {
			if names[claudeCommandName(arg)] {
				claude = true
				plugin = plugin || i+1 < len(process.Argv) && process.Argv[i+1] == "plugin"
			}
		}
		if claude && (plugin || slices.Contains(process.Env, profile)) {
			return process, true
		}
	}
	return legacyports.LiveProcess{}, false
}

func claudeCommandName(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return strings.TrimSuffix(strings.ToLower(filepath.Base(path)), ".exe")
}
