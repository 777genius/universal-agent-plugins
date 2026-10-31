package claude

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func activateClaudeProject(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	outcome := shared.StartedActivation(request)
	probe, err := PrepareClaudeActivationProbe(request)
	if err != nil {
		return shared.FailedActivation(outcome, "retry exact Claude Code project verification", err)
	}
	listed, err := runPreparedClaudeList(ctx, env, probe.Command)
	if err != nil {
		return shared.FailedActivation(outcome, "retry exact Claude Code project listing", err)
	}
	observation := observeClaudeProjectList(listed.Stdout, request.DeclaredName, probe.ActivePath, request.Plan.TargetContext.ProjectRoot, request.Client.ConfigRoot)
	switch observation.state {
	case projectListEnabled:
		outcome.Activation, outcome.Verification = domain.ActivationActive, domain.VerificationInstalled
		return outcome, nil
	case projectListSuppressed, projectListDisabled, projectListOtherSource:
		// Delivery ownership is established by the verified package transaction,
		// never by a sentinel or a same-name record from another source.
		if request.Delivery.ArtifactDigest == "" {
			return shared.FailedActivation(outcome, "verify the owned project package", fmt.Errorf("the Claude project delivery has no verified artifact digest"))
		}
		outcome.Activation, outcome.Verification = domain.ActivationPrepared, domain.VerificationInstalled
		outcome.UserActions = []string{observation.action()}
		if observation.state == projectListSuppressed && observation.otherSource {
			outcome.UserActions = append(outcome.UserActions, (projectListObservation{state: projectListOtherSource}).action())
		}
		return outcome, nil
	case projectListAbsent, projectListForeign:
		return shared.FailedActivation(outcome, "retry exact Claude Code project verification", fmt.Errorf("%w: the selected Claude Code project plugin is absent or claimed at another project path", shared.ErrRecognizedNegativeEvidence))
	default:
		return shared.FailedActivation(outcome, "inspect Claude Code project list output", fmt.Errorf("the Claude Code project plugin list output is not recognized"))
	}
}

func claudeProjectRegistryFinding(body []byte, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) clients.RegistryFinding {
	observation := observeClaudeProjectList(body, plan.DeclaredName, plan.ActivePath, plan.TargetContext.ProjectRoot, client.ConfigRoot)
	switch observation.state {
	case projectListEnabled:
		if managed == nil {
			return clients.RegistryCollision
		}
		return clients.RegistryExpected
	case projectListDisabled:
		if managed == nil {
			return clients.RegistryCollision
		}
		return clients.RegistryClear
	case projectListAbsent, projectListSuppressed, projectListOtherSource:
		// Clear applies only to this scoped native namespace. The prepared scan and
		// generic receipt/digest verification independently establish file ownership.
		// This is deliberately not Expected/native-discovery reconciliation.
		return clients.RegistryClear
	case projectListForeign:
		return clients.RegistryCollision
	default:
		return clients.RegistryIndeterminate
	}
}

func finishClaudeProjectDeactivation(outcome domain.DeactivationOutcome, body []byte, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	observation := observeClaudeProjectList(body, request.DeclaredName, request.ManagedArtifactPath, request.TargetContext.ProjectRoot, request.Client.ConfigRoot)
	switch observation.state {
	case projectListEnabled:
	case projectListAbsent, projectListSuppressed, projectListDisabled, projectListOtherSource:
		if request.CurrentActivation == domain.ActivationActive {
			return outcome, fmt.Errorf("%w: active Claude Code project identity is no longer confirmed; reobserve the owned installation before removal", shared.ErrRecognizedNegativeEvidence)
		}
	default:
		return outcome, fmt.Errorf("the Claude Code project plugin identity is not exact before removal")
	}
	// Skills-directory plugins have no external uninstall registration. The
	// ordinary removal kernel still verifies and deletes only the owned directory.
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
