package agentpluginscli

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestNormalizeTargetContract pins the exact --target vocabulary: eleven
// canonical ids, six aliases, case and whitespace folding, and the lenient
// pass-through that turns an unknown name into a lowercased ClientID instead of
// an error. Part 10 replaces this function with domain.ParseClientID, which has
// to keep all four properties.
func TestNormalizeTargetContract(t *testing.T) {
	t.Parallel()
	canonical := map[string]domain.ClientID{
		"codex":    domain.ClientCodex,
		"chatgpt":  domain.ClientChatGPT,
		"cursor":   domain.ClientCursor,
		"copilot":  domain.ClientCopilot,
		"vscode":   domain.ClientVSCode,
		"kiro":     domain.ClientKiro,
		"claude":   domain.ClientClaude,
		"gemini":   domain.ClientGemini,
		"opencode": domain.ClientOpenCode,
		"cline":    domain.ClientCline,
		"windsurf": domain.ClientWindsurf,
	}
	if len(canonical) != len(domain.SupportedClientIDs()) {
		t.Fatalf("the table covers %d clients, the registry has %d", len(canonical), len(domain.SupportedClientIDs()))
	}
	for input, want := range canonical {
		if got := normalizeTarget(input); got != want {
			t.Errorf("normalizeTarget(%q) = %q, want %q", input, got, want)
		}
	}

	aliases := map[string]domain.ClientID{
		"github-copilot": domain.ClientCopilot,
		"vs-code":        domain.ClientVSCode,
		"claude-code":    domain.ClientClaude,
		"gemini-cli":     domain.ClientGemini,
		"open-code":      domain.ClientOpenCode,
		"devin":          domain.ClientWindsurf,
	}
	for input, want := range aliases {
		if got := normalizeTarget(input); got != want {
			t.Errorf("normalizeTarget(%q) = %q, want %q", input, got, want)
		}
	}

	folding := map[string]domain.ClientID{
		"  cursor  ":      domain.ClientCursor,
		"CURSOR":          domain.ClientCursor,
		"Claude-Code":     domain.ClientClaude,
		" GitHub-Copilot": domain.ClientCopilot,
	}
	for input, want := range folding {
		if got := normalizeTarget(input); got != want {
			t.Errorf("normalizeTarget(%q) = %q, want %q", input, got, want)
		}
	}

	// Unknown names are not rejected here: they pass through lowercased and are
	// refused later, where the error can name the supported targets.
	passThrough := map[string]domain.ClientID{
		"Not-A-Client": domain.ClientID("not-a-client"),
		"  Zed  ":      domain.ClientID("zed"),
		"":             domain.ClientID(""),
	}
	for input, want := range passThrough {
		if got := normalizeTarget(input); got != want {
			t.Errorf("normalizeTarget(%q) = %q, want %q", input, got, want)
		}
	}
}
