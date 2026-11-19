package copilot

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type copilotWholeSkillCatalog struct {
	plugins, skills ports.CommandResult
}

// Native registration retains its own receipt and delivery semantics. Only a
// pure Skill package can additionally complete through ordinary finite lists.
func observeCopilotWholePluginSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, out domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if env.Runner == nil || env.ParseSkill == nil || env.Snapshots == nil || env.NativeConfig.RequireBoundedRead() != nil ||
		!copilotWholeSkillsAuthority(request, out) || !copilotProjectSkillCLI(request.Client) {
		return out, nil
	}
	executable := request.Client.ExecutablePath
	if filepath.Clean(executable) != executable || strings.ContainsAny(executable, "\x00\r\n") ||
		(request.BackendExecutable != "" && request.BackendExecutable != executable) {
		return out, nil
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	proof := captureCopilotWholeSkills(bounded, env, request)
	if proof == nil || bounded.Err() != nil {
		return out, bounded.Err()
	}
	observed, err := runCopilotWholeSkills(bounded, env, request, proof)
	// Catalog cancellation must not skip the post-commit custody observation.
	verification, finish := context.WithTimeout(ctx, 5*time.Second)
	defer finish()
	return finishCopilotWholeSkills(verification, env, request, out, proof, observed, err)
}

func copilotWholeSkillsAuthority(request domain.ActivationRequest, out domain.ActivationOutcome) bool {
	if !copilotProjectSkillAuthority(request, out) || !copilotHasProjectPlugin(request.Delivery.NativeObjects) {
		return false
	}
	for _, component := range request.Plan.Components {
		if component.Support != domain.SupportNative || component.Source != "" || component.SourceName != "" {
			return false
		}
	}
	return true
}

func runCopilotWholeSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, proof *copilotWholeSkillProof) (copilotWholeSkillCatalog, error) {
	var result copilotWholeSkillCatalog
	target, executable := request.Plan.TargetContext, request.Client.ExecutablePath
	plugins, err := shared.RunNativeRegistry(ctx, env.Runner, copilotProjectCatalogCommand(executable, target, "plugin"))
	result.plugins = plugins
	if err = copilotWholeCatalogFailure(plugins, err); err != nil {
		return result, err
	}
	if !copilotWholePluginListed(plugins.Stdout, proof) {
		return result, nil
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	remaining := (1 << 20) - len(plugins.Stdout) - len(plugins.Stderr)
	if remaining <= 0 {
		return result, errors.New("copilot combined catalog capture budget exhausted")
	}
	command := copilotProjectCatalogCommand(executable, target, "skill")
	command.StdoutLimitBytes = remaining
	result.skills, err = shared.RunNativeRegistry(ctx, env.Runner, command)
	if len(result.skills.Stdout)+len(result.skills.Stderr) > remaining {
		return result, errors.Join(copilotWholeCatalogFailure(result.skills, err), errors.New("copilot combined catalog capture budget exceeded"))
	}
	return result, copilotWholeCatalogFailure(result.skills, err)
}

func copilotWholeCatalogFailure(result ports.CommandResult, err error) error {
	if !result.CleanupComplete && !result.NoChildStarted && err == nil {
		return &domain.UnclosedClientProbe{Operation: "Copilot plugin Skill catalog", Err: errors.New("runner did not confirm process cleanup")}
	}
	if err != nil {
		return err
	}
	if result.ExitCode != 0 || result.NoChildStarted || len(result.Stdout)+len(result.Stderr) > 1<<20 || len(result.Stderr) != 0 {
		return errors.New("copilot ordinary plugin Skill catalog refused or exceeded its capture contract")
	}
	return nil
}

func finishCopilotWholeSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, out domain.ActivationOutcome, proof *copilotWholeSkillProof, observed copilotWholeSkillCatalog, queryErr error) (domain.ActivationOutcome, error) {
	if domain.IsUnclosedClientProbe(queryErr) {
		out.NativeEffect = domain.NativeEffectUncertain
		return shared.FailedActivation(out, "confirm prior Copilot catalog process cleanup before retrying", queryErr)
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	fresh := captureCopilotWholeSkills(ctx, env, request)
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if fresh == nil || !reflect.DeepEqual(proof, fresh) {
		out.NativeEffect = domain.NativeEffectUncertain
		return shared.FailedActivation(out, "repair exact owned Copilot plugin Skills before retrying", errors.Join(queryErr, errors.New("copilot plugin source, settings or trust changed during observation")))
	}
	if queryErr == nil && copilotWholePluginListed(observed.plugins.Stdout, proof) &&
		copilotProjectSkillsListed(observed.skills.Stdout, proof.skills) && copilotWholeSkillsAuthority(request, out) {
		out.Activation = domain.ActivationActive
		if out.Authentication == domain.AuthenticationNotChecked {
			out.Authentication = proof.authentication
		}
		out.UserActions = slices.DeleteFunc(slices.Clone(out.UserActions), func(action string) bool { return action == copilotWholePluginInstalledAction })
	}
	return out, nil
}
