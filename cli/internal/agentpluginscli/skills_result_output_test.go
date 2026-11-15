package agentpluginscli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestSkillsHumanPartialResultPreservesFailedAndUnattemptedBindings(t *testing.T) {
	var output bytes.Buffer
	result := SkillsBatchResult{Status: "partial", Items: []SkillsBatchItem{
		{Alias: "canvas-design", ClientID: "claude", Status: "failed", Reason: "project list refused"},
		{Alias: "discernment-nudge", ClientID: "claude", Status: "not_attempted"},
		{Alias: "canvas-design", ClientID: "codex", Status: "not_attempted"},
	}, RepairInstructions: []string{"Inspect agentplugins deps status"}}
	if err := renderHumanSkillsResult(&output, result); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"Skills installation partial", "Claude Code", "canvas-design — failed", "discernment-nudge — not attempted", "Codex", "project list refused", "agentplugins deps status"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "— installed") || strings.Contains(text, "\x1b") {
		t.Fatalf("invented success or terminal escapes: %s", text)
	}
}

func TestSkillsHumanResultReportsManualTrustWithoutClaimingActive(t *testing.T) {
	var output bytes.Buffer
	result := SkillsBatchResult{Status: "succeeded", Items: []SkillsBatchItem{{Alias: "canvas-design", ClientID: "claude", Action: "install", Status: "completed",
		Client:        installer.ClientResult{Activation: "prepared", Verification: "installed", Authentication: "not_required"},
		ManualActions: []string{"Trust the project and reload Claude Code", "Trust the project and reload Claude Code"},
	}}}
	if err := renderHumanSkillsResult(&output, result); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "— setup required") || strings.Count(text, "Trust the project") != 1 || strings.Contains(text, "— installed") {
		t.Fatalf("manual readiness was lost: %s", text)
	}
	if !strings.Contains(text, "Skills installation needs attention") || strings.Contains(text, "Skills are ready to use") {
		t.Fatal("unverified installation claimed readiness:", text)
	}
}

func TestSkillsHumanResultUsesObservedNativeReadiness(t *testing.T) {
	var output bytes.Buffer
	result := SkillsBatchResult{Status: "succeeded", Items: []SkillsBatchItem{
		{Alias: "docx", ClientID: "claude", Action: "install", Status: "completed", Client: completedSkillClient()},
		{Alias: "frontend-design", ClientID: "codex", Action: "install", Status: "completed", Client: completedSkillClient()},
	}}
	if err := renderHumanSkillsResult(&output, result); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"Skills installation succeeded", "docx — installed", "frontend-design — installed", "Skills installed and detected in the selected agents."} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	for _, unwanted := range []string{"setup required", "not verified", "Next steps:", "sign-in"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("verified native result retained obsolete guidance %q: %s", unwanted, text)
		}
	}
}

func TestSkillsReadinessRequiresEvidenceForEverySelectedBinding(t *testing.T) {
	ready := SkillsBatchItem{Status: "completed", Client: completedSkillClient()}
	if state, next := deriveSkillsReadiness("succeeded", ready); state != "ready" || len(next) != 0 {
		t.Fatalf("observed installation is not ready: %s %+v", state, next)
	}
	for _, item := range []SkillsBatchItem{
		{Status: "unchanged"},
		{Status: "not_attempted"},
		{Status: "completed", Client: installer.ClientResult{Activation: "prepared", Verification: "installation_verified", Authentication: "not_required"}},
		{Status: "completed", Client: completedSkillClient(), ManualActions: []string{"Complete native setup"}},
	} {
		if state, _ := deriveSkillsReadiness("succeeded", ready, item); state == "ready" {
			t.Fatalf("one ready binding concealed missing evidence: %+v", item)
		}
	}
}
