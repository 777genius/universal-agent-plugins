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
	for _, definition := range domain.ClientDefinitions() {
		_, ok := clients.As[clients.SessionPlanner](registry, definition.ID)
		if ok != (string(definition.ID) == "cline") {
			t.Fatalf("unqualified client session capability: %s", definition.ID)
		}
	}
}

func TestSessionRequiresSeparateConsentAndNoArbitraryChildArguments(t *testing.T) {
	for _, args := range [][]string{
		{"run"}, {"run", "--yes"}, {"run", "--confirm-client-start", "--", "sh", "-c", "anything"},
		{"run", "--target", "cline", "--scope", "user", "--confirm-client-start"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out bytes.Buffer
			root := NewRoot(App{Output: &out, ErrorOutput: &out})
			root.SetArgs(args)
			if err := root.Execute(); err == nil {
				t.Fatal("unsafe session invocation accepted", args)
			}
		})
	}
}
