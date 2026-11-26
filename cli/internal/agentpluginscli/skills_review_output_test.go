package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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

func TestSkillsReviewAcknowledgesOnlyAcceptedHumanConfirmationBeforeProgress(t *testing.T) {
	readFailure := errors.New("confirmation input failed")
	for _, test := range []struct {
		name, format                         string
		confirm, accepted, dryRun, wantApply bool
		confirmError                         error
	}{
		{"accepted", "human", true, true, false, true, nil},
		{"declined", "human", true, false, false, false, nil},
		{"canceled", "human", true, false, false, false, prompt.ErrPromptCanceled},
		{"read-failed", "human", true, false, false, false, readFailure},
		{"accepted-with-error", "human", true, true, false, false, readFailure},
		{"json", "json", true, true, false, true, nil},
		{"yes", "human", false, true, false, true, nil},
		{"dry-run", "human", true, true, true, false, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			const acknowledgment = "Skills installation confirmed."
			cmd, output, visible := retryTestCommand(t)
			calls := 0
			app := App{Terminal: true, Output: output, ErrorOutput: visible, reviewOutput: visible}
			app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				calls++
				if calls > 1 {
					// The failed Apply is now truthfully reportable and retryable.
					// Decline that separate retry question without re-authorizing it.
					return prompt.ConfirmationResult{Accepted: false}, nil
				}
				if strings.Contains(visible.String(), acknowledgment) {
					t.Fatal("confirmation was acknowledged before it was accepted")
				}
				return prompt.ConfirmationResult{Accepted: test.accepted}, test.confirmError
			}}
			// An invalid project context makes reaching Apply observable through
			// the existing stale-plan sentinel, without any mutation or source I/O.
			plan := &PreparedSkillsBatch{app: App{Scope: domain.ScopeProject, ProjectRoot: "relative"}, actions: []SkillsAction{{Alias: "review", ClientID: "codex", Action: "install", TargetPath: "/exact/path"}}}
			err := runSkillsReviewedPlan(cmd, app, &options{format: test.format, plain: true, dryRun: test.dryRun}, plan, test.confirm)
			if test.wantApply {
				if !errors.Is(err, ErrSkillsPlanStale) {
					t.Fatalf("input guard did not expose Apply boundary: %v", err)
				}
			} else if !errors.Is(err, test.confirmError) {
				t.Fatalf("confirmation result changed: %v", err)
			}
			wantCalls := 0
			if test.confirm && !test.dryRun {
				wantCalls++
			}
			if test.confirm && test.wantApply && test.format == "human" {
				wantCalls++
			}
			if calls != wantCalls {
				t.Fatalf("confirmation/retry calls = %d, wanted %d", calls, wantCalls)
			}
			text := visible.String()
			if strings.Contains(output.String(), acknowledgment) {
				t.Fatal("confirmation acknowledgment entered result stdout")
			}
			wantAcknowledgment := test.confirm && test.wantApply && test.format == "human"
			count := strings.Count(text, acknowledgment)
			if wantAcknowledgment && count != 1 || !wantAcknowledgment && count != 0 {
				t.Fatalf("unexpected confirmation acknowledgment: %s", text)
			}
			if wantAcknowledgment && strings.Index(text, "0/1 queued") < strings.Index(text, acknowledgment) {
				t.Fatalf("progress preceded confirmation acknowledgment: %s", text)
			}
		})
	}
}

func TestSkillsConfirmationWriteFailureStopsBeforeApplyAndProgress(t *testing.T) {
	writeFailure := errors.New("confirmation output failed")
	for _, test := range []struct {
		name   string
		writer io.Writer
		want   error
	}{
		{"error", retryCleanupWriter{cause: writeFailure, fail: func() bool { return true }}, writeFailure},
		{"short", shortPromptWriter{}, io.ErrShortWrite},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd, _, progress := retryTestCommand(t)
			accepted := false
			app := App{Terminal: true, ErrorOutput: progress, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				accepted = true
				cmd.SetOut(test.writer)
				return prompt.ConfirmationResult{Accepted: true}, nil
			}}}
			plan := &PreparedSkillsBatch{actions: []SkillsAction{{Alias: "review", ClientID: "codex", Action: "install", TargetPath: "/exact/path"}}}
			err := runSkillsReviewedPlan(cmd, app, &options{format: "human", plain: true}, plan, true)
			if !accepted || !errors.Is(err, test.want) || plan.applied || progress.Len() != 0 {
				t.Fatalf("failed acknowledgment reached Apply: accepted=%t applied=%t err=%v progress=%s", accepted, plan.applied, err, progress)
			}
		})
	}
}
