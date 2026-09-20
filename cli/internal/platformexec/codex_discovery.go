package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (codexPackageAdapter) DetectNative(root string) bool {
	return fileExists(filepath.Join(root, ".codex-plugin", "plugin.json"))
}

func (codexRuntimeAdapter) DetectNative(root string) bool {
	return fileExists(filepath.Join(root, ".codex", "config.toml"))
}

func (codexPackageAdapter) RefineDiscovery(root string, state *pluginmodel.TargetState) error {
	if rel := state.DocPath("package_metadata"); strings.TrimSpace(rel) != "" {
		if _, ok, err := readYAMLDoc[codexPackageMeta](root, rel); err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		} else if !ok {
			return nil
		}
	}
	if rel := state.DocPath("interface"); strings.TrimSpace(rel) != "" {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return err
		}
		if _, err := codexmanifest.ParseInterfaceDoc(body); err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
	}
	if rel := state.DocPath("app_manifest"); strings.TrimSpace(rel) != "" {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return err
		}
		if _, err := codexmanifest.ParseAppManifestDoc(body); err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
	}
	return nil
}

func (codexRuntimeAdapter) RefineDiscovery(root string, state *pluginmodel.TargetState) error {
	if rel := state.DocPath("package_metadata"); strings.TrimSpace(rel) != "" {
		if _, ok, err := readYAMLDoc[codexRuntimeMeta](root, rel); err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		} else if !ok {
			return nil
		}
	}
	return nil
}

func (codexPackageAdapter) ManagedPaths(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]string, error) {
	return nil, nil
}

func (codexRuntimeAdapter) ManagedPaths(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]string, error) {
	return nil, nil
}
