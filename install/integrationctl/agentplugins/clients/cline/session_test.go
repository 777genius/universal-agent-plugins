package cline

import (
	"context"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSessionRequiresInstalledClientBeforeFileAccess(t *testing.T) {
	for _, client := range []domain.DetectedClient{{}, {ClientID: domain.ClientCline, ExecutablePath: "relative", Status: domain.DetectionDetected}} {
		req := clients.SessionRequest{Client: client, Bindings: []domain.ClientBinding{{Scope: "project"}}}
		if _, err := New().PlanSession(context.Background(), clients.Env{}, req); err == nil {
			t.Fatal("unavailable client accepted")
		}
	}
}

func TestSessionWholeFileRefusesForeignDisabledAndMalformedMembers(t *testing.T) {
	for _, body := range []string{
		`{"mcpServers":{}}`,
		`{"mcpServers":{"owned":{},"foreign":{}}}`,
		`{"mcpServers":{"owned":{"disabled":true}}}`,
		`{"mcpServers":{"owned":{"disabled":"false"}}}`,
		`{"mcpServers":{"owned":{"autoApprove":["read_file"]}}}`,
		`{"mcpServers":{"owned":{"autoApprove":["*"]}}}`,
		`{"mcpServers":{"owned":{"autoApprove":true}}}`,
		`{"mcpServers":{"owned":{"autoApprove":null}}}`,
		`{"mcpServers":{"owned":{"autoApprove":{}}}}`,
		`{"mcpServers":{"owned":{}} ,"extra":{}}`,
		`{"mcpServers":{"owned":{},"owned":{}}}`,
		`{"mcpServers":{"owned":null}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"sse","url":"http://localhost"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"http","url":"http://localhost"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","cwd":"/project"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"","cwd":"/project"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":""}}}}`,
	} {
		if err := exactSessionServers([]byte(body), []string{"owned"}); err == nil {
			t.Fatal("unsafe session source accepted", body)
		}
	}
	for _, body := range []string{
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"disabled":false}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"disabled":false,"autoApprove":[]}}}`,
	} {
		if err := exactSessionServers([]byte(body), []string{"owned"}); err != nil {
			t.Fatal(err)
		}
	}
}
