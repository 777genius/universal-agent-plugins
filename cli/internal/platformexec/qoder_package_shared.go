package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func readImportedQoderPluginManifest(root string) (map[string]any, bool, error) {
	body, err := os.ReadFile(filepath.Join(root, ".qoder-plugin", "plugin.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	doc, err := decodeJSONObject(body, "Qoder plugin manifest .qoder-plugin/plugin.json")
	if err != nil {
		return nil, false, err
	}
	return doc, true, nil
}

func readQoderMCPDocument(root string) (map[string]any, bool, error) {
	body, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	doc, err := decodeJSONObject(body, "Qoder MCP config .mcp.json")
	if err != nil {
		return nil, false, err
	}
	return doc, true, nil
}

// qoderMCPServersFromDocument unwraps the Qoder-sidecar {"mcpServers": {...}} envelope;
// a bare servers map is tolerated for import of hand-authored files.
func qoderMCPServersFromDocument(doc map[string]any) (map[string]any, error) {
	servers, err := pluginmodel.UnwrapNativeMCPServers(doc)
	if err != nil {
		return nil, fmt.Errorf("Qoder MCP config .mcp.json: %w", err)
	}
	return servers, nil
}

func importedQoderPluginMCP(root string, manifest map[string]any) (map[string]any, bool, error) {
	value, ok := manifest["mcpServers"]
	if !ok {
		doc, ok, err := readQoderMCPDocument(root)
		if err != nil || !ok {
			return nil, false, err
		}
		servers, err := qoderMCPServersFromDocument(doc)
		if err != nil {
			return nil, false, err
		}
		return servers, true, nil
	}
	switch typed := value.(type) {
	case nil:
		return nil, false, nil
	case string:
		ref := strings.TrimSpace(typed)
		switch ref {
		case "", qoderPluginMCPRef, ".mcp.json":
		default:
			return nil, false, fmt.Errorf("unsupported Qoder plugin mcpServers ref %q: use %q", ref, qoderPluginMCPRef)
		}
		doc, ok, err := readQoderMCPDocument(root)
		if err != nil || !ok {
			return nil, false, err
		}
		servers, err := qoderMCPServersFromDocument(doc)
		if err != nil {
			return nil, false, err
		}
		return servers, true, nil
	case map[string]any:
		return typed, true, nil
	default:
		return nil, false, fmt.Errorf("Qoder plugin field %q must be a string ref or object", "mcpServers")
	}
}

func qoderPluginAuthorName(manifest map[string]any) string {
	value, ok := manifest["author"]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		return stringMapField(typed, "name")
	default:
		return ""
	}
}

func qoderPluginAuthor(author *pluginmodel.Author) map[string]any {
	if author == nil {
		return nil
	}
	name := strings.TrimSpace(author.Name)
	if name == "" {
		return nil
	}
	return map[string]any{"name": name}
}
