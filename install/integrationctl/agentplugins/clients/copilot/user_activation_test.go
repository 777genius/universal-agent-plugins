package copilot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
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
				if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, cause) || outcome.NativeEffect != domain.NativeEffectUncertain || outcome.Activation == domain.ActivationActive || len(outcome.UserActions) != 0 || len(outcome.LocalActions) != 0 || runner.calls != len(steps) {
					t.Fatalf("unclosed native command not held: outcome=%+v err=%v calls=%d expected=%d", outcome, err, runner.calls, len(steps))
				}
			})
		}
	}
}

func TestSharedCopilotUnclosedProcessStopsFallbacksAndRollback(t *testing.T) {
	for _, tc := range []struct {
		name      string
		replacing bool
		phase     string
	}{
		{name: "fresh marketplace add", phase: "marketplace"},
		{name: "replacement marketplace update", replacing: true, phase: "marketplace"},
		{name: "replacement plugin update", replacing: true, phase: "plugin-update"},
		{name: "fresh plugin install", phase: "plugin-install"},
		{name: "read-only plugin list", phase: "verify"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := copilotUserActivationRequest(t, tc.replacing)
			exe, marketplace := request.BackendExecutable, copilotUserLiveExpectedMarketplace(request.Plan.PhysicalArtifactID)
			cause := errors.New("native child closure not proved")
			failure := copilotUserLiveStep{err: &legacyports.ProcessClosureError{Cause: cause}}
			steps := []copilotUserLiveStep{}
			switch tc.phase {
			case "marketplace":
				if tc.replacing {
					failure.argv = []string{exe, "plugin", "marketplace", "update", marketplace}
				} else {
					failure.argv = []string{exe, "plugin", "marketplace", "add", request.Delivery.ActivePath}
				}
			case "plugin-update":
				steps = append(steps, copilotUserLiveStep{argv: []string{exe, "plugin", "marketplace", "update", marketplace}})
				failure.argv = []string{exe, "plugin", "update", request.DeclaredName + "@" + marketplace}
			case "plugin-install":
				steps = append(steps, copilotUserLiveStep{argv: []string{exe, "plugin", "marketplace", "add", request.Delivery.ActivePath}})
				failure.argv = []string{exe, "plugin", "install", request.DeclaredName + "@" + marketplace}
			case "verify":
				request.VerifyOnly = true
				failure.argv = []string{exe, "plugin", "list"}
			}
			steps = append(steps, failure)
			runner := &copilotUserLiveRunner{t: t, steps: steps}
			outcome, err := shared.CompleteCopilotActivationWithListingRecognition(context.Background(), clients.Env{Runner: runner}, request, shared.StartedActivation(request), recognizeCopilotUserActivation)
			if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, cause) || outcome.NativeEffect != domain.NativeEffectUncertain || outcome.Activation == domain.ActivationActive || len(outcome.UserActions) != 0 || len(outcome.LocalActions) != 0 || runner.calls != len(steps) {
				t.Fatalf("unsafe shared Copilot continuation: outcome=%+v err=%v calls=%d expected=%d", outcome, err, runner.calls, len(steps))
			}
		})
	}
}

func TestSharedCopilotClosedMarketplaceErrorStillUsesFallback(t *testing.T) {
	request := copilotUserActivationRequest(t, false)
	exe, marketplace := request.BackendExecutable, copilotUserLiveExpectedMarketplace(request.Plan.PhysicalArtifactID)
	runner := &copilotUserLiveRunner{t: t, steps: copilotUserActivationSteps(request,
		failedStep(exe, "plugin", "marketplace", "add", request.Delivery.ActivePath),
		copilotUserLiveStep{argv: []string{exe, "plugin", "marketplace", "update", marketplace}},
		copilotUserLiveStep{argv: []string{exe, "plugin", "install", request.DeclaredName + "@" + marketplace}},
	)}
	outcome, err := shared.CompleteCopilotActivationWithListingRecognition(context.Background(), clients.Env{Runner: runner}, request, shared.StartedActivation(request), recognizeCopilotUserActivation)
	if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled || runner.calls != len(runner.steps) {
		t.Fatalf("closed fallback regressed: outcome=%+v err=%v calls=%d", outcome, err, runner.calls)
	}
}

func TestSharedCopilotRemovalUnclosedProcessDoesNotContinue(t *testing.T) {
	request := copilotUserActivationRequest(t, false)
	exe, marketplace := request.BackendExecutable, copilotUserLiveExpectedMarketplace(request.Plan.PhysicalArtifactID)
	for _, phase := range []string{"uninstall", "marketplace remove"} {
		t.Run(phase, func(t *testing.T) {
			cause := errors.New("native removal child still running")
			steps := []copilotUserLiveStep{}
			failure := copilotUserLiveStep{err: &legacyports.ProcessClosureError{Cause: cause}}
			if phase == "uninstall" {
				failure.argv = []string{exe, "plugin", "uninstall", request.DeclaredName + "@" + marketplace}
				failure.result.Stderr = []byte("is not installed")
			} else {
				steps = append(steps, copilotUserLiveStep{argv: []string{exe, "plugin", "uninstall", request.DeclaredName + "@" + marketplace}})
				failure.argv = []string{exe, "plugin", "marketplace", "remove", marketplace}
				failure.result.Stderr = []byte("is not registered")
			}
			steps = append(steps, failure)
			runner := &copilotUserLiveRunner{t: t, steps: steps}
			remove := domain.DeactivationRequest{
				CurrentActivation:  domain.ActivationActive,
				Confirmed:          true,
				PhysicalArtifactID: request.Plan.PhysicalArtifactID,
				DeclaredName:       request.DeclaredName,
				BackendExecutable:  exe,
			}
			outcome, err := shared.CompleteCopilotDeactivation(context.Background(), clients.Env{Runner: runner}, remove, "manual removal")
			if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, cause) || outcome.NativeEffect != domain.NativeEffectUncertain || outcome.ExternalRemovalComplete || outcome.ArtifactRemovalAllowed || runner.calls != len(steps) {
				t.Fatalf("unsafe shared Copilot removal: outcome=%+v err=%v calls=%d expected=%d", outcome, err, runner.calls, len(steps))
			}
		})
	}
}
