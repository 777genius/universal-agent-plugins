package agentpluginscli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSkillsHumanResultSeparatesFilesDiscoveryAndActivation(t *testing.T) {
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	result := SkillsBatchResult{
		Status: "succeeded",
		Items: []SkillsBatchItem{
			{Alias: "review", ClientID: "codex", Status: "completed"},
			{Alias: "review", ClientID: "claude", Status: "unchanged"},
		},
	}
	if err := renderSkillsResult(cmd, &options{format: "human"}, result); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Skills installation succeeded",
		"review → codex: completed",
		"review → claude: unchanged",
		"Files: installed or already up to date.",
		"Agent discovery: not verified",
		"Activation: not verified",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("missing %q in %q", expected, output.String())
		}
	}
	if strings.Contains(output.String(), "Skill activated") {
		t.Fatal("file delivery cannot attest native activation")
	}
}

func TestSkillsHumanPartialResultPrintsRecoveryWithoutSuccessClaim(t *testing.T) {
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	result := SkillsBatchResult{
		Status:             "partial",
		Items:              []SkillsBatchItem{{Alias: "review", ClientID: "codex", Status: "failed"}},
		RepairInstructions: []string{"Inspect agentplugins deps status and follow recovery steps."},
	}
	if err := renderSkillsResult(cmd, &options{format: "human"}, result); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Skills installation partial", "review → codex: failed", "Next: Inspect agentplugins deps status"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("missing %q in %q", expected, output.String())
		}
	}
	if strings.Contains(output.String(), "Files: installed") || strings.Contains(output.String(), "Skills installation succeeded") {
		t.Fatal("partial installation claimed success")
	}
}
