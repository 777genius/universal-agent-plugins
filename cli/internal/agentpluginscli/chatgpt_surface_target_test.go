package agentpluginscli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestChatGPTSurfaceTargetParsingAndScope(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		want []domain.ClientID
	}{
		{"chatgpt-desktop", []domain.ClientID{domain.ClientCodex}},
		{"chatgpt-web", []domain.ClientID{domain.ClientChatGPT}},
		{"CHATGPT-DESKTOP,CHATGPT-WEB", []domain.ClientID{domain.ClientCodex, domain.ClientChatGPT}},
		{"codex,chatgpt", []domain.ClientID{domain.ClientCodex, domain.ClientChatGPT}},
	} {
		got, err := parseTargetOption(tc.raw)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("parseTargetOption(%q) = %v, %v; want %v", tc.raw, got, err, tc.want)
		}
	}
	for _, raw := range []string{"codex,chatgpt-desktop", "chatgpt,chatgpt-web"} {
		_, err := parseTargetOption(raw)
		if err == nil || !strings.Contains(err.Error(), "duplicate client") {
			t.Errorf("expected a duplicate backend refusal for %q; got %v", raw, err)
		}
	}
	for _, scope := range []string{"project", "user"} {
		for _, raw := range []string{" chatgpt-desktop ", "codex,CHATGPT-DESKTOP"} {
			err := validateCommonOptions(&options{format: "human", scope: scope, target: raw})
			if scope == "project" && err == nil {
				t.Errorf("ChatGPT Desktop must refuse project scope, %q", raw)
			}
		}
	}
}
