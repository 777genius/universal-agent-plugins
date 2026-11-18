package copilot

import (
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func copilotUserActivationRequest(t *testing.T, replacing bool) domain.ActivationRequest {
	t.Helper()
	request := copilotUserLiveRequest(t)
	request.VerifyOnly = false
	request.Replacing = replacing
	return request
}

func copilotUserActivationSteps(request domain.ActivationRequest, steps ...copilotUserLiveStep) []copilotUserLiveStep {
	live := strings.Replace(string(copilotUserLiveDocument(request)), copilot103BuiltinSection, copilot112BuiltinSection, 1)
	return append(steps, copilotUserLiveListStep(request, []byte(live), false))
}

func failedStep(argv ...string) copilotUserLiveStep {
	return copilotUserLiveStep{argv: argv, result: legacyports.CommandResult{ExitCode: 1, Stderr: []byte("failed")}}
}

// Regression: a retry after a failed first `plugin install` replaces the
// tracked installation. Copilot answers `plugin update` for a live directory
// plugin with exit 0 even when it is disabled, so the install must still run.
func TestCopilotUserReplacementAlwaysInstallsLivePlugin(t *testing.T) {
	request := copilotUserActivationRequest(t, true)
	exe, marketplace := request.BackendExecutable, copilotUserLiveExpectedMarketplace(request.Plan.PhysicalArtifactID)
	runner := &copilotUserLiveRunner{t: t, steps: copilotUserActivationSteps(request,
		failedStep(exe, "plugin", "marketplace", "update", marketplace),
		copilotUserLiveStep{argv: []string{exe, "plugin", "marketplace", "add", request.Plan.ActivePath}},
		copilotUserLiveStep{argv: []string{exe, "plugin", "install", request.DeclaredName + "@" + marketplace}},
	)}
	outcome, err := New().Activate(context.Background(), clients.Env{Runner: runner}, request)
	if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled || runner.calls != len(runner.steps) {
		t.Fatalf("outcome=%+v err=%v calls=%d", outcome, err, runner.calls)
	}
}

func TestCopilotUserFreshInstallFailureRollsBackMarketplace(t *testing.T) {
	request := copilotUserActivationRequest(t, false)
	exe, marketplace := request.BackendExecutable, copilotUserLiveExpectedMarketplace(request.Plan.PhysicalArtifactID)
	runner := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{
		{argv: []string{exe, "plugin", "marketplace", "add", request.Plan.ActivePath}},
		failedStep(exe, "plugin", "install", request.DeclaredName+"@"+marketplace),
		{argv: []string{exe, "plugin", "marketplace", "remove", marketplace}},
	}}
	outcome, err := New().Activate(context.Background(), clients.Env{Runner: runner}, request)
	if err == nil || outcome.Activation != domain.ActivationFailed || runner.calls != len(runner.steps) {
		t.Fatalf("outcome=%+v err=%v calls=%d", outcome, err, runner.calls)
	}
}

func TestCopilotUserReplacementInstallFailureKeepsMarketplace(t *testing.T) {
	request := copilotUserActivationRequest(t, true)
	exe, marketplace := request.BackendExecutable, copilotUserLiveExpectedMarketplace(request.Plan.PhysicalArtifactID)
	runner := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{
		{argv: []string{exe, "plugin", "marketplace", "update", marketplace}},
		failedStep(exe, "plugin", "install", request.DeclaredName+"@"+marketplace),
	}}
	outcome, err := New().Activate(context.Background(), clients.Env{Runner: runner}, request)
	if err == nil || outcome.Activation != domain.ActivationFailed || runner.calls != len(runner.steps) {
		t.Fatalf("outcome=%+v err=%v calls=%d", outcome, err, runner.calls)
	}
}
