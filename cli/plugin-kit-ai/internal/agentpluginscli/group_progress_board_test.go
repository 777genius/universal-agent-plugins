package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestStartGroupProgressBoardStaysOffOutsideLiveTTY(t *testing.T) {
	t.Parallel()
	selected := []domain.DetectedClient{
		{ClientID: domain.ClientClaude, DisplayName: "Claude Code"},
		{ClientID: domain.ClientCodex, DisplayName: "OpenAI Codex"},
	}
	if board := startGroupProgressBoard(App{Terminal: true, ErrorOutput: &bytes.Buffer{}}, "human", selected); board != nil {
		t.Fatal("buffer stderr must not start a live board")
	}
	if board := startGroupProgressBoard(App{Terminal: true}, "json", selected); board != nil {
		t.Fatal("json output must not start a live board")
	}
	if board := startGroupProgressBoard(App{Terminal: true, ErrorOutput: &bytes.Buffer{}}, "human", selected[:1]); board != nil {
		t.Fatal("a single target must not start a live board")
	}
}

func TestGroupProgressBoardRewritesEveryClient(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	selected := []domain.DetectedClient{
		{ClientID: domain.ClientClaude, DisplayName: "Claude Code"},
		{ClientID: domain.ClientCodex, DisplayName: "OpenAI Codex"},
		{ClientID: domain.ClientCursor, DisplayName: "Cursor"},
	}
	board := newGroupProgressBoard(&stderr, selected, true)
	if got := stderr.String(); !strings.Contains(got, "Claude Code") || !strings.Contains(got, "copying") || !strings.Contains(got, "copied") || !strings.Contains(got, "…") || !strings.Contains(got, " → ") {
		t.Fatalf("initial board = %q", got)
	}
	board.set(domain.ClientClaude, groupProgressActivating)
	board.set(domain.ClientCodex, groupProgressCopied)
	got := stderr.String()
	if !strings.Contains(got, "installing") || !strings.Contains(got, "copied") || !strings.Contains(got, "\033[") {
		t.Fatalf("live board did not rewrite in place: %q", got)
	}
	board.finish()
	if board.painted != 0 {
		t.Fatal("finish left the board painted")
	}
}

func TestGroupProgressBoardDecoratesStageAndActivate(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	board := newGroupProgressBoard(&stderr, []domain.DetectedClient{
		{ClientID: domain.ClientCopilot, DisplayName: "GitHub Copilot"},
		{ClientID: domain.ClientVSCode, DisplayName: "VS Code"},
	}, true)
	stager := &recordingProgressStager{}
	activator := &recordingProgressActivator{outcome: domain.ActivationOutcome{Activation: domain.ActivationActive}}
	service := board.decorate(usecase.Service{Stager: stager, Activator: activator})
	if _, err := service.Stager.Stage(context.Background(), domain.PackageEnvelope{}, domain.DeliveryPlan{ClientID: domain.ClientCopilot}, "op", domain.CompatibilityHints{}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Activator.Activate(context.Background(), domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientVSCode},
	}); err != nil {
		t.Fatal(err)
	}
	got := stderr.String()
	if !strings.Contains(got, "copying") || !strings.Contains(got, "copied") || !strings.Contains(got, "installing") || !strings.Contains(got, "done") {
		t.Fatalf("decorated progress = %q", got)
	}
	if stager.calls != 1 || activator.calls != 1 {
		t.Fatalf("inner calls stage=%d activate=%d", stager.calls, activator.calls)
	}
}

func TestGroupProgressBoardMarksFailedActivate(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	board := newGroupProgressBoard(&stderr, []domain.DetectedClient{{ClientID: domain.ClientKiro, DisplayName: "Kiro"}}, true)
	service := board.decorate(usecase.Service{Activator: &recordingProgressActivator{err: errors.New("denied")}})
	_, err := service.Activator.Activate(context.Background(), domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientKiro},
	})
	if err == nil || !strings.Contains(stderr.String(), "failed") {
		t.Fatalf("failed activate = err %v board %q", err, stderr.String())
	}
}

func TestProgressPipelineChangesStepColors(t *testing.T) {
	t.Parallel()
	theme := terminaltheme.Theme{Enabled: true}
	queued := progressPipeline(theme, groupProgressRow{step: groupProgressQueued})
	if !strings.Contains(queued, "copying") || !strings.Contains(queued, "copied") || !strings.Contains(queued, "…") || strings.Contains(queued, "installing") || strings.Contains(queued, "done") {
		t.Fatalf("queued pipeline = %q", queued)
	}
	installing := progressPipeline(theme, groupProgressRow{step: groupProgressActivating})
	if !strings.Contains(installing, "installing") || strings.Contains(installing, "…") {
		t.Fatalf("installing pipeline = %q", installing)
	}
	done := progressPipeline(theme, groupProgressRow{step: groupProgressDone})
	if !strings.Contains(done, "done") {
		t.Fatalf("done pipeline = %q", done)
	}
	failed := progressPipeline(theme, groupProgressRow{step: groupProgressActivating, failed: true})
	if !strings.Contains(failed, "failed") || strings.Contains(failed, "installing") {
		t.Fatalf("failed pipeline = %q", failed)
	}
	current := progressToken(theme, "copying", markCurrent)
	complete := progressToken(theme, "copying", markDone)
	pending := progressToken(theme, "copying", markPending)
	errored := progressToken(theme, "copying", markFailed)
	if current == complete || complete == pending || current == pending || errored == current {
		t.Fatalf("step colors did not change: current=%q done=%q pending=%q failed=%q", current, complete, pending, errored)
	}
}

type recordingProgressStager struct {
	calls int
}

func (stager *recordingProgressStager) Stage(context.Context, domain.PackageEnvelope, domain.DeliveryPlan, string, domain.CompatibilityHints) (domain.StagedDelivery, error) {
	stager.calls++
	return domain.StagedDelivery{}, nil
}

func (*recordingProgressStager) Discard(context.Context, domain.StagedDelivery) error { return nil }

func (*recordingProgressStager) Verify(context.Context, string, string) error { return nil }

type recordingProgressActivator struct {
	calls   int
	outcome domain.ActivationOutcome
	err     error
}

func (activator *recordingProgressActivator) Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	activator.calls++
	return activator.outcome, activator.err
}

func (*recordingProgressActivator) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return domain.DeactivationOutcome{}, nil
}
