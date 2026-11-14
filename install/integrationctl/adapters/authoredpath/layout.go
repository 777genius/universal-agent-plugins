package authoredpath

import (
	"os"
	"path/filepath"
)

type layout struct {
	manifestPath string
	root         string
}

func ManifestCandidates(root string) []string {
	return []string{
		filepath.Join(root, "plugin.yaml"),
		filepath.Join(root, "plugin", "plugin.yaml"),
		filepath.Join(root, "src", "plugin.yaml"),
	}
}

func Discover(root string) (manifestPath string, authoredRoot string, ok bool) {
	layouts := []layout{
		{manifestPath: filepath.Join(root, "plugin.yaml"), root: root},
		{manifestPath: filepath.Join(root, "plugin", "plugin.yaml"), root: filepath.Join(root, "plugin")},
		{manifestPath: filepath.Join(root, "src", "plugin.yaml"), root: filepath.Join(root, "src")},
	}
	for _, candidate := range layouts {
		if fileExists(candidate.manifestPath) {
			return candidate.manifestPath, candidate.root, true
		}
	}
	return "", "", false
}

func HasManifest(root string) bool {
	_, _, ok := Discover(root)
	return ok
}

func Dir(root string) string {
	_, authoredRoot, ok := Discover(root)
	if ok {
		return authoredRoot
	}
	return filepath.Join(root, "plugin")
}

func Join(root string, elems ...string) string {
	parts := append([]string{Dir(root)}, elems...)
	return filepath.Join(parts...)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
