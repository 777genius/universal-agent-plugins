package opencode

import (
	"errors"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/portablemcp"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func renderOpenCodeMCP(loaded portablemcp.Loaded, sourceRoot string) map[string]any {
	out := make(map[string]any, len(loaded.Servers))
	for alias, server := range loaded.Servers {
		switch server.Type {
		case "stdio":
			command := make([]any, 0, 1+len(server.Stdio.Args))
			command = append(command, interpolatePackageRoot(server.Stdio.Command, sourceRoot))
			for _, arg := range server.Stdio.Args {
				command = append(command, interpolatePackageRoot(arg, sourceRoot))
			}
			entry := map[string]any{
				"type":    "local",
				"command": command,
			}
			if len(server.Stdio.Env) > 0 {
				env := make(map[string]any, len(server.Stdio.Env))
				for key, value := range server.Stdio.Env {
					env[key] = interpolatePackageRoot(value, sourceRoot)
				}
				entry["environment"] = env
			}
			out[alias] = entry
		case "remote":
			entry := map[string]any{
				"type": "remote",
				"url":  interpolatePackageRoot(server.Remote.URL, sourceRoot),
			}
			if len(server.Remote.Headers) > 0 {
				headers := make(map[string]any, len(server.Remote.Headers))
				for key, value := range server.Remote.Headers {
					headers[key] = interpolatePackageRoot(value, sourceRoot)
				}
				entry["headers"] = headers
			}
			out[alias] = entry
		}
	}
	return out
}

func interpolatePackageRoot(value, packageRoot string) string {
	return strings.ReplaceAll(value, "${package.root}", packageRoot)
}

func isMissingPortableMCP(err error) bool {
	if err == nil {
		return false
	}
	var de *domain.Error
	if errors.As(err, &de) {
		return de.Code == domain.ErrManifestLoad && strings.Contains(strings.ToLower(de.Message), "portable mcp file not found")
	}
	return false
}
