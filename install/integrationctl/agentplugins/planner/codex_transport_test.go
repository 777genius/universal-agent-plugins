package planner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestCodexTransportSelectionPreservesSiblings(t *testing.T) {
	for _, onlySSE := range []bool{false, true} {
		t.Run(map[bool]string{false: "mixed", true: "sse-only"}[onlySSE], func(t *testing.T) {
			root := t.TempDir()
			e := testEnvelope()
			e.MCP.Servers = map[string]domain.MCPServer{"events": {Type: "sse"}}
			if onlySSE {
				e.Skills = nil
			} else {
				e.MCP.Servers["http"] = domain.MCPServer{Type: "streamable-http"}
				e.MCP.Servers["stdio"] = domain.MCPServer{Type: "stdio"}
			}
			plan, err := (Planner{ManagedRoot: filepath.Join(root, "managed")}).Plan(context.Background(), e, detectedClient(domain.ClientCodex, filepath.Join(root, "config")), domain.ScopeUser, "demo-0123456789ab")
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			if !onlySSE {
				want = []string{"http", "stdio"}
			}
			if got := domain.SelectedMCPNames(plan); len(got) != len(want) || (len(want) > 0 && !reflect.DeepEqual(got, want)) {
				t.Fatalf("selection=%v", got)
			}
			for _, c := range plan.Components {
				if c.Name == "events" && (c.Support != domain.SupportUnsupported || c.Reason != "declared_sse_not_supported_by_client") {
					t.Fatalf("SSE decision=%+v", c)
				}
			}
			if (plan.Status == domain.PlanUnsupported) != onlySSE {
				t.Fatalf("status=%s", plan.Status)
			}
			reports, err := Compatibility(e, []domain.ClientID{domain.ClientCodex})
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range reports[0].Components {
				if c.Kind == domain.ComponentMCPServer && c.Index == 1 {
					if c.Support != domain.SupportUnsupported || !compatContains(c.Limitations, "declared_sse_not_supported_by_client") {
						t.Fatalf("SSE compatibility=%+v", c)
					}
				} else if c.Kind == domain.ComponentMCPServer || c.Kind == domain.ComponentSkill {
					if c.Support != domain.SupportProjected {
						t.Fatalf("healthy compatibility=%+v", c)
					}
				}
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("planning wrote files: %v %v", entries, err)
			}
		})
	}
}
