package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Only the native boundary is inert. Website acquisition, the reviewed batch,
// SDK transaction, classified reports and the actual retry loop are exercised.
type skillsUnclosedCodex struct {
	*codex.Adapter
	failure error
	calls   int
}

func (adapter *skillsUnclosedCodex) Activate(context.Context, clients.Env, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	adapter.calls++
	return domain.ActivationOutcome{NativeEffect: domain.NativeEffectUncertain}, adapter.failure
}

func TestSkillsRetryStopsAfterLaterUnclosedNativeProbe(t *testing.T) {
	for _, wrapping := range []string{"bare", "wrapped", "joined"} {
		t.Run(wrapping, func(t *testing.T) {
			_, journey, source, _ := managementMutableWebsite(t)
			closure := errors.New("fixture native process cleanup is unconfirmed")
			probe := &domain.UnclosedClientProbe{Operation: "fixture native inventory", Err: closure}
			var cause error = probe
			if wrapping == "wrapped" {
				cause = fmt.Errorf("native observation failed: %w", cause)
			} else if wrapping == "joined" {
				cause = errors.Join(errors.New("fixture companion failure"), cause)
			}
			adapter := &skillsUnclosedCodex{Adapter: codex.New(), failure: cause}
			registry, err := clients.NewRegistry(claude.New(), adapter)
			if err != nil {
				t.Fatal(err)
			}
			helper, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			app := journey.cli.app
			root := app.DependenciesRoot
			app.ClientRegistry = registry
			app.Installer, err = installer.New(installer.Config{StateRoot: root, StateFile: journey.cli.store.Path, LockFile: filepath.Join(root, "mutation.lock"), OperationsDir: journey.cli.operations, PluginDataBase: filepath.Join(root, "plugin-data"), ManagedRoot: app.ManagedRoot, TempRoot: filepath.Join(root, "installer-tmp"), Registry: registry, Runner: journey.runner, HelperExecutable: helper})
			if err != nil {
				t.Fatal(err)
			}
			detected, err := app.Detector.Detect(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			nativeBefore := map[string]map[string]savedJourneyFile{}
			for _, client := range detected {
				sentinel := filepath.Join(client.ConfigRoot, "foreign", "keep.txt")
				if err := os.MkdirAll(filepath.Dir(sentinel), 0700); err != nil {
					t.Fatal(err)
				}
				dependencyWrite(t, sentinel, "preserve native profile")
				nativeBefore[client.ConfigRoot] = savedJourneyTree(t, client.ConfigRoot)
			}
			plan, err := app.PrepareSkills(t.Context(), SkillsBatchRequest{Selections: []SkillsSelection{{Website: source, Name: "pdf", Alias: "pdf"}}, TargetIDs: []string{"claude", "codex"}})
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			if actions := plan.Actions(); len(actions) != 2 || actions[0].ClientID != "claude" || actions[1].ClientID != "codex" {
				t.Fatalf("fixture must fail an ordinary target before the unclosed probe: %+v", actions)
			}
			unavailable := plan.request.detected[domain.ClientClaude]
			unavailable.Status = domain.DetectionNotDetected
			plan.request.detected[domain.ClientClaude] = unavailable
			prompts, reprepares := 0, 0
			app.SourceAcquirerForDryRun = func(string) SourceAcquirer {
				reprepares++
				return journey.cli.app.SourceAcquirer
			}
			app.Prompter = fakePrompter{
				confirmFn: func() (prompt.ConfirmationResult, error) {
					prompts++
					return prompt.ConfirmationResult{Accepted: true}, nil
				},
				selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
					prompts++
					return prompt.TargetSelectionResult{IDs: request.DefaultIDs}, nil
				},
			}
			cmd, out, errOut := retryTestCommand(t)
			app.Terminal, app.ErrorOutput = true, errOut
			got := applySkillsWithRecovery(cmd, app, &options{format: "human", scope: "user", plain: true}, plan, true)
			var retained *domain.UnclosedClientProbe
			if !errors.As(got, &retained) || retained != probe || !errors.Is(got, closure) || !errors.Is(got, installer.ErrTargetFactsUnavailable) {
				t.Fatalf("mixed first failure lost the later process-cleanup cause: %v", got)
			}
			if prompts != 0 || reprepares != 0 || adapter.calls != 1 || strings.Contains(errOut.String(), "Retrying ") {
				t.Fatalf("unclosed native process allowed another attempt: prompts=%d reprepares=%d native_calls=%d output=%q", prompts, reprepares, adapter.calls, errOut.String())
			}
			if !strings.Contains(out.String(), "Claude") || !strings.Contains(out.String(), "Codex") || !strings.Contains(out.String(), "Needs attention") {
				t.Fatalf("stopping retry lost the per-agent results: %s", out.String())
			}
			if strings.Contains(out.String(), "To retry only these agents") || strings.Contains(out.String(), "--target claude") {
				t.Fatalf("cleanup refusal still recommends another attempt: %s", out.String())
			}
			for root, before := range nativeBefore {
				if !reflect.DeepEqual(before, savedJourneyTree(t, root)) {
					t.Fatalf("stopped inert native attempt changed the profile at %s", root)
				}
			}
			for _, installation := range journey.state(t).Installations {
				for _, binding := range installation.Clients {
					if binding.ClientID == "claude" {
						t.Fatal("retry installed the earlier unavailable target after an unclosed probe")
					}
				}
			}
		})
	}
}
