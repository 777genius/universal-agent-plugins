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
		{"observed", "succeeded", "installed and detected", SkillsBatchItem{Status: "completed", Client: completedSkillClient()}, true, false},
		{"verified repeat", "succeeded", "up to date and detected", SkillsBatchItem{Status: "unchanged", Client: completedSkillClient()}, true, false},
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
			if strings.Contains(*outcome.Items[0].Message, "private-secret") || len(outcome.Result.RepairInstructions) != 1 {
				t.Fatalf("provider output escaped or recovery was lost: %+v", outcome)
			}
		})
	}
}
