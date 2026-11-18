package claude

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// claudeProjectState is the exact native project state one lifecycle step
// observed before acting on it.
type claudeProjectState struct {
	state      projectListState
	registered bool
	cache      string
}

// settleClaudeProjectInstall resolves the effect of a failed or interrupted
// Claude install command. Its process tree was reaped, so Claude's own
// registry is stable and is read once more, detached from the caller's
// cancellation. Only an exact install or a state that a later installation
// accepts becomes certain; anything else keeps the uncertain attempt.
func settleClaudeProjectInstall(ctx context.Context, env clients.Env, request domain.ActivationRequest, command legacyports.Command, before claudeProjectState, outcome *domain.ActivationOutcome, cause error) {
	if outcome.NativeEffect != domain.NativeEffectUncertain || domain.IsUnclosedClientProbe(cause) {
		return
	}
	// The listing keeps the ordinary ClaudeProbeTimeout bound; only the
	// caller's cancellation is detached for this one read.
	exact, after, err := readSettledClaudeProjectInstall(context.WithoutCancel(ctx), env, request, command)
	if err != nil {
		return
	}
	outcome.NativeEffect = domain.NativeEffectCommitted
	if exact {
		outcome.NativeObjects = append([]domain.NativeObjectOwnership(nil), request.Delivery.NativeObjects...)
		return
	}
	// Recorded ownership (the previous cache receipt) still matches the client.
	if after == before {
		outcome.NativeEffect = domain.NativeEffectUnchanged
	}
}

// readSettledClaudeProjectInstall is the one read-only re-read of Claude's own
// registry once no command that could change it is running. It reports an
// exact install of the requested delivery, or else the observed state when a
// later installation accepts it: registration, installation index and every
// recorded cache verified. Any other state is an error.
func readSettledClaudeProjectInstall(ctx context.Context, env clients.Env, request domain.ActivationRequest, command legacyports.Command) (bool, claudeProjectState, error) {
	listed, err := runPreparedClaudeList(ctx, env, command)
	if err != nil {
		return false, claudeProjectState{}, err
	}
	if verifyClaudeProjectInstalled(ctx, request, listed.Stdout) == nil {
		return true, claudeProjectState{}, nil
	}
	registered, err := claudeProjectMarketplaceRegistered(request.Client.ConfigRoot, request.Plan.ActivePath, request.Plan.PhysicalArtifactID)
	if err != nil {
		return false, claudeProjectState{}, err
	}
	observation, err := inspectClaudeProjectInstall(ctx, request, listed.Stdout, registered)
	if err != nil {
		return false, claudeProjectState{}, err
	}
	return false, claudeProjectState{state: observation.state, registered: registered}, nil
}

// settleClaudeProjectRemoval is the removal counterpart. A state identical to
// the one removal started from is unchanged. Any other state the removal
// itself accepts is settled partial progress: every recorded native object is
// still exact, so the core may clear its attempt and a retry continues.
func settleClaudeProjectRemoval(ctx context.Context, env clients.Env, request domain.DeactivationRequest, command legacyports.Command, before claudeProjectState, outcome *domain.DeactivationOutcome, cause error) {
	if outcome.NativeEffect == domain.NativeEffectUnchanged || domain.IsUnclosedClientProbe(cause) {
		return
	}
	observeCtx := context.WithoutCancel(ctx)
	listed, err := runPreparedClaudeList(observeCtx, env, command)
	if err != nil {
		return
	}
	after, err := inspectClaudeProjectRemoval(observeCtx, request, listed.Stdout)
	if err != nil {
		return
	}
	if after == before {
		outcome.NativeEffect = domain.NativeEffectUnchanged
		return
	}
	outcome.NativeEffect = domain.NativeEffectCommitted
	outcome.SettledPartialRemoval = true
}
