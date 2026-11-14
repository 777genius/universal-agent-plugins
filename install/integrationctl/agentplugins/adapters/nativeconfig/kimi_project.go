package nativeconfig

import (
	"fmt"
	"strings"
)

// Current Kimi Code project MCP uses transport:stdio/http/sse. Keep this
// project-only dialect separate from the preserved user plugin manifest.
func projectKimiServer(server Server, placeholders Placeholders) (map[string]any, error) {
	transport := "stdio"
	if strings.EqualFold(strings.TrimSpace(server.Type), "remote") {
		switch server.RemoteTransport {
		case "streamable-http":
			transport = "http"
		case "sse":
			transport = "sse"
		default:
			return nil, fmt.Errorf("kimi project remote MCP requires streamable-http or sse transport")
		}
		server.RemoteTransport = ""
	}
	entry, err := projectBaseServer(CodecMCPServers, server, placeholders)
	if err != nil {
		return nil, err
	}
	entry["transport"] = transport
	return entry, nil
}
