package platformexec

import (
	"fmt"
	"strings"
)

func validateGeminiMCPServers(path string, servers map[string]any) []Diagnostic {
	var diagnostics []Diagnostic
	for serverName, raw := range servers {
		server, ok := raw.(map[string]any)
		if !ok {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     path,
				Target:   "gemini",
				Message:  fmt.Sprintf("Gemini extension MCP server %q must be a JSON object", serverName),
			})
			continue
		}
		diagnostics = append(diagnostics, validateGeminiMCPServer(path, serverName, server)...)
	}
	return diagnostics
}

func validateGeminiMCPServer(path, serverName string, server map[string]any) []Diagnostic {
	var diagnostics []Diagnostic
	if _, blocked := server["trust"]; blocked {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     path,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension MCP server %q may not set trust", serverName),
		})
	}

	command, hasCommand, transportDiagnostics := validateGeminiMCPTransport(path, serverName, server)
	diagnostics = append(diagnostics, transportDiagnostics...)

	diagnostics = append(diagnostics, validateGeminiMCPCommandFields(path, serverName, server, hasCommand)...)
	diagnostics = append(diagnostics, validateGeminiMCPArgs(path, serverName, server["args"])...)
	diagnostics = append(diagnostics, validateGeminiMCPEnv(path, serverName, server["env"])...)
	diagnostics = append(diagnostics, validateGeminiMCPCwd(path, serverName, server["cwd"])...)

	if hasCommand && strings.Contains(command, " ") && server["args"] == nil {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityWarning,
			Code:     CodeGeminiMCPCommandStyle,
			Path:     path,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension MCP server %q uses a space-delimited command string; prefer command plus args", serverName),
		})
	}
	return diagnostics
}
