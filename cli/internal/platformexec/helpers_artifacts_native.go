package platformexec

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

// authoredComponentDir returns the authored directory whose tree maps onto the
// generated surface for kind. fallback names the targets/<target>/<kind>
// suffix that tree always keeps, so the mirror root is clamped to the suffix
// instead of the first path's parent: deriving it from one path alone flattens
// namespaces when every authored file lives in nested subdirectories
// (targets/qoder/commands/ship/run.md would mirror to commands/run.md).
func authoredComponentDir(state pluginmodel.TargetState, kind, fallback string) string {
	paths := state.ComponentPaths(kind)
	fallback = filepath.ToSlash(fallback)
	if len(paths) == 0 {
		return fallback
	}
	dir := filepath.ToSlash(filepath.Dir(paths[0]))
	if dir == "." {
		return fallback
	}
	if root, ok := trimAfterPathSuffix(dir, fallback); ok {
		return root
	}
	return dir
}

// trimAfterPathSuffix cuts dir back to the first segment-aligned occurrence of
// suffix, keeping every namespace below the authored component root intact.
func trimAfterPathSuffix(dir, suffix string) (string, bool) {
	for i := 0; i+len(suffix) <= len(dir); i++ {
		if (i > 0 && dir[i-1] != '/') || !strings.HasPrefix(dir[i:], suffix) {
			continue
		}
		if end := i + len(suffix); end < len(dir) && dir[end] != '/' {
			continue
		}
		return dir[:i+len(suffix)], true
	}
	return "", false
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
