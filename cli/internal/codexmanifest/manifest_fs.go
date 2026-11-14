package codexmanifest

import (
	"os"
	"path/filepath"
	"strings"
)

func ValidatePluginDirLayout(root string) error {
	entries, err := os.ReadDir(filepath.Join(root, PluginDir))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == PluginFileName {
			continue
		}
		return &PluginDirLayoutError{Path: filepath.ToSlash(filepath.Join(PluginDir, entry.Name()))}
	}
	return nil
}

func ReadImportedPluginManifest(root string) (ImportedPluginManifest, []byte, error) {
	body, err := os.ReadFile(filepath.Join(root, PluginDir, PluginFileName))
	if err != nil {
		return ImportedPluginManifest{}, nil, err
	}
	if err := ValidatePluginDirLayout(root); err != nil {
		return ImportedPluginManifest{}, nil, err
	}
	out, err := DecodeImportedPluginManifest(body)
	if err != nil {
		return ImportedPluginManifest{}, nil, err
	}
	return out, body, nil
}

func UnexpectedBundleSidecars(root string, manifest ImportedPluginManifest) []string {
	var paths []string
	if strings.TrimSpace(manifest.AppsRef) == "" && fileExists(filepath.Join(root, AppFileName)) {
		paths = append(paths, AppManifestPath())
	}
	if strings.TrimSpace(manifest.MCPServersRef) == "" && fileExists(filepath.Join(root, MCPFileName)) {
		paths = append(paths, MCPManifestPath())
	}
	return paths
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
