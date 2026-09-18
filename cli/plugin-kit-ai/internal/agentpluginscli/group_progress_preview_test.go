package agentpluginscli

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestLiveProgressBoardPreview(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_PROGRESS_DEMO") != "1" {
		t.Skip("set AGENTPLUGINS_PROGRESS_DEMO=1 and pass -v; go test hides passing output without -v")
	}
	selected := []domain.DetectedClient{
		{ClientID: domain.ClientClaude, DisplayName: "Claude Code"},
		{ClientID: domain.ClientCodex, DisplayName: "OpenAI Codex"},
		{ClientID: domain.ClientCursor, DisplayName: "Cursor"},
		{ClientID: domain.ClientCopilot, DisplayName: "GitHub Copilot CLI"},
		{ClientID: domain.ClientGemini, DisplayName: "Gemini CLI"},
		{ClientID: domain.ClientOpenCode, DisplayName: "OpenCode"},
		{ClientID: domain.ClientKiro, DisplayName: "Kiro"},
		{ClientID: domain.ClientWindsurf, DisplayName: "Windsurf / Devin"},
	}
	format := "human"
	policy := terminaltheme.Policy{Mode: "always", Explicit: true}
	writer := terminaltheme.Wrap(os.Stderr, &policy, &format)
	_, _ = os.Stderr.WriteString("Applying to every preflighted client...\n")
	board := newGroupProgressBoard(writer, selected, true)
	defer board.finish()
	pause := 450 * time.Millisecond
	var wait sync.WaitGroup
	for index, client := range selected {
		wait.Add(1)
		go func(index int, client domain.DetectedClient) {
			defer wait.Done()
			time.Sleep(time.Duration(index) * 80 * time.Millisecond)
			board.set(client.ClientID, groupProgressStaging)
			time.Sleep(pause)
			board.set(client.ClientID, groupProgressCopied)
			time.Sleep(pause)
			board.set(client.ClientID, groupProgressActivating)
			time.Sleep(pause)
			if client.ClientID == domain.ClientKiro {
				board.set(client.ClientID, groupProgressFailed)
				return
			}
			board.set(client.ClientID, groupProgressDone)
		}(index, client)
	}
	wait.Wait()
	time.Sleep(2 * time.Second)
}
