package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
)

func TestSkillsReviewGroupsActionsWithoutHidingWarningsOrPaths(t *testing.T) {
	review := skillspicker.Plan{Warnings: []skillspicker.Diagnostic{{Message: "Trust this workspace before activation"}}}
	for _, name := range []string{"canvas-design", "discernment-nudge"} {
		for _, client := range []string{"claude", "codex", "copilot", "cursor"} {
			review.Actions = append(review.Actions, skillspicker.Action{Action: "install", Alias: name, TargetId: client, DestinationPath: "/private/project/" + client + "/" + name})
		}
	}
	request := skillsReviewConfirmation(review)
	summary := strings.Join(request.Summary, "\n")
	if !strings.Contains(summary, "2 Skills · 4 agents · 8 bindings") || strings.Count(summary, "install canvas-design") != 1 || strings.Contains(summary, "/private/project") {
		t.Fatalf("review did not group the real bindings: %s", summary)
	}
	if !strings.Contains(summary, "Trust this workspace") || len(request.Details) != 8 || !strings.Contains(request.Details[0], "/private/project/claude/canvas-design") {
		t.Fatalf("warnings or exact paths were hidden: %+v", request)
	}
	if !strings.Contains(summary, "Claude Code") || !strings.Contains(summary, "Cursor") {
		t.Fatalf("friendly agent names missing: %s", summary)
	}
}

func TestSkillsReviewKeepsMixedOperationsDistinctAndLongPathsExact(t *testing.T) {
	path := "/project/" + strings.Repeat("x", 600)
	review := skillspicker.Plan{Actions: []skillspicker.Action{
		{Action: "install", Alias: "same", TargetId: "claude", DestinationPath: path},
		{Action: "remove", Alias: "same", TargetId: "codex", DestinationPath: "/other"},
		{Action: "unchanged", Alias: "same", TargetId: "cursor", DestinationPath: "/existing"},
	}}
	r := skillsReviewConfirmation(review)
	summary := strings.Join(r.Summary, "\n")
	for _, kind := range []string{"install same", "remove same", "unchanged same"} {
		if !strings.Contains(summary, kind) {
			t.Fatal("operation lost from review:", summary)
		}
	}
	if !strings.Contains(r.Details[0], path) {
		t.Fatal("authority path was shortened")
	}
}

type skillsReviewCapture struct {
	calls  int
	output *bytes.Buffer
	t      *testing.T
}

func (*skillsReviewCapture) RichReview() bool { return true }
func (*skillsReviewCapture) SelectTargets(context.Context, prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
	return prompt.TargetSelectionResult{}, errors.New("unexpected selection")
}
func (p *skillsReviewCapture) Confirm(_ context.Context, r prompt.ConfirmationRequest) (prompt.ConfirmationResult, error) {
	p.calls++
	if p.calls == 1 {
		if len(r.Details) != 1 {
			p.t.Fatal("rich review did not supply details")
		}
		return prompt.ConfirmationResult{}, prompt.ErrReviewTooLarge
	}
	if len(r.Details) != 0 || len(r.Summary) != 0 || !strings.Contains(p.output.String(), "/exact/path") {
		p.t.Fatal("fallback did not show authority before bounded confirmation")
	}
	return prompt.ConfirmationResult{Accepted: false}, nil
}

func TestSkillsReviewOversizeFallbackPrintsFullPlanBeforeBoundedConfirmation(t *testing.T) {
	var output bytes.Buffer
	p := &skillsReviewCapture{output: &output, t: t}
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	plan := &PreparedSkillsBatch{actions: []SkillsAction{{Alias: "review", ClientID: "codex", Action: "install", TargetPath: "/exact/path"}}}
	err := runSkillsReviewedPlan(cmd, App{Prompter: p}, &options{format: "human"}, plan, true)
	if err != nil || p.calls != 2 || !strings.Contains(output.String(), "Installation canceled") {
		t.Fatalf("fallback: calls=%d err=%v output=%s", p.calls, err, output.String())
	}
}
