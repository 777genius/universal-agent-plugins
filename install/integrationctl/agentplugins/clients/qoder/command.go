package qoder

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Bind both native profile selectors to the same canonical root. Preserve the
// ordinary process environment while removing ambient Qoder profile redirects.
func qoderCommand(root, executable string, args ...string) (legacyports.Command, error) {
	if err := validateProfile(root, ""); err != nil {
		return legacyports.Command{}, err
	}
	environment := []string{"QODER_CONFIG_DIR=" + root}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "QODER_CONFIG_DIR") && !strings.EqualFold(key, "QODER_HOME") {
			environment = append(environment, entry)
		}
	}
	return legacyports.Command{Argv: append([]string{executable, "--config-dir", root}, args...), Env: environment}, nil
}

func runQoder(ctx context.Context, env clients.Env, root, executable string, args ...string) error {
	command, err := qoderCommand(root, executable, args...)
	if err != nil {
		return err
	}
	if env.Runner == nil {
		return fmt.Errorf("qoder CLI runner is unavailable")
	}
	result, err := env.Runner.Run(ctx, command)
	if err != nil {
		return fmt.Errorf("start Qoder CLI: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("qoder CLI command failed with exit code %d", result.ExitCode)
	}
	return nil
}
