package pluginkitai

import (
	"os"
	"testing"

	internalclaude "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/claude"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// Golden fixtures (testdata/golden/) freeze minimal runtime wire shapes for regression coverage.
func TestGolden_PreToolUseMinDecodes(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden/pretooluse_min.json")
	if err != nil {
		t.Fatal(err)
	}
	evAny, _, err := internalclaude.DecodePreToolUse(runtime.Envelope{Stdin: raw})
	if err != nil {
		t.Fatal(err)
	}
	ev := evAny.(*internalclaude.PreToolUseInput)
	if ev.ToolName != "Bash" || ev.SessionID != "golden-session" {
		t.Fatalf("%+v", ev)
	}
	if len(ev.ToolInput) == 0 {
		t.Fatal("tool_input missing")
	}
}

func TestGolden_UserPromptMinDecodes(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden/userprompt_min.json")
	if err != nil {
		t.Fatal(err)
	}
	evAny, _, err := internalclaude.DecodeUserPromptSubmit(runtime.Envelope{Stdin: raw})
	if err != nil {
		t.Fatal(err)
	}
	ev := evAny.(*internalclaude.UserPromptSubmitInput)
	if ev.Prompt != "hello" {
		t.Fatalf("%+v", ev)
	}
}
