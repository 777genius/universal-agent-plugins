package agentpluginscli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestClientRegistryDrivesTargetValidationOrderAndAliases(t *testing.T) {
	targets, err := parseTargetOption("windsurf,claude,codex,qoder,opencode,cline,gemini")
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.ClientID{domain.ClientCodex, domain.ClientClaude, domain.ClientGemini, domain.ClientOpenCode, domain.ClientCline, domain.ClientWindsurf, domain.ClientQoder}
	if !reflect.DeepEqual(targets, want) {
		t.Fatalf("targets = %v, want %v", targets, want)
	}
	if normalizeTarget("devin") != domain.ClientWindsurf || normalizeTarget("claude-code") != domain.ClientClaude {
		t.Fatal("new client aliases were not normalized")
	}
	if normalizeTarget("qoder-cli") != domain.ClientQoder || normalizeTarget("qoder-desktop") != domain.ClientQoder {
		t.Fatal("qoder aliases were not normalized onto the shared identity")
	}
	for _, id := range []string{"claude", "gemini", "opencode", "cline", "windsurf", "qoder"} {
		if !strings.Contains(supportedTargetHelp(), id) {
			t.Fatalf("target help missing %q: %s", id, supportedTargetHelp())
		}
	}
}
