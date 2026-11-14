package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exact-base 55c5b5b OpenCode SSE artifacts retain portable type:sse in mcp.json
// and render the native entry as {type:remote,url:...}. Reconstruct those bytes
// with owned native receipts here; the new implementation never renders SSE.
func TestHistoricalSSESameRevisionLifecycle(t *testing.T) {
	for _, op := range []string{"add", "update", "repair", "group_add", "group_update", "group_repair"} {
		t.Run(op, func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode")}
			input := openCodePluginInput(t, client, "1.0.0", "sha256:historical-sse", "sha256:historical-sse-manifest", "unused")
			portable := func(kind string) domain.MCPServer {
				raw := json.RawMessage(`{"type":"` + kind + `","url":"https://example.test/events"}`)
				return domain.MCPServer{Name: "docs", Type: kind, Raw: raw, Decoded: map[string]any{"type": kind, "url": "https://example.test/events"}}
			}
			input.Envelope.MCP.Servers = map[string]domain.MCPServer{"docs": portable("streamable-http")}
			input.Confirmed = true
			installed, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			// The historical native representation equals HTTP-first remote, but the
			// authoritative source and retained portable artifact explicitly declare SSE.
			input.Envelope.MCP.Servers["docs"] = portable("sse")
			legacy := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"sse","url":"https://example.test/events"}}}`)
			input.Envelope.MCP.Raw = legacy
			for _, root := range []string{input.Envelope.SnapshotRoot, installed.Plan.ActivePath} {
				if err := os.WriteFile(filepath.Join(root, "mcp.json"), legacy, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var mismatch *ports.VerificationError
			if err := service.Stager.Verify(context.Background(), installed.Plan.ActivePath, "observe-historical"); !errors.As(err, &mismatch) || mismatch.ActualDigest == "" {
				t.Fatal(err)
			}
			state, _ := store.Load()
			for key, binding := range state.Installations[0].Clients {
				for i := range binding.NativeObjects {
					if binding.NativeObjects[i].Kind == "managed_package_directory" {
						binding.NativeObjects[i].ManagedDigest = mismatch.ActualDigest
					}
				}
				state.Installations[0].Clients[key] = binding
			}
			if err := store.Save(state); err != nil {
				t.Fatal(err)
			}
			input.InstallationID = installed.InstallationID
			input.OperationID = "historical-followup"
			configPath := filepath.Join(client.ConfigRoot, "opencode.json")
			if strings.HasSuffix(op, "repair") {
				if err := os.WriteFile(configPath, []byte(`{"mcp":{}}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(configPath)
			var result AddResult
			if strings.HasPrefix(op, "group_") {
				args := GroupInput{Targets: []AddInput{input}, Confirmed: true, OperationGroupID: "historical-next"}
				var g GroupResult
				switch op {
				case "group_add":
					g, err = service.AddGroup(context.Background(), args)
				case "group_update":
					g, err = service.UpdateGroup(context.Background(), args)
				default:
					g, err = service.RepairGroup(context.Background(), args)
				}
				if len(g.Targets) > 0 {
					result = g.Targets[0]
				}
			} else {
				switch op {
				case "add":
					result, err = service.Add(context.Background(), input)
				case "update":
					result, err = service.Update(context.Background(), input)
				default:
					result, err = service.Repair(context.Background(), input)
				}
			}
			after, _ := os.ReadFile(configPath)
			if strings.HasSuffix(op, "update") {
				if err != nil || !result.Mutated || result.NoChange || strings.Contains(string(after), "events") {
					t.Fatalf("update result=%+v err=%v config=%s", result, err, after)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "explicit update") || result.Mutated || string(before) != string(after) {
					t.Fatalf("refusal result=%+v err=%v", result, err)
				}
			}
		})
	}
}
