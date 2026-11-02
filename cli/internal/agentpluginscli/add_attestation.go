package agentpluginscli

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func resumeInteractiveLifecycle(
	ctx context.Context,
	cmd *cobra.Command,
	service usecase.Service,
	input usecase.AddInput,
	envelope domain.PackageEnvelope,
	current usecase.AddResult,
) error {
	if current.Plan.InstallIntent == domain.InstallIntentPrepare ||
		(current.Activation.Activation == domain.ActivationActive &&
			current.Activation.Verification == domain.VerificationInstalled &&
			current.Activation.Authentication != domain.AuthenticationPending &&
			current.Activation.Authentication != domain.AuthenticationNotChecked) {
		return nil
	}
	var err error
	if current.Activation.Activation != domain.ActivationActive || current.Activation.Verification != domain.VerificationInstalled {
		current, err = attestActivationComplete(ctx, cmd, service, input, envelope, current)
		if err != nil || current.Activation.Activation != domain.ActivationActive || current.Activation.Verification != domain.VerificationInstalled {
			return err
		}
	}
	if current.Activation.Authentication == domain.AuthenticationPending || current.Activation.Authentication == domain.AuthenticationNotChecked {
		current, err = attestAuthenticationComplete(ctx, cmd, service, input, envelope, current)
		if err != nil || current.Activation.Authentication == domain.AuthenticationPending || current.Activation.Authentication == domain.AuthenticationNotChecked {
			return err
		}
	}
	return renderAddResult(cmd.OutOrStdout(), "human", envelope, current, false)
}

func attestActivationComplete(ctx context.Context, cmd *cobra.Command, service usecase.Service, input usecase.AddInput, envelope domain.PackageEnvelope, current usecase.AddResult) (usecase.AddResult, error) {
	complete, err := promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Have you completed activation and verified the plugin is enabled in the client? [y/N]")
	if err != nil || !complete {
		if err == nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Activation remains unconfirmed. Rerun add when it is complete.")
		}
		return current, err
	}
	return resumeAddPhase(ctx, cmd, service, input, envelope, current, true, false)
}

func attestAuthenticationComplete(ctx context.Context, cmd *cobra.Command, service usecase.Service, input usecase.AddInput, envelope domain.PackageEnvelope, current usecase.AddResult) (usecase.AddResult, error) {
	complete, err := promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Have you completed required authentication, or reviewed the package and confirmed none is required? [y/N]")
	if err != nil || !complete {
		if err == nil {
			if current.Activation.ActivationAttested {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Activation is user-attested. Authentication remains unconfirmed; rerun add after completing or reviewing it.")
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Authentication remains unconfirmed. Rerun add after completing or reviewing it.")
			}
		}
		return current, err
	}
	return resumeAddPhase(ctx, cmd, service, input, envelope, current, false, true)
}

func resumeAddPhase(ctx context.Context, cmd *cobra.Command, service usecase.Service, input usecase.AddInput, envelope domain.PackageEnvelope, current usecase.AddResult, activationComplete, authComplete bool) (usecase.AddResult, error) {
	resume := input
	retainPlannedClient(&resume, current.Plan)
	resume.Confirmed = true
	resume.InstallationID = current.InstallationID
	resume.ActivationComplete = activationComplete
	resume.AuthComplete = authComplete
	result, err := service.Add(ctx, resume)
	if err != nil {
		if renderErr := renderAddResultError(cmd.OutOrStdout(), "human", envelope, result, false, err); renderErr != nil {
			return result, renderErr
		}
	}
	return result, err
}
