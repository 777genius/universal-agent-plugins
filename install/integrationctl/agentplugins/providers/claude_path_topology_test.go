package providers

import (
	"context"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeBundledStdioObservesIsolatedRuntime(t *testing.T) {
	for _, cwd := range []string{"./work", "./data/..", "${PLUGIN_ROOT}"} {
		t.Run(cwd, func(t *testing.T) {
			envelope := stagingEnvelope(t)
			for _, dir := range []string{"work", "data"} {
				if err := os.MkdirAll(filepath.Join(envelope.SnapshotRoot, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			envelope.MCP.Servers = map[string]domain.MCPServer{"local": {Name: "local", Type: "stdio", Decoded: map[string]any{
				"type": "stdio", "command": "./bin/../bin/run", "cwd": cwd, "args": []any{"${PLUGIN_ROOT}/config", "${UNKNOWN}"},
			}}}
			plan := stagingPlan(t, domain.ClientClaude, domain.PackageProjection)
			plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportProjected}}
			delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "claude-bundled-paths", domain.CompatibilityHints{})
			if err != nil {
				t.Fatal(err)
			}
			observed := filepath.Join(delivery.StagingPath, claudeRuntimeDirectory)
			if _, err := os.Stat(filepath.Join(delivery.StagingPath, "bin")); !os.IsNotExist(err) {
				t.Fatalf("authored bin remained exposed: %v", err)
			}
			if _, err := os.Stat(filepath.Join(observed, "bin/run")); err != nil {
				t.Fatal(err)
			}
			server := readObject(t, filepath.Join(delivery.StagingPath, ".mcp.json"))["local"].(map[string]any)
			activeRuntime := filepath.Join(plan.ActivePath, claudeRuntimeDirectory)
			expectedCWD := activeRuntime
			if cwd == "./work" {
				expectedCWD = filepath.Join(activeRuntime, "work")
			}
			if server["command"] != filepath.Join(activeRuntime, "bin/run") || server["cwd"] != expectedCWD {
				t.Fatalf("isolated stdio projection: %+v", server)
			}
			if args := server["args"].([]any); args[0] != filepath.Join(activeRuntime, "config") || args[1] != "${UNKNOWN}" {
				t.Fatalf("args: %+v", args)
			}
			if err := (Stager{}).Verify(context.Background(), delivery.StagingPath, delivery.ArtifactDigest); err != nil {
				t.Fatal(err)
			}
			if err := (Stager{}).Discard(context.Background(), delivery); err != nil {
				t.Fatal(err)
			}
		})
	}
}
