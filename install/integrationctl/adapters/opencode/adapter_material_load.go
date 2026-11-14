package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/portablemcp"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) loadBaseSourceMaterial(ctx context.Context, sourceRoot string) (sourceMaterial, error) {
	material := newSourceMaterial()

	plugins, err := readPlugins(authoredpath.Join(sourceRoot, "targets", "opencode", "package.yaml"))
	if err != nil {
		return sourceMaterial{}, err
	}
	material.Plugins = plugins

	mcp, err := a.loadOpenCodePortableMCP(ctx, sourceRoot)
	if err != nil {
		return sourceMaterial{}, err
	}
	material.MCP = mcp
	return material, nil
}

func newSourceMaterial() sourceMaterial {
	return sourceMaterial{
		WholeFields: map[string]any{
			"$schema": "https://opencode.ai/config.json",
		},
	}
}

func (a Adapter) loadOpenCodePortableMCP(ctx context.Context, sourceRoot string) (map[string]any, error) {
	loader := portablemcp.Loader{FS: a.fs()}
	loaded, err := loader.LoadForTarget(ctx, sourceRoot, domain.TargetOpenCode)
	switch {
	case err == nil:
		projected := renderOpenCodeMCP(loaded, sourceRoot)
		if len(projected) == 0 {
			return nil, nil
		}
		return projected, nil
	case isMissingPortableMCP(err):
		return nil, nil
	default:
		return nil, err
	}
}

func (a Adapter) completeSourceMaterial(sourceRoot, scope string, workspaceRoot string, material sourceMaterial) (sourceMaterial, error) {
	if err := material.loadFirstClassDocs(sourceRoot); err != nil {
		return sourceMaterial{}, err
	}
	extra, err := readConfigExtra(authoredpath.Join(sourceRoot, "targets", "opencode", "config.extra.json"))
	if err != nil {
		return sourceMaterial{}, err
	}
	if err := material.mergeExtra(extra); err != nil {
		return sourceMaterial{}, err
	}
	copyFiles, err := collectCopyFiles(sourceRoot, a.assetsRoot(scope, workspaceRoot))
	if err != nil {
		return sourceMaterial{}, err
	}
	material.CopyFiles = copyFiles
	return material, nil
}
