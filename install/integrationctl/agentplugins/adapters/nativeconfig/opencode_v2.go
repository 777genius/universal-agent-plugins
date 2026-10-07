package nativeconfig

import (
	"fmt"
	"strings"

	"github.com/tailscale/hujson"
)

// The native shape is bound to TASK-INPUTS/upstream-v2-mcp.ts at the accepted
// upstream 8a8bd622a3d7dc29ccf30ec17f84e363ed95ed72 / package 2.0.21.
// Shared local/remote fields reuse the V1 projector; V1 bytes and digest domain
// stay unchanged. This codec does not select a host or activate/migrate it.
func projectOpenCodeV2Server(server Server, placeholders Placeholders) (map[string]any, error) {
	if strings.ToLower(strings.TrimSpace(server.Type)) == "remote" {
		switch server.RemoteTransport {
		case "", "streamable-http": // Native remote URL uses Streamable HTTP.
			server.RemoteTransport = ""
		default:
			return nil, fmt.Errorf("OpenCode V2 remote MCP requires streamable-http transport")
		}
	}
	entry, err := projectBaseServer(CodecOpenCode, server, placeholders)
	if err != nil {
		return nil, err
	}
	entry["disabled"] = false
	if options := server.OpenCodeV2; options != nil {
		entry["disabled"] = options.Disabled
		if timeout := options.Timeout; timeout != nil {
			if timeout.Startup < 0 || timeout.Catalog < 0 || timeout.Execution < 0 {
				return nil, fmt.Errorf("OpenCode V2 timeout values must be positive when configured")
			}
			// Copy into native values so callers cannot mutate a projected entry.
			value := map[string]int{}
			if timeout.Startup != 0 {
				value["startup"] = timeout.Startup
			}
			if timeout.Catalog != 0 {
				value["catalog"] = timeout.Catalog
			}
			if timeout.Execution != 0 {
				value["execution"] = timeout.Execution
			}
			entry["timeout"] = value
		}
	}
	return entry, nil
}

// Only additions/updates need target-host root qualification. Read-only receipt
// inspection and exact-owned removal remain possible after a host disappears
// or foreign V1 entries arrive. Neither operation converts any foreign data.
func requireOpenCodeV2Root(doc *document) error {
	mcp, err := collection(doc, "mcp", false)
	if err != nil || mcp == nil {
		return err
	}
	for i := range mcp.Members {
		if mcp.Members[i].Name.Value.(hujson.Literal).String() != "servers" {
			return ErrNativeMigrationRequired
		}
	}
	servers, err := objectCollection(mcp, "servers", false)
	if err != nil || servers == nil {
		return err
	}
	// Flat V1 may have a server literally named "servers". Never turn it into
	// a nested V2 container, even if the proposed name is absent.
	for _, key := range []string{"type", "enabled"} {
		member, _ := objectMember(servers, key)
		if member != nil {
			if _, object := member.Value.Value.(*hujson.Object); !object {
				return ErrNativeMigrationRequired
			}
		}
	}
	return nil
}
