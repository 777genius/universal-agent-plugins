package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"github.com/tailscale/hujson"
)

func applyConfigMutation(state configPatchState, mutation configMutation) error {
	removeConfigKeys(state.obj, mutation.WholeRemove)

	mergedPlugins := mergePluginRefs(state.currentPlugins, mutation.PluginsRemove, mutation.PluginsSet)
	if err := applyConfigPlugins(state.obj, mergedPlugins); err != nil {
		return domain.NewError(domain.ErrMutationApply, "patch OpenCode plugin refs", err)
	}
	mergedMCP := mergeNamedObject(state.currentMCP, mutation.MCPRemove, mutation.MCPSet)
	if err := applyConfigMCP(state.obj, mergedMCP); err != nil {
		return domain.NewError(domain.ErrMutationApply, "patch OpenCode MCP config", err)
	}
	for key, value := range mutation.WholeSet {
		if err := setTopLevelMember(state.obj, key, value); err != nil {
			return domain.NewError(domain.ErrMutationApply, "patch OpenCode config", err)
		}
	}
	return nil
}

func removeConfigKeys(obj *hujson.Object, keys []string) {
	for _, key := range keys {
		if key == "" || key == "$schema" {
			continue
		}
		removeTopLevelMember(obj, key)
	}
}

func applyConfigPlugins(obj *hujson.Object, plugins []pluginRef) error {
	if len(plugins) == 0 {
		removeTopLevelMember(obj, "plugin")
		return nil
	}
	return setTopLevelMember(obj, "plugin", pluginRefsToJSON(plugins))
}

func applyConfigMCP(obj *hujson.Object, mcp map[string]any) error {
	if len(mcp) == 0 {
		removeTopLevelMember(obj, "mcp")
		return nil
	}
	return setTopLevelMember(obj, "mcp", mcp)
}

func (a Adapter) writeConfigPatch(ctx context.Context, path string, state configPatchState, mutation configMutation) (configPatchResult, error) {
	if len(state.obj.Members) == 0 {
		if err := a.fs().Remove(ctx, path); err != nil {
			return configPatchResult{}, domain.NewError(domain.ErrMutationApply, "remove empty OpenCode config", err)
		}
		return configPatchResult{ConfigPath: path}, nil
	}

	rendered := state.ast.Pack()
	if _, err := a.mutator().MutateFile(ctx, ports.SafeFileMutationInput{
		Path: path,
		Mode: 0o644,
		Build: func(_ []byte, _ bool) ([]byte, error) {
			return rendered, nil
		},
		ValidateBefore: func(next []byte) error {
			_, err := hujson.Parse(next)
			return err
		},
		ValidateAfter: func(_ context.Context, path string, _ []byte) error {
			body, err := a.fs().ReadFile(ctx, path)
			if err != nil {
				return err
			}
			_, err = hujson.Parse(body)
			return err
		},
	}); err != nil {
		return configPatchResult{}, domain.NewError(domain.ErrMutationApply, "write OpenCode config", err)
	}
	return configPatchResult{
		Body:            rendered,
		ConfigPath:      path,
		ManagedKeys:     sortedManagedKeys(mutation.WholeSet),
		OwnedPluginRefs: pluginRefNames(mutation.PluginsSet),
		OwnedMCPAliases: sortedMapKeys(mutation.MCPSet),
	}, nil
}
