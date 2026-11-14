package portablemcp

import (
	"context"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (l Loader) readPortableMCP(ctx context.Context, root string) (string, []byte, error) {
	candidates := []string{authoredpath.Join(root, "mcp", "servers.yaml")}
	fallback := filepath.Join(root, "mcp", "servers.yaml")
	if fallback != candidates[0] {
		candidates = append(candidates, fallback)
	}
	for _, path := range candidates {
		body, err := l.FS.ReadFile(ctx, path)
		if err == nil {
			return path, body, nil
		}
	}
	return "", nil, domain.NewError(domain.ErrManifestLoad, "portable MCP file not found", nil)
}
