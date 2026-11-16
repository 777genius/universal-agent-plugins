package copilot

import (
	"context"
	"errors"
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
	return copilotUserLiveStep{argv: argv, result: legacyports.CommandResult{ExitCode: 1, Stderr: []byte("failed"), CleanupComplete: true}}
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


// A client command whose process closure was not proved must never be
// followed by a marketplace fallback, another install, or rollback cleanup.
// Both typed process-closure errors and a raw runner cleanup failure are
// admissible evidence of an unclosed child.
func TestCopilotUserUnclosedCommandStopsNativeFollowups(t *testing.T) {
	for _, tc := range []struct {
		name      string
		replacing bool
		install   bool
	}{
		{name: "initial registration"},
		{name: "replacement refresh", replacing: true},
		{name: "initial plugin install", install: true},
		{name: "replacement plugin install", replacing: true, install: true},
	} {
		for _, typed := range []bool{false, true} {
			name := tc.name + "/raw"
			if typed {
				name = tc.name + "/typed"
			}
			t.Run(name, func(t *testing.T) {
				request := copilotUserActivationRequest(t, tc.replacing)
				exe, marketplace := request.BackendExecutable, copilotUserLiveExpectedMarketplace(request.Plan.PhysicalArtifactID)
				first := []string{exe, "plugin", "marketplace", "add", request.Delivery.ActivePath}
				if tc.replacing {
					first = []string{exe, "plugin", "marketplace", "update", marketplace}
				}
				steps := []copilotUserLiveStep{}
				failed := first
				if tc.install {
					steps = append(steps, copilotUserLiveStep{argv: first, result: legacyports.CommandResult{CleanupComplete: true}})
					failed = []string{exe, "plugin", "install", request.DeclaredName + "@" + marketplace}
				}
				cause := errors.New("native process closure is unproved")
				var failure error = cause
				if typed {
					failure = &legacyports.ProcessClosureError{Cause: cause}
				}
				steps = append(steps, copilotUserLiveStep{argv: failed, err: failure})
				runner := &copilotUserLiveRunner{t: t, steps: steps}
				outcome, err := New().Activate(context.Background(), clients.Env{Runner: runner}, request)
				if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, cause) || outcome.NativeEffect != domain.NativeEffectUncertain || outcome.Activation != domain.ActivationFailed || runner.calls != len(steps) {
					t.Fatalf("unclosed native command not held: outcome=%+v err=%v calls=%d expected=%d", outcome, err, runner.calls, len(steps))
				}
			})
		}
	}
}
