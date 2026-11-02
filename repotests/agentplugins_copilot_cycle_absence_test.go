//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"os"
	"path/filepath"
)

func copilotCycleAbsent(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("Copilot absence path is noncanonical")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("Copilot exact owned absence required")
	}
	for parent, depth := filepath.Dir(path), 0; ; parent, depth = filepath.Dir(parent), depth+1 {
		if depth > 128 {
			return fmt.Errorf("Copilot absence ancestor bound")
		}
		info, err := os.Lstat(parent)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return fmt.Errorf("Copilot absent path has an alias/non-directory ancestor")
		}
		if filepath.Dir(parent) == parent {
			return nil
		}
	}
}
