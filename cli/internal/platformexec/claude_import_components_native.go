package platformexec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func importClaudeLSP(root string, manifest importedClaudePluginManifest) ([]pluginmodel.Artifact, []pluginmodel.Warning, error) {
	const targetPath = "targets/claude/lsp.json"
	if fileExists(filepath.Join(root, ".lsp.json")) {
		body, err := os.ReadFile(filepath.Join(root, ".lsp.json"))
		if err != nil {
			return nil, nil, err
		}
		return []pluginmodel.Artifact{{RelPath: targetPath, Content: body}}, nil, nil
	}
	if !manifest.LSPOverride {
		return nil, nil, nil
	}
	switch {
	case manifest.InlineLSP != nil:
		return []pluginmodel.Artifact{{RelPath: targetPath, Content: mustJSON(manifest.InlineLSP)}}, []pluginmodel.Warning{claudeImportManifestWarning("inline Claude lspServers were normalized into targets/claude/lsp.json")}, nil
	case len(manifest.LSPRefs) == 1:
		ref := cleanRelativeRef(manifest.LSPRefs[0])
		body, err := os.ReadFile(filepath.Join(root, ref))
		if err != nil {
			return nil, nil, err
		}
		return []pluginmodel.Artifact{{RelPath: targetPath, Content: body}}, []pluginmodel.Warning{claudeImportManifestWarning("custom Claude lspServers path was normalized into targets/claude/lsp.json")}, nil
	case len(manifest.LSPRefs) > 1:
		body, err := mergeClaudeObjectRefs(root, manifest.LSPRefs, "Claude lspServers")
		if err != nil {
			return nil, nil, err
		}
		return []pluginmodel.Artifact{{RelPath: targetPath, Content: body}}, []pluginmodel.Warning{claudeImportManifestWarning("custom Claude lspServers path array was normalized into canonical package-standard layout")}, nil
	default:
		return nil, nil, nil
	}
}

func importClaudeMCP(root string, manifest importedClaudePluginManifest) ([]pluginmodel.Artifact, []pluginmodel.Warning, error) {
	if fileExists(filepath.Join(root, ".mcp.json")) || !manifest.MCPOverride {
		return nil, nil, nil
	}
	switch {
	case manifest.InlineMCP != nil:
		artifact, err := importedPortableMCPArtifact("claude", manifest.InlineMCP)
		if err != nil {
			return nil, nil, err
		}
		return []pluginmodel.Artifact{artifact}, []pluginmodel.Warning{claudeImportManifestWarning(fmt.Sprintf("inline Claude mcpServers were normalized into %s/mcp/servers.yaml", pluginmodel.SourceDirName))}, nil
	case len(manifest.MCPRefs) == 1:
		ref := cleanRelativeRef(manifest.MCPRefs[0])
		body, err := os.ReadFile(filepath.Join(root, ref))
		if err != nil {
			return nil, nil, err
		}
		doc, err := decodeJSONObject(body, "Claude mcpServers")
		if err != nil {
			return nil, nil, err
		}
		artifact, err := importedPortableMCPArtifact("claude", doc)
		if err != nil {
			return nil, nil, err
		}
		return []pluginmodel.Artifact{artifact}, []pluginmodel.Warning{claudeImportManifestWarning(fmt.Sprintf("custom Claude mcpServers path was normalized into %s/mcp/servers.yaml", pluginmodel.SourceDirName))}, nil
	case len(manifest.MCPRefs) > 1:
		body, err := mergeClaudeObjectRefs(root, manifest.MCPRefs, "Claude mcpServers")
		if err != nil {
			return nil, nil, err
		}
		doc, err := decodeJSONObject(body, "Claude mcpServers")
		if err != nil {
			return nil, nil, err
		}
		artifact, err := importedPortableMCPArtifact("claude", doc)
		if err != nil {
			return nil, nil, err
		}
		return []pluginmodel.Artifact{artifact}, []pluginmodel.Warning{claudeImportManifestWarning("custom Claude mcpServers path array was normalized into canonical package-standard layout")}, nil
	default:
		return nil, nil, nil
	}
}

func claudeImportManifestWarning(message string) pluginmodel.Warning {
	return pluginmodel.Warning{
		Kind:    pluginmodel.WarningFidelity,
		Path:    filepath.ToSlash(filepath.Join(".claude-plugin", "plugin.json")),
		Message: message,
	}
}
