package platformexec

import (
	"fmt"
	"strings"
)

func validateGeminiMCPCommandFields(path, serverName string, server map[string]any, hasCommand bool) []Diagnostic {
	var diagnostics []Diagnostic
	if server["args"] != nil && !hasCommand {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     path,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension MCP server %q may only use args with command-based stdio transport", serverName),
		})
	}
	if server["env"] != nil && !hasCommand {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     path,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension MCP server %q may only use env with command-based stdio transport", serverName),
		})
	}
	if server["cwd"] != nil && !hasCommand {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     path,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension MCP server %q may only use cwd with command-based stdio transport", serverName),
		})
	}
	return diagnostics
}

func validateGeminiMCPArgs(path, serverName string, value any) []Diagnostic {
	if value == nil {
		return nil
	}
	items, valid := geminiStringSlice(value)
	if !valid {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     path,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension MCP server %q args must be an array of strings", serverName),
		}}
	}
	if len(items) == 0 {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     path,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension MCP server %q args may not be empty when provided", serverName),
		}}
	}
	return nil
}

func validateGeminiMCPEnv(path, serverName string, value any) []Diagnostic {
	if value == nil {
		return nil
	}
	if _, valid := geminiStringMap(value); !valid {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     path,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension MCP server %q env must be an object of string values", serverName),
		}}
	}
	return nil
}

func validateGeminiMCPCwd(path, serverName string, value any) []Diagnostic {
	if value == nil {
		return nil
	}
	if cwd, ok := value.(string); !ok || strings.TrimSpace(cwd) == "" {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     path,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension MCP server %q cwd must be a non-empty string", serverName),
		}}
	}
	return nil
}
