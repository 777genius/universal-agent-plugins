package cline

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
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

// An unrelated binding must never trigger a native MCP config read.
// Nil NativeConfig makes an attempted unauthorized read fail immediately.
func TestSessionRejectsForeignBindingBeforeNativeConfigRead(t *testing.T) {
	root := t.TempDir()
	req := clients.SessionRequest{
		Client: domain.DetectedClient{
			ClientID: domain.ClientCline, Status: domain.DetectionDetected,
			ExecutablePath: filepath.Join(root, "cline"),
			Surfaces:       []domain.ClientSurface{{ID: "cline_cli", Detected: true}},
		},
		ProjectRoot: root,
		Bindings:    []domain.ClientBinding{{Scope: "user"}},
	}
	if _, err := New().PlanSession(context.Background(), clients.Env{}, req); err == nil || !strings.Contains(err.Error(), "Project scope") {
		t.Fatalf("foreign binding was not refused before file access: %v", err)
	}
}

func TestSessionWholeFileRefusesForeignDisabledAndMalformedMembers(t *testing.T) {
	// Use platform-native absolute paths, including Windows drive/share paths.
	// Relative launchers must never be resolved through the host PATH or cwd.
	executable, err := json.Marshal(filepath.Join(t.TempDir(), "managed", "probe"))
	if err != nil {
		t.Fatal(err)
	}
	directory, err := json.Marshal(filepath.Join(t.TempDir(), "project"))
	if err != nil {
		t.Fatal(err)
	}
	nativePaths := func(body string) string {
		body = strings.ReplaceAll(body, `"/managed/probe"`, string(executable))
		return strings.ReplaceAll(body, `"/project"`, string(directory))
	}
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
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"managed/probe","cwd":"/project"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"relative/project"}}}}`,
	} {
		if err := exactSessionServers([]byte(nativePaths(body)), []string{"owned"}); err == nil {
			t.Fatal("unsafe session source accepted", body)
		}
	}
	for _, body := range []string{
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"disabled":false}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"disabled":false,"autoApprove":[]}}}`,
	} {
		if err := exactSessionServers([]byte(nativePaths(body)), []string{"owned"}); err != nil {
			t.Fatal(err)
		}
	}
}
