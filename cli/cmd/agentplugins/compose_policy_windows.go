package main

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func machinePolicyPath() (string, error) {
	root, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
	if err != nil {
		return "", fmt.Errorf("locate machine installation policy: %w", err)
	}
	return filepath.Join(root, "AgentPlugins", "policy.yaml"), nil
}
