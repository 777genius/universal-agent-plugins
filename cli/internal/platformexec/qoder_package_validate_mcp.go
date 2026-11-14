package platformexec

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateQoderPortableMCPContract(root string, manifest map[string]any, graph pluginmodel.PackageGraph) ([]Diagnostic, error) {
	if graph.Portable.MCP == nil {
		return validateQoderManifestWithoutPortableMCP(manifest), nil
	}
	return validateQoderPortableMCP(root, manifest, graph)
}

func validateQoderManifestWithoutPortableMCP(manifest map[string]any) []Diagnostic {
	if _, ok := manifest["mcpServers"]; !ok {
		return nil
	}
	return []Diagnostic{qoderPluginManifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("Qoder plugin manifest %s may not define mcpServers when portable MCP is absent", qoderPluginManifestPath),
	)}
}

func validateQoderPortableMCP(root string, manifest map[string]any, graph pluginmodel.PackageGraph) ([]Diagnostic, error) {
	if diagnostics := validateQoderMCPRef(manifest); len(diagnostics) > 0 {
		return diagnostics, nil
	}
	projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "qoder")
	if err != nil {
		return nil, err
	}
	want := map[string]any{"mcpServers": projected}
	doc, ok, err := readQoderMCPDocument(root)
	if err != nil {
		return []Diagnostic{qoderMCPDiagnostic(fmt.Sprintf("Qoder plugin MCP manifest .mcp.json is invalid: %v", err))}, nil
	}
	if !ok {
		return []Diagnostic{qoderMCPDiagnostic("Qoder plugin MCP manifest .mcp.json is not readable")}, nil
	}
	if _, wrapped := doc["mcpServers"]; !wrapped {
		return []Diagnostic{qoderMCPDiagnostic(`Qoder plugin MCP manifest .mcp.json must wrap servers under "mcpServers"`)}, nil
	}
	if jsonDocumentsEqual(want, doc) {
		return nil, nil
	}
	return []Diagnostic{qoderMCPDiagnostic("Qoder plugin MCP manifest .mcp.json does not match authored portable MCP projection")}, nil
}

func qoderMCPDiagnostic(message string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     ".mcp.json",
		Target:   "qoder",
		Message:  message,
	}
}

func validateQoderMCPRef(manifest map[string]any) []Diagnostic {
	ref, ok := manifest["mcpServers"]
	if !ok {
		return []Diagnostic{qoderPluginManifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("Qoder plugin manifest %s must reference %q when portable MCP is authored", qoderPluginManifestPath, qoderPluginMCPRef),
		)}
	}
	refText, ok := ref.(string)
	if !ok {
		return []Diagnostic{qoderPluginManifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("Qoder plugin manifest %s field %q must be the string %q", qoderPluginManifestPath, "mcpServers", qoderPluginMCPRef),
		)}
	}
	if strings.TrimSpace(refText) == qoderPluginMCPRef {
		return nil
	}
	return []Diagnostic{qoderPluginManifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("Qoder plugin manifest %s must use %q for mcpServers when portable MCP is present", qoderPluginManifestPath, qoderPluginMCPRef),
	)}
}
