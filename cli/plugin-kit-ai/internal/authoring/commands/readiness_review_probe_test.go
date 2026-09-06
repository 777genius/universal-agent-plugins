package commands_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestReadinessReviewEmptyMCPName(t *testing.T) {
	for _, badName := range []string{"bad", ""} {
		t.Run("invalid-name="+badName, func(t *testing.T) {
			root, scratch := t.TempDir(), t.TempDir()
			write(t, root, "plugin.json", plugin(""))
			key, _ := json.Marshal(badName)
			write(t, root, "mcp.json", mcp(`{`+string(key)+`:{"type":"stdio","command":7},"good":{"type":"stdio","command":"node"}}`))
			app := commands.App{Projects: project.Service{Scratch: scratch}, Revision: "0e74767e09169f58145d56f5d3e10b4038171e3a"}
			args := []string{"compat", root, "--target=cursor", "--format=json"}
			r, code, raw := execute(t, app, args, false)
			_, mountedCode, mountedRaw := execute(t, app, args, true)
			if code != 1 || mountedCode != code || !bytes.Equal(raw, mountedRaw) {
				t.Fatal("entrypoint parity or failed conformance exit changed")
			}
			good := 0
			for _, c := range r.Components {
				if c.Status == report.Pass {
					good++
				}
			}
			if good != 1 {
				t.Fatalf("control lost its valid sibling: %+v", r.Components)
			}
			if len(r.Clients) != 1 {
				t.Fatal("expected Cursor compatibility")
			}
			summary, _ := json.Marshal(r.Clients[0].Components)
			t.Logf("badName=%q reportComponents=%d validSiblings=%d compatibility=%s conformance=%s exit=%d decisions=%s", badName, len(r.Components), good, r.Compatibility.Status, r.Conformance.Status, code, summary)
			if len(r.Clients[0].Components) != 2 {
				t.Errorf("invalid MCP entry vanished: got %d compatibility entries; want 2", len(r.Clients[0].Components))
			}
			supported := 0
			for _, c := range r.Clients[0].Components {
				if c.Kind == domain.ComponentMCPServer && c.Support == domain.SupportNative {
					supported++
				}
			}
			if supported != 1 {
				t.Errorf("invalid sibling poisoned valid server support: got %d native MCP decisions; want 1", supported)
			}
			if len(tree(t, scratch)) != 0 {
				t.Fatal("scratch residue")
			}
		})
	}
}

func TestReadinessReviewMCPBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, body          string
		code, count, native int
	}{
		{"empty-invalid", mcp(`{"":{"type":"stdio","command":7},"good":{"type":"stdio","command":"node"}}`), 1, 2, 1},
		{"ordinary-invalid", mcp(`{"bad":{"type":"stdio","command":7},"good":{"type":"stdio","command":"node"}}`), 1, 2, 1},
		{"empty-valid", mcp(`{"":{"type":"stdio","command":"node"},"good":{"type":"stdio","command":"node"}}`), 0, 2, 2},
		{"document-invalid", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","unexpected":true,"mcpServers":{"good":{"type":"stdio","command":"node"}}}`, 1, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, scratch := t.TempDir(), t.TempDir()
			write(t, root, "plugin.json", plugin(""))
			write(t, root, "mcp.json", tc.body)
			app := commands.App{Projects: project.Service{Scratch: scratch}}
			for _, command := range []string{"compat", "inspect"} {
				args := []string{command, root, "--target=cursor", "--format=json"}
				r, code, raw := execute(t, app, args, false)
				_, mountedCode, mountedRaw := execute(t, app, args, true)
				if code != tc.code || mountedCode != code || !bytes.Equal(raw, mountedRaw) {
					t.Fatal("exit or mount parity")
				}
				if len(r.Clients) != 1 || len(r.Clients[0].Components) != tc.count {
					t.Fatalf("inventory: %+v", r.Clients)
				}
				native := 0
				for i, c := range r.Clients[0].Components {
					if c.Index != i+1 || c.Kind != domain.ComponentMCPServer {
						t.Fatal("opaque ordering")
					}
					if c.Support == domain.SupportNative {
						native++
					}
				}
				if native != tc.native {
					t.Fatalf("native=%d want=%d", native, tc.native)
				}
				want := report.Pass
				if tc.code == 1 {
					want = report.Fail
				}
				if r.Conformance.Status != want {
					t.Fatal("conformance changed")
				}
				if tc.count == 2 && r.Compatibility.Status != want {
					t.Fatal("compatibility aggregate changed")
				}
				if bytes.Contains(raw, []byte(`"good"`)) || bytes.Contains(raw, []byte(`"bad"`)) || bytes.Contains(raw, []byte(root)) {
					t.Fatal("raw identity disclosure")
				}
			}
			if len(tree(t, scratch)) != 0 {
				t.Fatal("scratch residue")
			}
		})
	}
}
