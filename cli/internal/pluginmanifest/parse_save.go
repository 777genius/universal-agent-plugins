package pluginmanifest

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func saveManifest(root string, manifest Manifest, force bool) error {
	layout, err := detectAuthoredLayout(root)
	if err != nil {
		return err
	}
	normalizeManifest(&manifest)
	if err := manifest.Validate(); err != nil {
		return err
	}
	full := filepath.Join(root, layout.Path(FileName))
	if _, err := os.Stat(full); err == nil && !force {
		return fmt.Errorf("refusing to overwrite existing file %s (use --force)", FileName)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	body, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal plugin.yaml: %w", err)
	}
	return os.WriteFile(full, body, 0o644)
}

func saveLauncher(root string, launcher Launcher, force bool) error {
	layout, err := detectAuthoredLayout(root)
	if err != nil {
		return err
	}
	normalizeLauncher(&launcher)
	if err := launcher.Validate(); err != nil {
		return err
	}
	full := filepath.Join(root, layout.Path(LauncherFileName))
	if _, err := os.Stat(full); err == nil && !force {
		return fmt.Errorf("refusing to overwrite existing file %s (use --force)", LauncherFileName)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	body, err := yaml.Marshal(launcher)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", LauncherFileName, err)
	}
	return os.WriteFile(full, body, 0o644)
}

func normalizePackage(root string, force bool) ([]Warning, error) {
	manifest, warnings, err := loadManifestWithWarnings(root)
	if err != nil {
		return nil, err
	}
	if err := saveManifest(root, manifest, force); err != nil {
		return warnings, err
	}
	if launcher, err := loadLauncher(root); err == nil {
		if err := saveLauncher(root, launcher, force); err != nil {
			return warnings, err
		}
	}
	return warnings, nil
}

func saveManifestWithLayout(root string, layout authoredLayout, manifest Manifest, force bool) error {
	normalizeManifest(&manifest)
	if err := manifest.Validate(); err != nil {
		return err
	}
	full := filepath.Join(root, layout.Path(FileName))
	if _, err := os.Stat(full); err == nil && !force {
		return fmt.Errorf("refusing to overwrite existing file %s (use --force)", layout.Path(FileName))
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	body, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", layout.Path(FileName), err)
	}
	return os.WriteFile(full, body, 0o644)
}

func saveLauncherWithLayout(root string, layout authoredLayout, launcher Launcher, force bool) error {
	normalizeLauncher(&launcher)
	if err := launcher.Validate(); err != nil {
		return err
	}
	full := filepath.Join(root, layout.Path(LauncherFileName))
	if _, err := os.Stat(full); err == nil && !force {
		return fmt.Errorf("refusing to overwrite existing file %s (use --force)", layout.Path(LauncherFileName))
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	body, err := yaml.Marshal(launcher)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", layout.Path(LauncherFileName), err)
	}
	return os.WriteFile(full, body, 0o644)
}
