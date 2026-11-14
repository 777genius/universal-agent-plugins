package opencode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Whole-envelope validation makes the excluded historical SSE case red before
// projection. Dropping selected unknown transports or missing names makes the
// refusal cases red at RefinePlan, before any staging/native effect is allowed.
func TestOpenCodeSelectedNativeEffects(t *testing.T) {
	for _, transport := range []string{"streamable-http", "sse", "unknown", "missing"} {
		t.Run(transport, func(t *testing.T) {
			root := t.TempDir()
			config, staging := filepath.Join(root, "config"), filepath.Join(root, "staging")
			for _, dir := range []string{config, staging} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			foreign := []byte("{ // foreign comment\n\"mcp\":{\"foreign\":{\"type\":\"remote\",\"url\":\"https://foreign.test\",\"enabled\":false}}}\n")
			configPath := filepath.Join(config, "opencode.jsonc")
			if err := os.WriteFile(configPath, foreign, 0600); err != nil {
				t.Fatal(err)
			}
			host := contracttest.OpenCodeV1Host{}
			envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
				"selected":   {Type: transport, Decoded: map[string]any{"url": "https://selected.test/mcp"}},
				"historical": {Type: "sse"},
			}}}
			if transport == "missing" {
				delete(envelope.MCP.Servers, "selected")
			}
			plan := domain.DeliveryPlan{NativeRegistryRoot: config, ActivePath: filepath.Join(root, "active"), Components: []domain.ComponentDecision{
				{Kind: domain.ComponentMCPServer, Name: "selected", Support: domain.SupportProjected},
				{Kind: domain.ComponentMCPServer, Name: "historical", Support: domain.SupportUnsupported},
			}}
			adapter := &Adapter{}
			err := adapter.RefinePlan(context.Background(), clients.PlanInput{Client: domain.DetectedClient{ConfigRoot: config, OpenCodeHost: host}, Envelope: envelope}, &plan)
			wantError := transport != "streamable-http"
			if (err != nil) != wantError {
				t.Fatalf("refine selected %s: %v", transport, err)
			}
			// Test Project's independent validation even if refinement refused.
			plan.OpenCodeHost = host
			objects, err := adapter.Project(context.Background(), clients.ProjectionInput{StagingPath: staging, Envelope: envelope, Plan: plan})
			if (err != nil) != wantError {
				t.Fatalf("project selected %s: %v", transport, err)
			}
			if wantError {
				if _, err := os.Lstat(filepath.Join(staging, OpenCodeProjectionFile)); !os.IsNotExist(err) {
					t.Fatal("refused selection wrote projection")
				}
			} else {
				if len(objects) != 1 || objects[0].ObjectID != "opencode-mcp:selected" {
					t.Fatalf("unexpected ownership: %+v", objects)
				}
				body, err := os.ReadFile(filepath.Join(staging, OpenCodeProjectionFile))
				if err != nil || strings.Contains(string(body), "historical") {
					t.Fatalf("excluded SSE rendered: %s, %v", body, err)
				}
			}
			if body, err := os.ReadFile(configPath); err != nil || string(body) != string(foreign) {
				t.Fatalf("staging changed foreign config: %s, %v", body, err)
			}
		})
	}
}
