package opencode

import (
	"context"
	"errors"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/tailscale/hujson"
)

type configPatchState struct {
	ast            hujson.Value
	obj            *hujson.Object
	currentPlugins map[string]pluginRef
	currentMCP     map[string]any
}

func (a Adapter) loadConfigPatchState(ctx context.Context, path string) (configPatchState, error) {
	body, err := a.fs().ReadFile(ctx, path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return configPatchState{}, domain.NewError(domain.ErrMutationApply, "read OpenCode config", err)
	}
	if errors.Is(err, os.ErrNotExist) {
		body = []byte("{}\n")
	}
	ast, err := hujson.Parse(body)
	if err != nil {
		return configPatchState{}, domain.NewError(domain.ErrMutationApply, "parse OpenCode config", err)
	}
	obj, ok := ast.Value.(*hujson.Object)
	if !ok {
		return configPatchState{}, domain.NewError(domain.ErrMutationApply, "OpenCode config root must be an object", nil)
	}
	doc, err := decodeConfigMap(body)
	if err != nil {
		return configPatchState{}, domain.NewError(domain.ErrMutationApply, "decode OpenCode config", err)
	}
	currentPlugins, err := existingPluginRefs(doc["plugin"])
	if err != nil {
		return configPatchState{}, domain.NewError(domain.ErrMutationApply, "parse OpenCode plugin refs", err)
	}
	currentMCP, err := existingObjectMap(doc["mcp"], "mcp")
	if err != nil {
		return configPatchState{}, domain.NewError(domain.ErrMutationApply, "parse OpenCode MCP config", err)
	}
	return configPatchState{
		ast:            ast,
		obj:            obj,
		currentPlugins: currentPlugins,
		currentMCP:     currentMCP,
	}, nil
}
