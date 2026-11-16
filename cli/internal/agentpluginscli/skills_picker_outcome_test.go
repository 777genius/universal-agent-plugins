package agentpluginscli

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestSkillsPickerOutcomeRequiresObservedReadiness(t *testing.T) {
	for _, test := range []struct {
		name, status, message string
		item                  SkillsBatchItem
		ready, setup          bool
	}{
		{"observed", "succeeded", "Skill installed and verified.", SkillsBatchItem{Status: "completed", Client: completedSkillClient()}, true, false},
		{"verified repeat", "succeeded", "Skill is up to date; installation verified.", SkillsBatchItem{Status: "unchanged", Client: completedSkillClient()}, true, false},
		{"prepared", "succeeded", "setup still needs verification", SkillsBatchItem{Status: "completed", Client: installer.ClientResult{Activation: "prepared", Verification: "installation_verified", Authentication: "not_required"}}, false, true},
		{"authentication pending", "succeeded", "Complete authentication", SkillsBatchItem{Status: "completed", Client: installer.ClientResult{Activation: "active", Verification: "installation_verified", Authentication: string(domain.AuthenticationPending)}}, false, true},
		{"repeat without evidence", "succeeded", "setup could not be verified", SkillsBatchItem{Status: "unchanged"}, false, true},
		{"remaining manual action", "succeeded", "setup still needs verification", SkillsBatchItem{Status: "completed", Client: completedSkillClient(), ManualActions: []string{"Complete native setup"}}, false, true},
		{"partial failure", "partial", "Installation stopped", SkillsBatchItem{Status: "failed", Reason: "native-output credential=private-secret"}, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome := skillBatchOutcome(SkillsBatchResult{Status: test.status, Items: []SkillsBatchItem{test.item}, RepairInstructions: []string{"Retained recovery instruction"}})
			if (outcome.Result.Message == skillsReadyMessage) != test.ready {
				t.Fatalf("incorrect discovery claim: %+v", outcome)
			}
			setup := false
			for _, diagnostic := range outcome.Diagnostics {
				setup = setup || diagnostic.Code == "NATIVE_SETUP_REQUIRED"
			}
			if setup != test.setup || len(outcome.Items) != 1 || outcome.Items[0].Message == nil || !strings.Contains(*outcome.Items[0].Message, test.message) {
				t.Fatalf("lost readiness or item guidance: %+v", outcome)
			}
			if test.ready && (strings.Contains(*outcome.Items[0].Message, "not checked") || strings.Contains(outcome.Result.Message, "not verified")) {
				t.Fatalf("verified installation retained obsolete guidance: %+v", outcome)
			}
			if strings.Contains(*outcome.Items[0].Message, "detected by the agent") || strings.Contains(outcome.Result.Message, "detected in the selected agents") {
				t.Fatalf("unobserved agent discovery was claimed: %+v", outcome)
			}
			if strings.Contains(*outcome.Items[0].Message, "private-secret") || len(outcome.Result.RepairInstructions) != 1 || outcome.Result.RepairInstructions[0] != "Retained recovery instruction" {
				t.Fatalf("provider output escaped or recovery was lost: %+v", outcome)
			}
		})
	}
}

func TestSkillsPickerKeepsConcreteSetupActionsDuringPartialFailure(t *testing.T) {
	outcome := skillBatchOutcome(SkillsBatchResult{Status: "partial", Items: []SkillsBatchItem{
		{ClientID: "codex", Status: "completed", Client: completedSkillClient(), ManualActions: []string{" Inspect selected server ", "Inspect selected server", ""}},
		{ClientID: "claude", Status: "completed", Client: completedSkillClient(), ManualActions: []string{"Reload selected plugin"}},
		{ClientID: "codex", Status: "failed", Reason: "credential=private-secret"},
	}})
	if len(outcome.Diagnostics) != 2 {
		t.Fatalf("concrete setup actions were lost or duplicated: %+v", outcome.Diagnostics)
	}
	for index, want := range []string{"OpenAI Codex: Inspect selected server", "Claude Code: Reload selected plugin"} {
		diagnostic := outcome.Diagnostics[index]
		if diagnostic.Code != "NATIVE_SETUP_REQUIRED" || diagnostic.Message != want || strings.Contains(diagnostic.Message, "private-secret") {
			t.Fatalf("unexpected setup diagnostic: %+v", diagnostic)
		}
	}
}
