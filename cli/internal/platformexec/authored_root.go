package platformexec

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func authoredRootHint(state pluginmodel.TargetState, portable pluginmodel.PortableComponents) string {
	for _, path := range state.Docs {
		if root := authoredRootFromPath(path); root != "" {
			return root
		}
	}
	for _, paths := range state.Components {
		for _, path := range paths {
			if root := authoredRootFromPath(path); root != "" {
				return root
			}
		}
	}
	for _, paths := range portable.Items {
		for _, path := range paths {
			if root := authoredRootFromPath(path); root != "" {
				return root
			}
		}
	}
	if portable.MCP != nil {
		if root := authoredRootFromPath(portable.MCP.Path); root != "" {
			return root
		}
	}
	return pluginmodel.SourceDirName
}

func authoredRootFromPath(path string) string {
	path = strings.TrimSpace(path)
	switch {
	case strings.HasPrefix(path, pluginmodel.SourceDirName+"/"), path == pluginmodel.SourceDirName:
		return pluginmodel.SourceDirName
	default:
		return ""
	}
}
