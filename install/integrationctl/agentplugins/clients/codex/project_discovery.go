package codex

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

const codexProjectDiscoveryTimeout = 60 * time.Second

func codexProjectSkillsOnly(plan domain.DeliveryPlan) bool {
	count := 0
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if component.Kind != domain.ComponentSkill {
			return false
		}
		count++
	}
	return count > 0
}

// Discovery reads the selected project and initializes only its selected local
// stdio MCP servers. It never starts a thread or requests a model response.
func completeCodexProjectDiscovery(ctx context.Context, env clients.Env, request domain.ActivationRequest, root string, conditions []string, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if outcome.Verification != domain.VerificationInstalled || !codexNativeDiscoverySupported(request) || len(conditions) != 0 {
		return outcome, nil
	}
	if err := observeCodexProjectComponents(ctx, env, request, root); err != nil {
		if domain.IsUnclosedClientProbe(err) {
			outcome.NativeEffect = domain.NativeEffectUncertain
			return outcome, err
		}
		if ctx.Err() != nil {
			return outcome, ctx.Err()
		}
		// Files are already durably installed. An unavailable or unrecognized
		// native observer must not turn them into a fabricated activation or a
		// failed transaction. Keep the existing prepared outcome and next step.
		outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("automatic Codex component discovery could not be confirmed: %v", err))
		return outcome, nil
	}
	if err := verifyCodexProjectObjects(root, request.Delivery.NativeObjects, false, env.NativeConfig); err != nil {
		return shared.FailedActivation(outcome, "repair the exact managed Codex project components", err)
	}
	outcome.Activation = domain.ActivationActive
	outcome.Authentication = codexNativeAuthentication(request.Plan)
	actions := outcome.UserActions[:0]
	for _, action := range outcome.UserActions {
		if action != codexProjectSetupAction(request.Plan) {
			actions = append(actions, action)
		}
	}
	outcome.UserActions = actions
	return outcome, nil
}

func observeCodexProjectComponents(ctx context.Context, env clients.Env, request domain.ActivationRequest, root string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	runner, ok := env.Runner.(ports.CompletionCommandRunner)
	if !ok || !shared.HasListingCLI(request.BackendExecutable) {
		return fmt.Errorf("the Codex CLI or interactive process runner is unavailable")
	}
	command, err := codexCommand(request.Client.ConfigRoot, request.BackendExecutable, os.Environ(), "app-server")
	if err != nil {
		return err
	}
	command.Dir = root
	bounded, cancel := context.WithTimeout(ctx, codexProjectDiscoveryTimeout)
	defer cancel()
	if err := runner.RunDuplexToCompletion(bounded, command, func(stdin io.Writer, stdout io.Reader) error {
		return exchangeCodexProjectComponents(stdin, stdout, request.Client.ConfigRoot, root, request.Plan)
	}); err != nil {
		return shared.CompletionProbeFailure(err)
	}
	return bounded.Err()
}

// Readiness can initialize only selected local stdio servers here. Remote
// transports retain their existing authentication and manual readiness boundary.
func codexNativeDiscoverySupported(request domain.ActivationRequest) bool {
	if !shared.OnlyNativeComponents(request.Plan.Components) {
		return false
	}
	for _, name := range shared.SupportedMCPNames(request.Plan) {
		server, err := readCodexProjectServer(request.Delivery.ActivePath, name)
		if err != nil || server.Type != "stdio" {
			return false
		}
	}
	return len(request.Plan.Components) > 0
}
