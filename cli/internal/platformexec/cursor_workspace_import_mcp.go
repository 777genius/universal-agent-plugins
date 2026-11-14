package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
)

func appendCursorWorkspaceMCPArtifacts(root string, includeUserScope bool, result *ImportResult) (bool, error) {
	mergedServers, hasCursorState, err := loadCursorWorkspaceMCPServers(root, includeUserScope)
	if err != nil {
		return false, err
	}
	if len(mergedServers) == 0 {
		return hasCursorState, nil
	}
	artifact, err := importedPortableMCPArtifact("cursor-workspace", mergedServers)
	if err != nil {
		return false, err
	}
	result.Artifacts = append(result.Artifacts, artifact)
	return true, nil
}

func loadCursorWorkspaceMCPServers(root string, includeUserScope bool) (map[string]any, bool, error) {
	mergedServers := map[string]any{}
	var hasCursorState bool

	if includeUserScope {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, false, fmt.Errorf("resolve user home for Cursor import: %w", err)
		}
		if imported, ok, err := readCursorMCPServers(filepath.Join(home, ".cursor", "mcp.json"), filepath.ToSlash(filepath.Join("~", ".cursor", "mcp.json"))); err != nil {
			return nil, false, err
		} else if ok {
			mergeOpenCodeObject(mergedServers, imported)
			hasCursorState = true
		}
	}

	if imported, ok, err := readCursorMCPServers(filepath.Join(root, ".cursor", "mcp.json"), ".cursor/mcp.json"); err != nil {
		return nil, false, err
	} else if ok {
		mergeOpenCodeObject(mergedServers, imported)
		hasCursorState = true
	}

	return mergedServers, hasCursorState, nil
}

func readCursorMCPServers(path string, label string) (map[string]any, bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	doc, err := decodeJSONObject(body, "Cursor MCP config "+label)
	if err != nil {
		return nil, false, err
	}
	servers, err := cursorMCPServersFromDocument(doc)
	if err != nil {
		return nil, false, err
	}
	return servers, true, nil
}

func cursorMCPServersFromDocument(doc map[string]any) (map[string]any, error) {
	if value, ok := doc["mcpServers"]; ok {
		servers, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Cursor MCP config .cursor/mcp.json field %q must be a JSON object", "mcpServers")
		}
		if servers == nil {
			return map[string]any{}, nil
		}
		return servers, nil
	}
	return doc, nil
}
