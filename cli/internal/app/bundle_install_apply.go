package app

import (
	"path/filepath"
)

func installBundleArchive(archivePath, dest string, force bool) (string, error) {
	dest = filepath.Clean(dest)
	tmp, cleanup, err := prepareBundleInstallWorkspace(dest)
	if err != nil {
		return "", err
	}
	success := false
	defer func() {
		if !success {
			cleanup()
		}
	}()

	if err := extractBundleArchiveToDir(archivePath, tmp); err != nil {
		return "", err
	}
	if err := replaceInstalledBundle(tmp, dest, force); err != nil {
		return "", err
	}
	success = true
	return dest, nil
}
