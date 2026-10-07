package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

// Regression: generic HTTP installation must qualify its own transport without
// requiring observer support. This verifies config delivery, not a host call.
func TestOpenCodeHTTPQualificationFlowsIntoNativeDelivery(t *testing.T) {
	root := openCodeTestRoot(t)
	executable := buildOpenCodeTarget(t, filepath.Join(root, "v1"), "1.18.34", "ok")
	req := openCodeRequest(t, root, executable)
	mcp := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"sample-notify":{"type":"streamable-http","url":"https://fixture.invalid/mcp"}}}`)
	if err := os.WriteFile(filepath.Join(req.PackageRoot, "mcp.json"), mcp, 0600); err != nil {
		t.Fatal(err)
	}
	engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: executable, OpenCodeProbeEnvironment: []string{"PATH="}})
	handle, err := engine.Prepare(testCtx(t), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = handle.Close() }()
	plan := handle.Plan()
	if plan.OpenCodeProfile.Capabilities[opencodehost.MCPStreamableHTTP] != opencodehost.Supported || plan.OpenCodeProfile.Capabilities[opencodehost.ObserverPermission] != opencodehost.Unverified {
		t.Fatalf("capabilities: %+v", plan.OpenCodeProfile)
	}
	if _, err := engine.Apply(testCtx(t), handle, Decision{Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(req.ClientConfigRoot, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		MCP map[string]struct{ Type, URL string }
	}
	if err := json.Unmarshal(body, &config); err != nil {
		t.Fatal(err)
	}
	if server := config.MCP["sample-notify"]; server.Type != "remote" || server.URL != "https://fixture.invalid/mcp" {
		t.Fatalf("HTTP delivery: %s", body)
	}
}
