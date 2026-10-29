package opencode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestOpenCodeConfigSelectionFollowsActualMCPEffects(t *testing.T) {
	cases := []struct {
		name                             string
		previousMCP, desiredMCP, refused bool
	}{
		{"skills only", false, false, false},
		{"new MCP", false, true, true},
		{"replace owned MCP", true, true, true},
		{"remove final owned MCP", true, false, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			jsonPath, jsoncPath := filepath.Join(root, "opencode.json"), filepath.Join(root, "opencode.jsonc")
			original := []byte("{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n")
			if err := os.WriteFile(jsoncPath, original, 0600); err != nil {
				t.Fatal(err)
			}
			projection := OpenCodeProjection{ConfigPath: jsonPath, ConfigJSON: jsonPath, ConfigJSONC: jsoncPath}
			skill := domain.NativeObjectOwnership{Kind: openCodeSkillKind, LogicalName: "guide", Path: filepath.Join(root, "skills", "guide")}
			previous, desired := []domain.NativeObjectOwnership{skill}, []domain.NativeObjectOwnership{skill}
			mcp := domain.NativeObjectOwnership{Kind: OpenCodeMCPObjectKind, LogicalName: "docs", Path: jsonPath}
			if test.previousMCP {
				previous = append(previous, mcp)
			}
			if test.desiredMCP {
				desired = append(desired, mcp)
				projection.MCPServers = map[string]nativeconfig.Server{"docs": {Type: "remote", URL: "https://docs.test"}}
			}
			err := confirmOpenCodeConfigSelection(root, projection, previous, desired)
			if (err != nil) != test.refused {
				t.Fatalf("config selection error %v; refused=%v", err, test.refused)
			}
			if body, err := os.ReadFile(jsoncPath); err != nil || string(body) != string(original) {
				t.Fatal("selection preflight changed client config", err)
			}
			if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
				t.Fatal("selection preflight created another config", err)
			}
		})
	}
}
