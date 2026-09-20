package runtimecheck

import (
	"path/filepath"
	"strconv"
	"strings"
)

func YarnBerry(root string, packageManager string) bool {
	if fileExists(filepath.Join(root, ".yarnrc.yml")) {
		return true
	}
	if !strings.HasPrefix(packageManager, "yarn@") {
		return false
	}
	version := strings.TrimPrefix(packageManager, "yarn@")
	majorText := version
	if idx := strings.Index(majorText, "."); idx >= 0 {
		majorText = majorText[:idx]
	}
	major, err := strconv.Atoi(majorText)
	return err == nil && major >= 2
}
