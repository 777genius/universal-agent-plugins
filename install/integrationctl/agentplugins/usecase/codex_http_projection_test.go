package usecase

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestCodexStreamableHTTPProjectionPreservesLiteralURLAndHeaders proves the
// one piece of the "HTTP headers and redirects" requirement that is actually
// UAP's own responsibility and testable without a live Codex process: the
// declared streamable-http MCP server's URL and literal header map survive
// projection into both files UAP writes for Codex -- the client-facing
// .mcp.json (built by projectOpenAI from the decoded config) and the
// portable package's own mcp.json (built by writeSanitizedMCP from the
// original raw bytes) -- byte-for-byte, through the real production stager
// (no fakes). Both Raw and Decoded are set on the fixture server, matching
// what the real loader always produces (adapters/loader/mcp.go), so this
// exercises both projection paths honestly. The .mcp.json projection of a
// declared streamable-http server as "type": "http" is a documented,
// intentional decision, not a bug -- see docs/CODEX_TRANSPORT_EVIDENCE.md
// for the source evidence behind it; that specific assertion already exists
// in providers/stager_test.go, it is repeated here only as a sanity check
// alongside the header assertions. The literal header-map preservation
// (across both files) is what this test actually adds.
//
// What this does NOT prove, and cannot prove without a live native Codex
// client connection: whether Codex's own HTTP client actually sends these
// exact headers on the first request, honors client-generated protocol/auth
// header priority, or refuses to forward a configured header to a different
// origin on redirect. Those are native client runtime properties, not UAP
// config-projection properties; no test at this level, or currently at any
// level in this codebase for any client, proves them. This remains an
// honest, explicitly unproven gap for every client in this codebase, not
// just Codex.
func TestCodexStreamableHTTPProjectionPreservesLiteralURLAndHeaders(t *testing.T) {
	t.Parallel()
	service, _, _ := serviceFixture(t)
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	install := addInput(t, codex, "https://example.com/codex-http-headers")
	serverRaw := json.RawMessage(`{"type":"streamable-http","url":"https://example.invalid/mcp","headers":{"X-Test-Literal":"keep-me-exact","Authorization":"Bearer test-token-not-a-real-secret"}}`)
	install.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
		"remote": {Name: "remote", Type: "streamable-http", Raw: serverRaw, Decoded: map[string]any{
			"type": "streamable-http",
			"url":  "https://example.invalid/mcp",
			"headers": map[string]any{
				"X-Test-Literal": "keep-me-exact",
				"Authorization":  "Bearer test-token-not-a-real-secret",
			},
		}},
	}}
	install.Envelope.Inventory.MCPServers = []string{"remote"}

	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{install}, OperationGroupID: "codex-http-headers-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(added.Targets[0].Plan.ActivePath, ".mcp.json"))
	if err != nil {
		t.Fatalf("read projected .mcp.json: %v", err)
	}
	var projected struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &projected); err != nil {
		t.Fatalf("decode projected .mcp.json: %v\n%s", err, body)
	}
	remote, ok := projected.MCPServers["remote"]
	if !ok {
		t.Fatalf("remote server missing from projection: %s", body)
	}
	if remote["url"] != "https://example.invalid/mcp" {
		t.Fatalf("url not preserved literally: %+v", remote)
	}
	headers, ok := remote["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers missing or wrong shape from projection: %+v", remote)
	}
	if headers["X-Test-Literal"] != "keep-me-exact" {
		t.Fatalf("custom header not preserved literally: %+v", headers)
	}
	if headers["Authorization"] != "Bearer test-token-not-a-real-secret" {
		t.Fatalf("authorization header not preserved literally: %+v", headers)
	}
	if remote["type"] != "http" {
		t.Fatalf(`.mcp.json type must be "http" for a declared streamable-http server: %+v`, remote)
	}

	portableBody, err := os.ReadFile(filepath.Join(added.Targets[0].Plan.ActivePath, "mcp.json"))
	if err != nil {
		t.Fatalf("read portable mcp.json: %v", err)
	}
	var portable struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(portableBody, &portable); err != nil {
		t.Fatalf("decode portable mcp.json: %v\n%s", err, portableBody)
	}
	portableRemote, ok := portable.MCPServers["remote"]
	if !ok || portableRemote == nil {
		t.Fatalf("portable mcp.json did not preserve the raw server entry: %s", portableBody)
	}
	if portableRemote["url"] != "https://example.invalid/mcp" {
		t.Fatalf("portable mcp.json url not preserved literally: %+v", portableRemote)
	}
	portableHeaders, ok := portableRemote["headers"].(map[string]any)
	if !ok || portableHeaders["Authorization"] != "Bearer test-token-not-a-real-secret" || portableHeaders["X-Test-Literal"] != "keep-me-exact" {
		t.Fatalf("portable mcp.json headers not preserved literally: %+v", portableRemote)
	}
	if portableRemote["type"] != "streamable-http" {
		t.Fatalf("portable mcp.json must keep the original declared transport, not the client-projected one: %+v", portableRemote)
	}
}
