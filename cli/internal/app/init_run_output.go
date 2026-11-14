package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveInitOutputDir(projectName, outputDir string) (string, error) {
	out := strings.TrimSpace(outputDir)
	if out == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		return filepath.Join(wd, projectName), nil
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return "", fmt.Errorf("resolve output path: %w", err)
	}
	return abs, nil
}
