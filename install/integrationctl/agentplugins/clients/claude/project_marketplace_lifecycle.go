package claude

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func mutateClaudeProject(ctx context.Context, env clients.Env, command legacyports.Command, effect *domain.NativeEffectState, args ...string) error {
	command.Argv = append([]string{command.Argv[0]}, args...)
	command.StdoutLimitBytes = 1 << 20
	result, err := RunClaudeListCommand(ctx, env.Runner, command)
	if err == nil && result.ExitCode != 0 {
		diagnostic := strings.TrimSpace(string(result.Stderr))
		if diagnostic == "" {
			diagnostic = strings.TrimSpace(string(result.Stdout))
		}
		if len(diagnostic) > 2048 {
			diagnostic = diagnostic[:2048] + "..."
		}
		err = fmt.Errorf("the Claude Code %s failed (exit %d): %s", strings.Join(args[:2], " "), result.ExitCode, diagnostic)
	}
	if err != nil {
		if !result.NoChildStarted || *effect != domain.NativeEffectUnchanged {
			*effect = domain.NativeEffectUncertain
		}
		return err
	}
	*effect = domain.NativeEffectCommitted
	return nil
}

func activateClaudeProjectMarketplace(ctx context.Context, env clients.Env, request domain.ActivationRequest, command legacyports.Command) (domain.ActivationOutcome, error) {
	outcome := shared.StartedActivation(request)
	outcome.NativeEffect = domain.NativeEffectUnchanged
	outcome.NativeObjects = append([]domain.NativeObjectOwnership(nil), request.PreviousNativeObjects...)
	fail := func(err error) (domain.ActivationOutcome, error) {
		if domain.IsUnclosedClientProbe(err) {
			outcome.NativeEffect = domain.NativeEffectUncertain
		}
		return shared.FailedActivation(outcome, "retry the owned Claude Code project installation", err)
	}
	body, registered, observation, err := claudeProjectInstallPreflight(ctx, env, request, command)
	if err != nil {
		return fail(err)
	}
	if observation.state == projectListDisabled {
		outcome.Activation, outcome.Verification = domain.ActivationPrepared, domain.VerificationInstalled
		outcome.UserActions = []string{"the owned Claude Code project plugin is disabled; review its enablement in Claude Code"}
		return outcome, nil
	}
	if verifyClaudeProjectInstalled(ctx, request, body) == nil {
		outcome.Activation, outcome.Verification = domain.ActivationActive, domain.VerificationInstalled
		return outcome, nil
	}
	if request.VerifyOnly {
		return fail(fmt.Errorf("%w: exact Claude project plugin content is not enabled", shared.ErrRecognizedNegativeEvidence))
	}
	if err := installClaudeProjectMarketplace(ctx, env, request, command, registered, observation.state, &outcome.NativeEffect); err != nil {
		return fail(err)
	}
	listed, err := runPreparedClaudeList(ctx, env, command)
	if err == nil {
		err = verifyClaudeProjectInstalled(ctx, request, listed.Stdout)
	}
	if err != nil {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return fail(err)
	}
	outcome.NativeObjects = append([]domain.NativeObjectOwnership(nil), request.Delivery.NativeObjects...)
	outcome.Activation, outcome.Verification = domain.ActivationActive, domain.VerificationInstalled
	return outcome, nil
}

func claudeProjectInstallPreflight(ctx context.Context, env clients.Env, request domain.ActivationRequest, command legacyports.Command) ([]byte, bool, claudeMarketplaceObservation, error) {
	registered, err := claudeProjectMarketplaceRegistered(request.Client.ConfigRoot, request.Plan.ActivePath, request.Plan.PhysicalArtifactID)
	if err != nil {
		return nil, false, claudeMarketplaceObservation{}, err
	}
	listed, err := runPreparedClaudeList(ctx, env, command)
	if err != nil {
		return nil, false, claudeMarketplaceObservation{}, err
	}
	observation := inspectClaudeMarketplacePlugin(listed.Stdout, request.DeclaredName, request.Plan.PhysicalArtifactID, request.Plan.ActivePath, request.Plan.TargetContext.ProjectRoot, request.Client.ConfigRoot, "")
	if observation.state == projectListUnknown || observation.state == projectListForeign || observation.state != projectListAbsent && !registered {
		return nil, false, observation, fmt.Errorf("the Claude project plugin registration does not match the owned source")
	}
	if registered && !request.Replacing && !request.VerifyOnly {
		return nil, false, observation, fmt.Errorf("the Claude project marketplace was registered before this installation")
	}
	err = verifyClaudeProjectBindings(request.Client.ConfigRoot, request.DeclaredName, request.Plan.PhysicalArtifactID, request.Plan.TargetContext.ProjectRoot, observation.cache)
	if err == nil && observation.cache != "" {
		err = verifyClaudeProjectCache(ctx, request.Client.ConfigRoot, request.DeclaredName, request.Plan.PhysicalArtifactID, request.Plan.TargetContext.ProjectRoot, observation.cache, request.PreviousNativeObjects, false)
	}
	if err == nil {
		cache := filepath.Join(request.Client.ConfigRoot, "plugins", "cache", ProjectMarketplaceName(request.Plan.PhysicalArtifactID, request.Plan.TargetContext.ProjectRoot), request.DeclaredName, claudeProjectVersion(request.Plan.DeclaredVersion))
		err = verifyClaudeProjectCache(ctx, request.Client.ConfigRoot, request.DeclaredName, request.Plan.PhysicalArtifactID, request.Plan.TargetContext.ProjectRoot, cache, request.PreviousNativeObjects, true)
	}
	return listed.Stdout, registered, observation, err
}

func verifyClaudeProjectInstalled(ctx context.Context, request domain.ActivationRequest, body []byte) error {
	registered, err := claudeProjectMarketplaceRegistered(request.Client.ConfigRoot, request.Plan.ActivePath, request.Plan.PhysicalArtifactID)
	if err != nil {
		return err
	}
	observed := inspectClaudeMarketplacePlugin(body, request.DeclaredName, request.Plan.PhysicalArtifactID, request.Plan.ActivePath, request.Plan.TargetContext.ProjectRoot, request.Client.ConfigRoot, claudeProjectVersion(request.Plan.DeclaredVersion))
	if !registered || observed.state != projectListEnabled {
		return fmt.Errorf("%w: Claude did not enable the exact installed project plugin", shared.ErrRecognizedNegativeEvidence)
	}
	if err := verifyClaudeProjectBindings(request.Client.ConfigRoot, request.DeclaredName, request.Plan.PhysicalArtifactID, request.Plan.TargetContext.ProjectRoot, observed.cache); err != nil {
		return err
	}
	return verifyClaudeProjectCache(ctx, request.Client.ConfigRoot, request.DeclaredName, request.Plan.PhysicalArtifactID, request.Plan.TargetContext.ProjectRoot, observed.cache, request.Delivery.NativeObjects, false)
}

func installClaudeProjectMarketplace(ctx context.Context, env clients.Env, request domain.ActivationRequest, command legacyports.Command, registered bool, state projectListState, effect *domain.NativeEffectState) error {
	marketplace := ProjectMarketplaceName(request.Plan.PhysicalArtifactID, request.Plan.TargetContext.ProjectRoot)
	id := request.DeclaredName + "@" + marketplace
	if !registered {
		if err := mutateClaudeProject(ctx, env, command, effect, "plugin", "marketplace", "add", request.Plan.ActivePath, "--scope", "project"); err != nil {
			return err
		}
	}
	if state == projectListEnabled {
		// Native update deliberately retains a same-version cache. A selected
		// skill set can change without a manifest version bump, so reinstall the
		// exact previously verified binding while retaining all plugin data.
		if err := mutateClaudeProject(ctx, env, command, effect, "plugin", "uninstall", id, "--scope", "project", "--keep-data"); err != nil {
			return err
		}
	}
	return mutateClaudeProject(ctx, env, command, effect, "plugin", "install", id, "--scope", "project")
}

func deactivateClaudeProjectMarketplace(ctx context.Context, env clients.Env, request domain.DeactivationRequest, body []byte) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	outcome.NativeEffect = domain.NativeEffectUnchanged
	observed := inspectClaudeMarketplacePlugin(body, request.DeclaredName, request.PhysicalArtifactID, request.ManagedArtifactPath, request.TargetContext.ProjectRoot, request.Client.ConfigRoot, "")
	registered, err := claudeProjectMarketplaceRegistered(request.Client.ConfigRoot, request.ManagedArtifactPath, request.PhysicalArtifactID)
	if err != nil {
		return outcome, err
	}
	if observed.state == projectListUnknown || observed.state == projectListForeign || observed.state != projectListAbsent && !registered {
		return outcome, fmt.Errorf("the Claude project identity is not exact before removal")
	}
	if err := verifyClaudeProjectBindings(request.Client.ConfigRoot, request.DeclaredName, request.PhysicalArtifactID, request.TargetContext.ProjectRoot, observed.cache); err != nil {
		return outcome, err
	}
	if observed.cache != "" {
		if err := verifyClaudeProjectCache(ctx, request.Client.ConfigRoot, request.DeclaredName, request.PhysicalArtifactID, request.TargetContext.ProjectRoot, observed.cache, request.NativeObjects, false); err != nil {
			return outcome, err
		}
	}
	command, err := claudeProjectListCommand(request.BackendExecutable, request.Client, request.TargetContext, request.NativeRegistryRoot, request.ManagedArtifactPath)
	if err != nil {
		return outcome, err
	}
	err = uninstallClaudeProjectMarketplace(ctx, env, request, command, observed.state, registered, &outcome.NativeEffect)
	if err != nil {
		return outcome, err
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}

func uninstallClaudeProjectMarketplace(ctx context.Context, env clients.Env, request domain.DeactivationRequest, command legacyports.Command, state projectListState, registered bool, effect *domain.NativeEffectState) error {
	marketplace := ProjectMarketplaceName(request.PhysicalArtifactID, request.TargetContext.ProjectRoot)
	if state != projectListAbsent {
		if err := mutateClaudeProject(ctx, env, command, effect, "plugin", "uninstall", request.DeclaredName+"@"+marketplace, "--scope", "project", "--keep-data"); err != nil {
			return err
		}
		listed, err := runPreparedClaudeList(ctx, env, command)
		if err != nil {
			return err
		}
		if observeClaudeMarketplacePlugin(listed.Stdout, request.DeclaredName, request.PhysicalArtifactID, request.ManagedArtifactPath, request.TargetContext.ProjectRoot, request.Client.ConfigRoot, "") != projectListAbsent {
			return fmt.Errorf("the Claude project uninstall did not establish native absence")
		}
	}
	if !registered {
		return nil
	}
	if err := mutateClaudeProject(ctx, env, command, effect, "plugin", "marketplace", "remove", marketplace, "--scope", "project"); err != nil {
		return err
	}
	remaining, err := claudeProjectMarketplaceRegistered(request.Client.ConfigRoot, request.ManagedArtifactPath, request.PhysicalArtifactID)
	if err != nil {
		return err
	}
	if remaining {
		return fmt.Errorf("the Claude project marketplace removal could not be confirmed")
	}
	return nil
}
