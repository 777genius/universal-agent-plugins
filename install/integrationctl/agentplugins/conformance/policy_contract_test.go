package conformance

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestPolicyFixtureClassification(t *testing.T) {
	body, e := os.ReadFile("testdata/policy-v1/index.json")
	if e != nil {
		t.Fatal(e)
	}
	var index struct {
		Profile  string
		Fixtures []struct {
			ID, Kind, Body, Code string
			State                InputState
			Layer                PolicyLayer
			Boundary             domain.FailureBoundary
			Outcome              Outcome
		}
		OtherEvidence map[string]string `json:"other_evidence"`
	}
	if e = json.Unmarshal(body, &index); e != nil {
		t.Fatal(e)
	}
	if index.Profile != "author-document-bounds/v1" || len(index.Fixtures) < 30 {
		t.Fatal("fixture profile incomplete")
	}
	d := decoder(t)
	seen := map[string]bool{}
	for _, tc := range index.Fixtures {
		t.Run(tc.ID, func(t *testing.T) {
			if seen[tc.ID] {
				t.Fatal("duplicate fixture id")
			}
			seen[tc.ID] = true
			i := minimal()
			state := tc.State
			if state == "" {
				state = Present
			}
			switch tc.Kind {
			case "plugin":
				i.Plugin = NewDocument("plugin.json", state, []byte(tc.Body))
			case "plugin-fields":
				i.Plugin = NewDocument("plugin.json", state, []byte(`{"$schema":"`+domain.PluginSchemaV1+`",`+tc.Body+`}`))
			case "mcp":
				i.MCP = NewDocument("mcp.json", state, []byte(tc.Body))
			case "servers":
				i.MCP = NewDocument("mcp.json", state, []byte(mcpBody(tc.Body)))
			case "server":
				i.MCP = NewDocument("mcp.json", state, []byte(mcpBody(`{"bad":`+tc.Body+`,"good":{"type":"stdio","command":"node"}}`)))
			case "skill":
				i.SkillsRoot = Present
				i.Skills = []SkillInput{{"bad", NewDocument("skills/bad/SKILL.md", state, []byte(tc.Body))}, skillInput("good", "")}
			case "skills-root":
				i.SkillsRoot = state
			default:
				t.Fatal("unknown fixture kind")
			}
			if tc.Kind == "mcp" || tc.Kind == "servers" || tc.Kind == "server" {
				i.SkillsRoot = Present
				i.Skills = []SkillInput{skillInput("good", "")}
			}
			if tc.Kind == "skill" || tc.Kind == "skills-root" {
				i.MCP = NewDocument("mcp.json", Present, []byte(mcpBody(`{"good":{"type":"stdio","command":"node"}}`)))
			}
			f, e := d.Decode(context.Background(), i)
			if e != nil || f.Conformance != tc.Outcome || !hasFinding(f, tc.Code, tc.Layer, tc.Boundary) {
				t.Fatalf("got %s %v %+v", f.Conformance, e, f.Findings)
			}
			if tc.Kind == "server" && len(f.Package.MCP.Servers) != 1 {
				t.Fatal("server sibling erased")
			}
			if (tc.Kind == "mcp" || tc.Kind == "servers" || tc.Kind == "server") && len(f.Package.Skills) != 1 {
				t.Fatal("Skill sibling erased")
			}
			if (tc.Kind == "skill" || tc.Kind == "skills-root") && len(f.Package.MCP.Servers) != 1 {
				t.Fatal("MCP sibling erased")
			}
		})
	}
	if index.OtherEvidence["DIG-001..027,DIG-ID-001"] == "" || index.OtherEvidence["ACQ-001..024"] == "" {
		t.Fatal("untouched ownership ledger missing")
	}
}
