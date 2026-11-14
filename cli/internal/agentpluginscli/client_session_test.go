package agentpluginscli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	clientregistry "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSessionCapabilityIsOptIn(t *testing.T) {
	registry := clientregistry.Project()
	// Registration exposes planning; it does not qualify a runtime or scope.
	// Each adapter still owns PlanSession and VerifySession admission.
	expected := map[domain.ClientID]bool{domain.ClientCline: true, domain.ClientHermes: true}
	for _, definition := range domain.ClientDefinitions() {
		_, ok := clients.As[clients.SessionPlanner](registry, definition.ID)
		if ok != expected[definition.ID] {
			t.Fatalf("unexpected registered client session capability: %s", definition.ID)
		}
	}
}

func TestSessionRequiresSeparateConsentAndNoArbitraryChildArguments(t *testing.T) {
	for _, target := range []string{"", string(domain.ClientCline), string(domain.ClientHermes)} {
		for _, test := range []struct {
			arguments []string
			refusal   string
		}{
			{nil, "client startup is separate from installation"},
			{[]string{"--yes"}, "unknown flag: --yes"},
			{[]string{"--confirm-client-start", "--", "sh", "-c", "anything"}, "unknown command"},
			{[]string{"--scope", "user", "--confirm-client-start"}, "client startup requires one explicit --target and --scope project"},
		} {
			args := []string{"run"}
			if target != "" {
				args = append(args, "--target", target)
			}
			args = append(args, test.arguments...)
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				var out bytes.Buffer
				root := NewRoot(App{Output: &out, ErrorOutput: &out})
				root.SetArgs(args)
				if err := root.Execute(); err == nil || !strings.Contains(err.Error(), test.refusal) {
					t.Fatalf("session refusal = %v, want %q", err, test.refusal)
				}
			})
		}
	}
}
