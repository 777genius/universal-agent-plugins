package pluginmanifest

import (
	"os"
	"path/filepath"
)

func loadManifest(root string) (Manifest, error) {
	manifest, _, err := loadManifestWithWarnings(root)
	return manifest, err
}

func loadLauncher(root string) (Launcher, error) {
	launcher, _, err := loadLauncherWithWarnings(root)
	return launcher, err
}

func loadManifestWithWarnings(root string) (Manifest, []Warning, error) {
	layout, err := detectAuthoredLayout(root)
	if err != nil {
		return Manifest{}, nil, err
	}
	body, err := os.ReadFile(filepath.Join(root, layout.Path(FileName)))
	if err != nil {
		return Manifest{}, nil, err
	}
	return analyzeManifest(body)
}

func loadLauncherWithWarnings(root string) (Launcher, []Warning, error) {
	layout, err := detectAuthoredLayout(root)
	if err != nil {
		return Launcher{}, nil, err
	}
	body, err := os.ReadFile(filepath.Join(root, layout.Path(LauncherFileName)))
	if err != nil {
		return Launcher{}, nil, err
	}
	return analyzeLauncher(body)
}

func parseManifest(body []byte) (Manifest, error) {
	manifest, _, err := analyzeManifest(body)
	return manifest, err
}

func parseLauncher(body []byte) (Launcher, error) {
	launcher, _, err := analyzeLauncher(body)
	return launcher, err
}
