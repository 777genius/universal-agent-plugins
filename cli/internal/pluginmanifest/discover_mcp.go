package pluginmanifest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func discoverMCP(root string, layout authoredLayout) (*PortableMCP, bool, error) {
	for _, legacyRel := range []string{"mcp/servers.json", "mcp/servers.yml"} {
		if fileExists(filepath.Join(root, layout.Path(legacyRel))) {
			return nil, false, fmt.Errorf("unsupported portable MCP authored path %s: use %s/mcp/servers.yaml", legacyRel, pluginmodel.SourceDirName)
		}
	}
	for _, rel := range []string{"mcp/servers.yaml"} {
		authoredRel := layout.Path(rel)
		full := filepath.Join(root, authoredRel)
		body, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		parsed, err := pluginmodel.ParsePortableMCP(authoredRel, body)
		if err != nil {
			return nil, false, err
		}
		return &PortableMCP{Path: authoredRel, Servers: parsed.Servers, File: parsed.File}, true, nil
	}
	return nil, false, nil
}
