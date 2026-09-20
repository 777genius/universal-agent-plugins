package platformexec

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateCursorPortableMCPContract(root string, manifest map[string]any, graph pluginmodel.PackageGraph) ([]Diagnostic, error) {
	if graph.Portable.MCP == nil {
		return validateCursorManifestWithoutPortableMCP(manifest), nil
	}
	return validateCursorPortableMCP(root, manifest, graph)
}

func validateCursorManifestWithoutPortableMCP(manifest map[string]any) []Diagnostic {
	if _, ok := manifest["mcpServers"]; !ok {
		return nil
	}
	return []Diagnostic{cursorPluginManifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("Cursor plugin manifest %s may not define mcpServers when portable MCP is absent", cursorPluginManifestPath),
	)}
}

func validateCursorPortableMCP(root string, manifest map[string]any, graph pluginmodel.PackageGraph) ([]Diagnostic, error) {
	diagnostics := validateCursorMCPRef(manifest)
	if len(diagnostics) > 0 {
		return diagnostics, nil
	}
	projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "cursor")
	if err != nil {
		return nil, err
	}
	rendered, ok, err := readCursorMCPServers(filepath.Join(root, ".mcp.json"), ".mcp.json")
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     ".mcp.json",
			Target:   "cursor",
			Message:  fmt.Sprintf("Cursor plugin MCP manifest .mcp.json is invalid: %v", err),
		}}, nil
	}
	if !ok {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     ".mcp.json",
			Target:   "cursor",
			Message:  "Cursor plugin MCP manifest .mcp.json is not readable",
		}}, nil
	}
	if jsonDocumentsEqual(projected, rendered) {
		return nil, nil
	}
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     ".mcp.json",
		Target:   "cursor",
		Message:  "Cursor plugin MCP manifest .mcp.json does not match authored portable MCP projection",
	}}, nil
}

func validateCursorMCPRef(manifest map[string]any) []Diagnostic {
	ref, ok := manifest["mcpServers"]
	if !ok {
		return []Diagnostic{cursorPluginManifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("Cursor plugin manifest %s must reference %q when portable MCP is authored", cursorPluginManifestPath, cursorPluginMCPRef),
		)}
	}
	refText, ok := ref.(string)
	if !ok {
		return []Diagnostic{cursorPluginManifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("Cursor plugin manifest %s field %q must be the string %q", cursorPluginManifestPath, "mcpServers", cursorPluginMCPRef),
		)}
	}
	if strings.TrimSpace(refText) == cursorPluginMCPRef {
		return nil
	}
	return []Diagnostic{cursorPluginManifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("Cursor plugin manifest %s must use %q for mcpServers when portable MCP is present", cursorPluginManifestPath, cursorPluginMCPRef),
	)}
}
