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
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"disabled":true}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"disabled":"false"}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"autoApprove":["read_file"]}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"autoApprove":["*"]}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"autoApprove":true}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"autoApprove":null}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"autoApprove":{}}}}`,
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
