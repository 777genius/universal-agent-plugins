package app

import (
	"fmt"
	"strings"
)

type exportServiceInput struct {
	root     string
	platform string
}

func resolveExportServiceInput(opts PluginExportOptions) (exportServiceInput, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}
	platform := strings.TrimSpace(opts.Platform)
	if platform == "" {
		return exportServiceInput{}, fmt.Errorf("export requires --platform")
	}
	return exportServiceInput{root: root, platform: platform}, nil
}
