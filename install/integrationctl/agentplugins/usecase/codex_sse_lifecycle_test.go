package usecase

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Reconstruct the pre-narrowing Codex capability in memory, retaining the real
// stager, transaction and receipts. No client binary or network is invoked.
type historicalCodexSSEPlanner struct{ ports.DeliveryPlanner }

func (p historicalCodexSSEPlanner) Plan(ctx context.Context, e domain.PackageEnvelope, c domain.DetectedClient, s domain.InstallScope, id string) (domain.DeliveryPlan, error) {
	planning := e
	planning.MCP.Servers = make(map[string]domain.MCPServer, len(e.MCP.Servers))
	for name, server := range e.MCP.Servers {
		if server.Type == "sse" {
			server.Type = "streamable-http"
		}
		planning.MCP.Servers[name] = server
	}
	return p.DeliveryPlanner.Plan(ctx, planning, c, s, id)
}

func codexSSEInput(t *testing.T, client domain.DetectedClient, healthy bool) AddInput {
	t.Helper()
	input := addInput(t, client, "https://example.invalid/codex-sse-fixture")
	input.Confirmed = true
	servers := map[string]domain.MCPServer{"events": {Name: "events", Type: "sse", Decoded: map[string]any{"type": "sse", "url": "https://example.invalid/events"}}}
	if healthy {
		servers["http"] = domain.MCPServer{Name: "http", Type: "streamable-http", Decoded: map[string]any{"type": "streamable-http", "url": "https://example.invalid/mcp"}}
		skill := []byte("---\nname: docs\ndescription: Fixture documentation\n---\nRead fixture documentation.\n")
		path := filepath.Join(input.Envelope.SnapshotRoot, "skills", "docs", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, skill, 0600); err != nil {
			t.Fatal(err)
		}
		input.Envelope.Skills = map[string]domain.Skill{"docs": {Name: "docs", RelativePath: "skills/docs/SKILL.md", Raw: skill}}
	}
	rawServers := map[string]any{}
	for name, server := range servers {
		raw, err := json.Marshal(server.Decoded)
		if err != nil {
			t.Fatal(err)
		}
		server.Raw = raw
		servers[name] = server
		rawServers[name] = server.Decoded
	}
	raw, err := json.Marshal(map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": rawServers})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "mcp.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: servers, Raw: raw}
	return input
}

func TestCodexSSEHistoricalLifecycle(t *testing.T) {
	for _, op := range []string{"add", "repair", "update-preview", "update", "all-unsupported-update"} {
		t.Run(op, func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			client := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "codex")}
			input := codexSSEInput(t, client, op != "all-unsupported-update")
			currentPlanner := service.Planner
			service.Planner = historicalCodexSSEPlanner{currentPlanner}
			installed, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			service.Planner = currentPlanner
			for _, surface := range []string{"mcp.json", ".mcp.json"} {
				servers := readUsecaseObject(t, filepath.Join(installed.Plan.ActivePath, surface))["mcpServers"].(map[string]any)
				if servers["events"].(map[string]any)["type"] != "sse" {
					t.Fatalf("historical %s not SSE: %v", surface, servers)
				}
			}
			beforeState, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			before := codexTransportFiles(t, installed.Plan.ActivePath)
			input.InstallationID = installed.InstallationID
			input.OperationID = "codex-historical-followup"
			input.Confirmed = op != "update-preview"
			var result AddResult
			switch op {
			case "add":
				result, err = service.Add(context.Background(), input)
			case "repair":
				result, err = service.Repair(context.Background(), input)
			default:
				result, err = service.Update(context.Background(), input)
			}
			if op == "update" {
				if err != nil || !result.Mutated || result.NoChange {
					t.Fatalf("update=%+v err=%v", result, err)
				}
				for _, surface := range []string{"mcp.json", ".mcp.json"} {
					servers := readUsecaseObject(t, filepath.Join(installed.Plan.ActivePath, surface))["mcpServers"].(map[string]any)
					if len(servers) != 1 || servers["http"] == nil {
						t.Fatalf("withdrawal %s=%v", surface, servers)
					}
				}
				if _, err := os.Stat(filepath.Join(installed.Plan.ActivePath, "skills", "docs", "SKILL.md")); err != nil {
					t.Fatal(err)
				}
				return
			}
			if op == "update-preview" {
				if err != nil || !result.RequiresConfirmation {
					t.Fatalf("preview=%+v err=%v", result, err)
				}
			} else if err == nil {
				t.Fatalf("%s silently accepted historical SSE: %+v", op, result)
			}
			if (op == "add" || op == "repair") && !strings.Contains(err.Error(), "explicit update") {
				t.Fatalf("unexpected refusal: %v", err)
			}
			afterState, loadErr := store.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if result.Mutated || !reflect.DeepEqual(beforeState, afterState) || !reflect.DeepEqual(before, codexTransportFiles(t, installed.Plan.ActivePath)) {
				t.Fatalf("%s changed installation", op)
			}
		})
	}
}

func TestCodexSSEOnlyAddDoesNotWriteInstallation(t *testing.T) {
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "codex")}
	input := codexSSEInput(t, client, false)
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Add(context.Background(), input)
	if err == nil || result.Mutated {
		t.Fatalf("SSE-only add=%+v err=%v", result, err)
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unsupported add wrote state")
	}
	if _, err := os.Stat(client.ConfigRoot); !os.IsNotExist(err) {
		t.Fatalf("unsupported add wrote client root: %v", err)
	}
}

func TestCodexTransportTransientUpdateRetainsInstalledBytes(t *testing.T) {
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "codex")}
	input := codexSSEInput(t, client, true)
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	beforeState, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	before := codexTransportFiles(t, installed.Plan.ActivePath)
	input.Envelope.MCP.Servers["http"] = domain.MCPServer{Name: "http", Type: "stdio", Decoded: map[string]any{"type": "stdio", "command": "agentplugins-definitely-unavailable-runtime"}}
	input.InstallationID = installed.InstallationID
	input.OperationID = "codex-transient-update"
	result, err := service.Update(context.Background(), input)
	if err == nil || result.Mutated || len(result.Plan.Diagnostics) == 0 {
		t.Fatalf("transient update=%+v err=%v", result, err)
	}
	afterState, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeState, afterState) || !reflect.DeepEqual(before, codexTransportFiles(t, installed.Plan.ActivePath)) {
		t.Fatal("transient failure removed installed component")
	}
}

func codexTransportFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[relative] = string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
