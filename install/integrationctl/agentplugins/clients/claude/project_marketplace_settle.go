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
	observeCtx := context.WithoutCancel(ctx)
	listed, err := runPreparedClaudeList(observeCtx, env, command)
	if err != nil {
		return
	}
	if verifyClaudeProjectInstalled(observeCtx, request, listed.Stdout) == nil {
		outcome.NativeEffect = domain.NativeEffectCommitted
		outcome.NativeObjects = append([]domain.NativeObjectOwnership(nil), request.Delivery.NativeObjects...)
		return
	}
	registered, err := claudeProjectMarketplaceRegistered(request.Client.ConfigRoot, request.Plan.ActivePath, request.Plan.PhysicalArtifactID)
	if err != nil {
		return
	}
	observation, err := inspectClaudeProjectInstall(observeCtx, request, listed.Stdout, registered)
	if err != nil {
		return
	}
	// Recorded ownership (the previous cache receipt) still matches the client.
	outcome.NativeEffect = domain.NativeEffectCommitted
	if registered == before.registered && observation.state == before.state {
		outcome.NativeEffect = domain.NativeEffectUnchanged
	}
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
