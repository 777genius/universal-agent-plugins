package pluginmanifest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

func newPortableComponents() PortableComponents {
	return pluginmodel.NewPortableComponents()
}

func newTargetComponents(target string) TargetComponents {
	return pluginmodel.NewTargetState(target)
}

func discoveredNativeDocPaths(tc TargetComponents) map[string]string {
	if len(tc.Docs) == 0 {
		return nil
	}
	out := make(map[string]string, len(tc.Docs))
	for kind, path := range tc.Docs {
		if strings.TrimSpace(path) == "" {
			continue
		}
		out[kind] = path
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneStringMap(items map[string]string) map[string]string {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]string, len(items))
	for key, value := range items {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneStringSlice(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	return append([]string{}, items...)
}

func normalizeManifest(m *Manifest) {
	pluginmodel.NormalizeManifest(m)
}

func normalizeLauncher(l *Launcher) {
	pluginmodel.NormalizeLauncher(l)
}

func normalizeTarget(target string) string {
	return pluginmodel.NormalizeTarget(target)
}

func normalizeRuntime(runtime string) string {
	return pluginmodel.NormalizeRuntime(runtime)
}

func defaultName(root string) string {
	name := filepath.Base(filepath.Clean(root))
	if err := scaffold.ValidateProjectName(name); err == nil {
		return name
	}
	return "plugin"
}

func discoverPublication(root string, layout authoredLayout) (publishschema.State, error) {
	return publishschema.DiscoverInLayout(root, layout.Path(""))
}

func addSourceFiles(set map[string]struct{}, files []string) {
	for _, rel := range files {
		set[rel] = struct{}{}
	}
}

func setOf(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, filepath.ToSlash(key))
	}
	slices.Sort(out)
	return out
}

func appendWarning(seen map[string]struct{}, warnings *[]Warning, warning Warning) {
	key := string(warning.Kind) + ":" + warning.Path
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*warnings = append(*warnings, warning)
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func addManagedCopies(set map[string]struct{}, files []string, srcDir, dstRoot string) {
	for _, rel := range files {
		relPath, err := filepath.Rel(filepath.ToSlash(srcDir), rel)
		if err != nil {
			continue
		}
		set[filepath.ToSlash(filepath.Join(dstRoot, relPath))] = struct{}{}
	}
}

func cleanRelativeRef(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	path = strings.TrimPrefix(path, "./")
	if path == "." {
		return ""
	}
	return path
}
