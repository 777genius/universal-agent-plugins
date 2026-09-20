package platformexec

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func authoredComponentDir(state pluginmodel.TargetState, kind, fallback string) string {
	paths := state.ComponentPaths(kind)
	if len(paths) == 0 {
		return filepath.ToSlash(fallback)
	}
	dir := filepath.ToSlash(filepath.Dir(paths[0]))
	if dir == "." {
		return filepath.ToSlash(fallback)
	}
	return dir
}

func managedKeysForNativeDoc(target, kind string) []string {
	profile, ok := platformmeta.Lookup(target)
	if !ok {
		return nil
	}
	for _, doc := range profile.NativeDocs {
		if doc.Kind != kind {
			continue
		}
		if len(doc.ManagedKeys) == 0 {
			return nil
		}
		return append([]string(nil), doc.ManagedKeys...)
	}
	return nil
}

func renderPortableMCPForTarget(mcp *pluginmodel.PortableMCP, target string) (map[string]any, error) {
	if mcp == nil {
		return nil, nil
	}
	return mcp.RenderForTarget(target)
}

func importedPortableMCPArtifact(sourceTarget string, servers map[string]any) (pluginmodel.Artifact, error) {
	body, err := pluginmodel.ImportedPortableMCPYAML(sourceTarget, servers)
	if err != nil {
		return pluginmodel.Artifact{}, err
	}
	return pluginmodel.Artifact{
		RelPath: filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "mcp", "servers.yaml")),
		Content: body,
	}, nil
}
