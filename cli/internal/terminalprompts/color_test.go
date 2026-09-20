package terminalprompts

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestPlainColoredAndRichNever(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out strings.Builder
	format := "human"
	policy := terminaltheme.Policy{Mode: "always", Explicit: true}
	p := PlainPrompter{Input: strings.NewReader("yes\n"), Output: terminaltheme.Wrap(&out, &policy, &format)}
	result, err := p.Confirm(context.Background(), prompt.ConfirmationRequest{Title: "Apply fixture?"})
	if err != nil || !result.Accepted || !strings.Contains(out.String(), "\x1b[36mApply fixture?\x1b[m") {
		t.Fatal(result, err, out.String())
	}
	out.Reset()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err = (HuhPrompter{Input: strings.NewReader("\r"), Output: &out, NoColor: true}).Confirm(ctx, prompt.ConfirmationRequest{Title: "Apply fixture?"})
	if err != nil || result.Accepted {
		t.Fatal(result, err)
	}
	if regexp.MustCompile(`\x1b\[[0-9;]*m`).MatchString(out.String()) {
		t.Fatalf("SGR in color-disabled rich prompt: %q", out.String())
	}
}

func TestSyntheticColorSelectionDemo(t *testing.T) {
	var out strings.Builder
	format := "human"
	policy := terminaltheme.Policy{Mode: "always", Explicit: true}
	p := PlainPrompter{Input: strings.NewReader("1\n"), Output: terminaltheme.Wrap(&out, &policy, &format)}
	request := prompt.TargetSelectionRequest{Choices: []prompt.TargetChoice{{ID: "cursor", Label: "Synthetic Cursor"}, {ID: "codex", Label: "Synthetic Codex"}}, DefaultIDs: []domain.ClientID{"cursor", "codex"}, SkippedLabels: []string{"Synthetic unsupported client"}}
	selected, err := p.SelectTargets(context.Background(), request)
	if err != nil || len(selected.IDs) != 1 || selected.IDs[0] != "cursor" {
		t.Fatal(selected, err)
	}
	if !strings.Contains(out.String(), "\x1b[33mSkipped") || !strings.Contains(out.String(), "\x1b[36mChoose") {
		t.Fatal("missing semantic selection colors")
	}
	t.Log("Synthetic selection; input is the in-memory line 1, not a live terminal session.\n" + out.String())
}
