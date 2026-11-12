package cline

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Cline's ordinary detector deliberately retains file-only authority. Resolve
// its observed CLI only for an explicit session, not for install/version probes.
func clineSessionExecutable(env clients.Env, client domain.DetectedClient, bindings int) (string, error) {
	if client.ClientID != domain.ClientCline || client.Status != domain.DetectionDetected || bindings == 0 {
		return "", fmt.Errorf("an installed Cline CLI and owned project bindings are required")
	}
	// An absolute executable field is not proof of the CLI surface:
	// editor-only detection must never enter the native process runner.
	if !hasClineCLISurface(client.Surfaces) {
		return "", fmt.Errorf("Cline CLI surface is required; editor-only detection is insufficient")
	}
	executable := client.ExecutablePath
	if executable == "" {
		if env.LookupExecutable == nil {
			return "", fmt.Errorf("Cline CLI session resolver is unavailable")
		}
		var err error
		executable, err = env.LookupExecutable("cline")
		if err != nil {
			return "", fmt.Errorf("resolve selected Cline CLI: %w", err)
		}
	}
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("Cline session executable must resolve to an absolute path")
	}
	return executable, nil
}

func hasClineCLISurface(surfaces []domain.ClientSurface) bool {
	for _, surface := range surfaces {
		if surface.ID == "cline_cli" && surface.Detected {
			return true
		}
	}
	return false
}
