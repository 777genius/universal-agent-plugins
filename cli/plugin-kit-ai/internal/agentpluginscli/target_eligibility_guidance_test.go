package agentpluginscli

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

type deniedEligibilityActivator struct{ providers.Activator }

func (deniedEligibilityActivator) PreflightActivation(request domain.ActivationRequest) error {
	if request.Client.ClientID == domain.ClientKiro {
		return errors.New("\x1b[31m/private/profile/secret-token: unsafe verifier")
	}
	return nil
}

func TestSkippedReasonsSurviveZeroSingleAndMultipleChoices(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			clients := []domain.DetectedClient{fixtureClient(t, domain.ClientKiro), fixtureClient(t, domain.ClientChatGPT)}
			if count > 0 {
				clients = append(clients, fixtureClient(t, domain.ClientCursor))
			}
			if count > 1 {
				clients = append(clients, fixtureClient(t, domain.ClientCodex))
			}
			fixture := newCLIFixture(t, clients)
			fixture.app.Lifecycle.Activator = deniedEligibilityActivator{}
			plugin := writeCLIPlugin(t)
			writeCLIMCP(t, plugin)
			stdout, _, err := fixture.executeInput(true, "\n", "add", plugin, "--dry-run")
			if (err != nil) != (count == 0) {
				t.Fatalf("output=%q err=%v", stdout, err)
			}
			for _, want := range []string{"kiro: this CLI cannot automatically check MCP connections", "Nothing was installed in Kiro", "chatgpt: this package is not ready for ChatGPT", "registered connection mapping (.app.json)", "You do not need to create this file", "https://developers.openai.com/plugins/build/plugins"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("missing %q: %q", want, stdout)
				}
			}
			for _, bad := range []string{"cannot install together", "/private/", "secret-token", "\x1b[31m"} {
				if strings.Contains(stdout, bad) {
					t.Fatalf("unsafe/misleading output: %q", stdout)
				}
			}
			state, loadErr := fixture.store.Load()
			if loadErr != nil || len(state.Installations) != 0 {
				t.Fatalf("state=%+v err=%v", state, loadErr)
			}
			if entries, readErr := os.ReadDir(fixture.app.ManagedRoot); readErr == nil && len(entries) > 0 {
				t.Fatalf("staged files: %v", entries)
			}
		})
	}
}

func TestKiroEligibilityDoesNotGuessFromClientName(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientKiro)})
	fixture.app.Lifecycle.Activator = providers.Activator{Runner: &cliRunOnlyRunner{}}
	stdout, _, err := fixture.executeInput(true, "\n", "add", writeCLIPlugin(t), "--dry-run")
	if err != nil || strings.Contains(stdout, "Skipped") {
		t.Fatalf("skills-only Kiro was rejected: %q %v", stdout, err)
	}
}

func TestExplicitKiroPreflightDoesNotSuggestActivatingInstalledConfiguration(t *testing.T) {
	kiro := fixtureClient(t, domain.ClientKiro)
	kiro.ExecutablePath = "/test/bin/kiro-cli"
	fixture := newCLIFixture(t, []domain.DetectedClient{kiro})
	fixture.app.Lifecycle.Activator = providers.Activator{Runner: &cliRunOnlyRunner{}}
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "kiro", "--dry-run")
	if err == nil || !strings.Contains(stdout, "Nothing was installed") || strings.Contains(stdout, "Planned action:") {
		t.Fatalf("output=%q err=%v", stdout, err)
	}
}

func TestDirectoryZeroChoicesExplainReleaseSelection(t *testing.T) {
	rollout := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCursor}, []domain.ClientID{domain.ClientCursor})
	rollout.cli.app.Detector = staticDetector{clients: []domain.DetectedClient{fixtureClient(t, domain.ClientCodex)}}
	stdout, _, err := rollout.cli.executeInput(true, "\n", "add", "rollout-demo", "--dry-run")
	if err == nil || !strings.Contains(stdout, "codex: the catalog has no compatible release") || !strings.Contains(stdout, "--target codex") || strings.Contains(stdout, "verification preflight failed") {
		t.Fatalf("output=%q err=%v", stdout, err)
	}
	if rollout.acquirer.verifiedCalls != 0 {
		t.Fatal("acquired package with no eligible release")
	}
}

func TestPreflightNextActionsDoNotPromiseAutomaticInstallation(t *testing.T) {
	targets := []addTargetResult{{NextAction: "agentplugins will install and verify the package's global Kiro skills and MCP servers automatically"}}
	targets[0].Output.Result.Plan.Status = domain.PlanReady
	targets[0].Output.Result.Plan.Authentication = domain.AuthenticationNotRequired
	setPreflightNextActions(targets)
	if strings.Contains(addGroupNextAction(targets), "automatically") || !strings.Contains(targets[0].NextAction, "nothing was installed") {
		t.Fatalf("preflight retained the ready-plan promise: %+v", targets)
	}
	targets[0].Output.Result.Plan.Status = domain.PlanUnsupported
	targets[0].NextAction = "ask the publisher to register the ChatGPT connection"
	setPreflightNextActions(targets)
	if targets[0].NextAction != "ask the publisher to register the ChatGPT connection" {
		t.Fatal("unsupported-package recovery action was lost")
	}
}
