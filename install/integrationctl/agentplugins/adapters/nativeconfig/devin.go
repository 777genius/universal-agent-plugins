package nativeconfig

import "fmt"

// Current Devin uses command/args/env without cwd, and url/headers with an
// explicit http/sse transport. It is separate from legacy Cascade serverUrl.
func projectDevinServer(server Server, placeholders Placeholders) (map[string]any, error) {
	if server.CWD != "" {
		return nil, fmt.Errorf("devin stdio MCP does not accept cwd; use the managed launcher")
	}
	transport := ""
	if server.Type == "remote" {
		switch server.RemoteTransport {
		case "streamable-http":
			transport = "http"
		case "sse":
			transport = "sse"
		default:
			return nil, fmt.Errorf("devin remote MCP requires streamable-http or sse transport")
		}
		server.RemoteTransport = ""
	}
	projected, err := projectBaseServer(CodecMCPServers, server, placeholders)
	if err != nil {
		return nil, err
	}
	if transport != "" {
		projected["transport"] = transport
	}
	return projected, nil
}
