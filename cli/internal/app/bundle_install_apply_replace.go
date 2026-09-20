package app

import (
	"fmt"
	"os"
)

func replaceInstalledBundle(tmp, dest string, force bool) error {
	exists, err := pathExists(dest)
	if err != nil {
		return err
	}
	if err := removeInstalledBundleDest(dest, exists, force); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func removeInstalledBundleDest(dest string, exists, force bool) error {
	if !exists {
		return nil
	}
	empty, err := pathEmpty(dest)
	if err != nil {
		return err
	}
	if !force && !empty {
		return fmt.Errorf("bundle install destination %s already exists and is not empty; use --force to overwrite", dest)
	}
	return os.RemoveAll(dest)
}
