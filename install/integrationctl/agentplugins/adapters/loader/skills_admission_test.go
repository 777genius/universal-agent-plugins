package loader

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSkillAdmissionKeepsHealthyComponents(t *testing.T) {
	for _, extra := range []string{"metadata: {count: 5}\n", "metadata: {nested: {value: x}}\n", "compatibility: ''\n"} {
		t.Run(extra, func(t *testing.T) {
			root := t.TempDir()
			writeMinimalPlugin(t, root, "mixed")
			writeLoaderFile(t, filepath.Join(root, "skills", "bad", "SKILL.md"), "---\nname: bad\ndescription: Invalid\n"+extra+"---\n")
			writeLoaderFile(t, filepath.Join(root, "skills", "good", "SKILL.md"), "---\nname: good\ndescription: Healthy\n---\n")
			writeLoaderFile(t, filepath.Join(root, "mcp.json"), `{"$schema":"`+domain.MCPSchemaV1+`","mcpServers":{"remote":{"type":"streamable-http","url":"https://example.test/mcp"}}}`)
			pkg, err := testLoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
			if err != nil {
				t.Fatal(err)
			}
			if len(pkg.Skills) != 1 || pkg.Skills["good"].Name != "good" || len(pkg.MCP.Servers) != 1 || pkg.MCP.Servers["remote"].Type != "streamable-http" {
				t.Fatalf("healthy components lost: %+v", pkg)
			}
			if len(pkg.Inventory.InvalidSkills) != 1 || pkg.Inventory.InvalidSkills[0] != "bad" {
				t.Fatalf("invalid inventory = %+v", pkg.Inventory)
			}
			for _, d := range pkg.Diagnostics {
				if d.Code == "skill_invalid" && d.Boundary == domain.BoundarySkill && d.Item == "bad" {
					return
				}
			}
			t.Fatal("missing skill-boundary diagnostic for bad")
		})
	}
}
