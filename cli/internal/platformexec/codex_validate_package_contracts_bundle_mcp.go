package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateCodexBundleMCPDiagnostics(root string, graph pluginmodel.PackageGraph, pluginManifest codexmanifest.ImportedPluginManifest) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	if hasMCP := graph.Portable.MCP != nil; hasMCP {
		if strings.TrimSpace(pluginManifest.MCPServersRef) == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeGeneratedContractInvalid,
				Path:     codexmanifest.PluginManifestPath(),
				Target:   "codex-package",
				Message:  fmt.Sprintf("Codex plugin manifest .codex-plugin/plugin.json must reference %q when portable MCP is authored", codexmanifest.MCPServersRef),
			})
		}
	} else if strings.TrimSpace(pluginManifest.MCPServersRef) != "" {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  "Codex plugin manifest .codex-plugin/plugin.json may not reference mcpServers when no portable MCP is authored",
		})
	}
	if ref := strings.TrimSpace(pluginManifest.MCPServersRef); ref != "" {
		refDiagnostics, err := validateCodexBundleMCPRefDiagnostics(root, ref, graph)
		if err != nil {
			return nil, err
		}
		diagnostics = append(diagnostics, refDiagnostics...)
	}
	return diagnostics, nil
}

func validateCodexBundleMCPRefDiagnostics(root, ref string, graph pluginmodel.PackageGraph) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	if ref != codexmanifest.MCPServersRef {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest .codex-plugin/plugin.json must use %q for mcpServers when present", codexmanifest.MCPServersRef),
		})
	}
	refPath, err := resolveRelativeRef(root, ref)
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest .codex-plugin/plugin.json uses an invalid mcpServers ref %q: %v", ref, err),
		})
		return diagnostics, nil
	}
	renderedMCP, readDiagnostics := readCodexBundleMCPDoc(root, refPath)
	if len(readDiagnostics) > 0 {
		return append(diagnostics, readDiagnostics...), nil
	}
	if graph.Portable.MCP != nil {
		projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "codex-package")
		if err != nil {
			return nil, err
		}
		if !jsonDocumentsEqual(projected, renderedMCP) {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeGeneratedContractInvalid,
				Path:     filepath.ToSlash(refPath),
				Target:   "codex-package",
				Message:  "Codex MCP manifest .mcp.json does not match authored portable MCP projection",
			})
		}
	}
	return diagnostics, nil
}

func readCodexBundleMCPDoc(root, refPath string) (map[string]any, []Diagnostic) {
	mcpBody, err := os.ReadFile(filepath.Join(root, refPath))
	if err != nil {
		return nil, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     filepath.ToSlash(refPath),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex MCP manifest %s is not readable: %v", filepath.ToSlash(refPath), err),
		}}
	}
	renderedMCP, err := decodeJSONObject(mcpBody, fmt.Sprintf("Codex MCP manifest %s", filepath.ToSlash(refPath)))
	if err != nil {
		return nil, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     filepath.ToSlash(refPath),
			Target:   "codex-package",
			Message:  err.Error(),
		}}
	}
	return renderedMCP, nil
}
