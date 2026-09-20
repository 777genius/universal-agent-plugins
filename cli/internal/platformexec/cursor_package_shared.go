package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readImportedCursorPluginManifest(root string) (map[string]any, bool, error) {
	body, err := os.ReadFile(filepath.Join(root, ".cursor-plugin", "plugin.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	doc, err := decodeJSONObject(body, "Cursor plugin manifest .cursor-plugin/plugin.json")
	if err != nil {
		return nil, false, err
	}
	return doc, true, nil
}

func importedCursorPluginMCP(root string, manifest map[string]any) (map[string]any, bool, error) {
	value, ok := manifest["mcpServers"]
	if !ok {
		if servers, ok, err := readCursorMCPServers(filepath.Join(root, ".mcp.json"), ".mcp.json"); err != nil {
			return nil, false, err
		} else if ok {
			return servers, true, nil
		}
		return nil, false, nil
	}
	switch typed := value.(type) {
	case nil:
		return nil, false, nil
	case string:
		ref := strings.TrimSpace(typed)
		switch ref {
		case "", cursorPluginMCPRef, ".mcp.json":
		default:
			return nil, false, fmt.Errorf("unsupported Cursor plugin mcpServers ref %q: use %q", ref, cursorPluginMCPRef)
		}
		return readCursorMCPServers(filepath.Join(root, ".mcp.json"), ".mcp.json")
	case map[string]any:
		return typed, true, nil
	default:
		return nil, false, fmt.Errorf("Cursor plugin field %q must be a string ref or object", "mcpServers")
	}
}

func stringMapField(doc map[string]any, key string) string {
	value, ok := doc[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
