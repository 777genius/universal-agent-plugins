package pluginmanifest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sourceresolver "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/source"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var errCanonicalSourceImport = errors.New("canonical package sources are already in package-standard layout; clone or sync the repo directly instead of import --source")

func materializePreparedImport(prepared preparedImport) (string, func(), error) {
	root, err := os.MkdirTemp("", "plugin-kit-ai-source-*")
	if err != nil {
		return "", nil, err
	}
	if err := writePreparedImport(root, prepared, true); err != nil {
		_ = os.RemoveAll(root)
		return "", nil, err
	}
	return root, func() { _ = os.RemoveAll(root) }, nil
}

func resolveSourceRef(sourceRef string) (ports.ResolvedSource, func(), error) {
	sourceRef = strings.TrimSpace(sourceRef)
	if sourceRef == "" {
		return ports.ResolvedSource{}, nil, fmt.Errorf("source is required")
	}
	resolved, err := sourceresolver.Resolver{}.Resolve(context.Background(), domain.IntegrationRef{Raw: sourceRef})
	if err != nil {
		return ports.ResolvedSource{}, nil, err
	}
	return resolved, func() {
		if resolved.Cleanup != nil {
			_ = resolved.Cleanup()
		}
	}, nil
}

func isPackageStandardSource(root string) bool {
	layout, err := detectAuthoredLayout(root)
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(root, layout.Path(FileName)))
	return err == nil && !info.IsDir()
}
