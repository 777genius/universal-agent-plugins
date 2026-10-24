package nativeconfig

import (
	"fmt"
	"strings"
)

// Copilot repository configuration uses explicit stdio/http/sse transport tags.
// The adapter supplies a managed launcher instead of an unqualified native cwd.
func projectCopilotServer(server Server, placeholders Placeholders) (map[string]any, error) {
	nativeType := "stdio"
	if strings.EqualFold(strings.TrimSpace(server.Type), "remote") {
		switch server.RemoteTransport {
		case "streamable-http":
			nativeType = "http"
		case "sse":
			nativeType = "sse"
		default:
			return nil, fmt.Errorf("copilot remote MCP server requires streamable-http or sse transport")
		}
		server.RemoteTransport = ""
	}
	if server.CWD != "" {
		return nil, fmt.Errorf("copilot project stdio requires managed launcher cwd binding")
	}
	entry, err := projectBaseServer(CodecMCPServers, server, placeholders)
	if err != nil {
		return nil, err
	}
	entry["type"], entry["tools"] = nativeType, []string{"*"}
	return entry, nil
}
