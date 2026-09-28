package agentpluginscli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestGroupProgressBoardPlainFallbackKeepsActualOutcomes(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	clients := []domain.DetectedClient{
		{ClientID: domain.ClientCodex, DisplayName: "OpenAI Codex"},
		{ClientID: domain.ClientClaude, DisplayName: "Claude Code"},
	}
	board := newGroupProgressBoard(&output, clients)
	board.observe(usecase.GroupProgressEvent{ClientID: domain.ClientCodex, Phase: usecase.GroupProgressPreparing})
	board.observe(usecase.GroupProgressEvent{ClientID: domain.ClientCodex, Phase: usecase.GroupProgressConfiguring})
	board.observe(usecase.GroupProgressEvent{ClientID: domain.ClientCodex, Phase: usecase.GroupProgressConfigured})
	board.observe(usecase.GroupProgressEvent{ClientID: domain.ClientCodex, Phase: usecase.GroupProgressActivated, Result: usecase.AddResult{
		GroupPhase: usecase.GroupTargetExternalCompleted,
		Activation: domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotRequired},
	}})
	board.finish([]usecase.AddResult{
		{GroupPhase: usecase.GroupTargetExternalCompleted, Activation: domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotRequired}},
		{GroupPhase: usecase.GroupTargetExternalNotAttempted},
	})
	got := output.String()
	for _, want := range []string{"OpenAI Codex", "Claude Code", "● preparing", "✓ preparing → ● configuring", "✓ configuring → ● installing", "✓ installed", "! not completed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("live board missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "\033[") {
		t.Fatal("nonterminal fallback emitted cursor controls")
	}
	if board.painted != 0 {
		t.Fatal("board retained cursor position after finish")
	}
}

func TestGroupProgressBoardDoesNotStartForNonTTYOrPlain(t *testing.T) {
	t.Parallel()
	selected := []domain.DetectedClient{{ClientID: domain.ClientCodex}, {ClientID: domain.ClientClaude}}
	if board := startGroupProgressBoard(App{Terminal: true, ErrorOutput: &bytes.Buffer{}}, &options{format: "human"}, selected); board != nil {
		t.Fatal("redirected stderr must keep the plain progress fallback")
	}
	if board := startGroupProgressBoard(App{Terminal: true}, &options{format: "json"}, selected); board != nil {
		t.Fatal("JSON must never receive a live board")
	}
	if board := startGroupProgressBoard(App{Terminal: true}, &options{format: "human", plain: true}, selected); board != nil {
		t.Fatal("--plain must never receive a live board")
	}
}
