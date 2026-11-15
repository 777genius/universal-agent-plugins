package managedstdio

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

func resolveRuntimeCommand(plugin, command string) (string, error) {
	if strings.HasPrefix(command, "./") {
		relative, err := pathcontract.ParseCommand(command)
		if err != nil {
			return "", err
		}
		result := pathcontract.Resolve(plugin, relative)
		if result.State != pathcontract.Resolved {
			return "", fmt.Errorf("bundled command unavailable: %w", result.Err)
		}
		command = result.Path
	} else {
		if command == "" || strings.ContainsAny(command, "/\\\x00") {
			return "", fmt.Errorf("command must be a bare executable or bundled path")
		}
		// Resolve PATH before chdir. Go rejects implicit current-directory lookup.
		var err error
		command, err = exec.LookPath(command)
		if err != nil {
			return "", fmt.Errorf("runtime unavailable: %w", err)
		}
		if !filepath.IsAbs(command) {
			return "", fmt.Errorf("runtime PATH resolved to a relative executable")
		}
	}
	return command, nil
}
