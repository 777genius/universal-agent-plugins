package nativeconfig

import (
	"fmt"
	"strings"
)

// VS Code's project servers use type:stdio/http/sse. This is independent of
// Copilot's mcpServers dialect: cwd is supported and no tools policy is added.
func projectVSCodeServer(server Server, placeholders Placeholders) (map[string]any, error) {
	nativeType := "stdio"
	if strings.EqualFold(strings.TrimSpace(server.Type), "remote") {
		switch server.RemoteTransport {
		case "streamable-http":
			nativeType = "http"
		case "sse":
			nativeType = "sse"
		default:
			return nil, fmt.Errorf("VS Code project remote MCP requires streamable-http or sse transport")
		}
		server.RemoteTransport = ""
	}
	entry, err := projectBaseServer(CodecMCPServers, server, placeholders)
	if err != nil {
		return nil, err
	}
	entry["type"] = nativeType
	return entry, nil
}
