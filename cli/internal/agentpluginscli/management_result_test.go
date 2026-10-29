package agentpluginscli

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestManagementOutcomePreservesPublicManualInstructions(t *testing.T) {
	prepared := managementPrepared{
		backend: &ManagementBackend{config: management.Startup{Targets: []management.StartupTarget{{Id: "profile-opencode", Client: "opencode"}}}},
		request: dependencySyncRequest{prepared: []dependencyAction{{Alias: "demo", ClientID: "opencode", Action: environment.ActionInstall}}},
	}
	const restart = "restart OpenCode to load the installed plugin"
	const cleanup = "native config was committed, but lock cleanup degraded; retry repair if the next operation reports a busy lock"
	native := installer.Result{
		Outcome: installer.OutcomeCompleted, Mutated: true, ManualActions: []string{restart, cleanup},
		Reason: "private-provider-stderr", Delivery: &installer.DeliveryPlan{LocalActions: []string{"private-local-command"}},
	}
	out := prepared.outcome(dependencyApplicationResult{Results: []installer.Result{native}}, nil)
	want := []management.NextAction{
		{Kind: "manual", Reason: restart, Alias: "demo", TargetId: "profile-opencode"},
		{Kind: "manual", Reason: cleanup, Alias: "demo", TargetId: "profile-opencode"},
	}
	if out.State != management.Succeeded || out.Result.Outcome != "completed" || !out.Result.Mutated || !reflect.DeepEqual(out.Result.NextActions, want) {
		t.Fatal("public completion instructions changed success or lost their binding", out)
	}

	// Native errors, diagnostic reasons and operational commands remain private,
	// including when an operation returns a known partial result.
	native.NextActions = []installer.NextAction{{Kind: "activate", Reason: "private-native-next-reason"}}
	partial := prepared.outcome(dependencyApplicationResult{Results: []installer.Result{native}}, errors.New("private-provider-error"))
	for _, result := range []management.Outcome{out, partial} {
		body, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "private-") {
			t.Fatal("private native diagnostic or command reached management output", string(body))
		}
	}
	if partial.State != management.Partial {
		t.Fatal("public instructions hid the partial native result", partial)
	}
}

func TestManagementManualInstructionsBounded(t *testing.T) {
	fallback := managementNativeNextAction("manual", "demo", "target")
	for _, test := range []struct {
		name, instruction string
	}{
		{"empty", ""},
		{"blank", " \n\t"},
		{"nul", "run command\x00suffix"},
		{"escape", "\x1b[31mcommand"},
		{"invalid-utf8", string([]byte{0xff})},
		{"oversized-ascii", strings.Repeat("x", managementInstructionMaxLength+1)},
		{"oversized-unicode", strings.Repeat("界", managementInstructionMaxLength+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			actions := managementManualNextActions([]string{"first step", test.instruction, "last step"}, "demo", "target")
			want := []management.NextAction{
				{Kind: "manual", Reason: "first step", Alias: "demo", TargetId: "target"},
				fallback,
				{Kind: "manual", Reason: "last step", Alias: "demo", TargetId: "target"},
			}
			if !reflect.DeepEqual(actions, want) {
				t.Fatal("invalid instruction was truncated or lost adjacent public guidance", actions)
			}
		})
	}
	for _, instruction := range []string{strings.Repeat("界", managementInstructionMaxLength), "Restart the client, then run:\n\tclient inspect --profile=demo"} {
		actions := managementManualNextActions([]string{instruction}, "demo", "target")
		if len(actions) != 1 || actions[0].Reason != instruction {
			t.Fatal("valid public instruction was changed")
		}
	}
	instructions := make([]string, managementInstructionMaxCount+1)
	for i := range instructions {
		instructions[i] = "complete step"
	}
	if got := managementManualNextActions(instructions, "demo", "target"); !reflect.DeepEqual(got, []management.NextAction{fallback}) {
		t.Fatal("oversized instruction list was partially forwarded", got)
	}
	if got := managementManualNextActions(nil, "demo", "target"); len(got) != 0 {
		t.Fatal("invented a manual requirement", got)
	}
}
