package nativeconfig

import (
	"fmt"
	"strings"
)

const (
	OpenCodeMCPObjectKind   = "opencode_global_mcp_server"
	OpenCodeV2MCPObjectKind = "opencode_v2_global_mcp_server"
)

// OpenCodeCodecForDialect is the closed prepared-profile/projection mapping.
// It accepts the serialized dialect so the config kernel does not depend on
// host qualification types. Stored ownership uses OpenCodeCodecForKind.
func OpenCodeCodecForDialect(dialect string) (Codec, error) {
	switch dialect {
	case "opencode_v1":
		return CodecOpenCode, nil
	case "opencode_v2":
		return CodecOpenCodeV2, nil
	default:
		return "", fmt.Errorf("unknown OpenCode config dialect %q", dialect)
	}
}

// OpenCodeCodecForKind is the closed stored-ownership decoder. Skills and
// package receipts are not MCP receipts. A claimed but unknown OpenCode kind
// fails closed rather than disappearing from verification or removal.
func OpenCodeCodecForKind(kind string) (Codec, bool, error) {
	switch kind {
	case OpenCodeMCPObjectKind:
		return CodecOpenCode, true, nil
	case OpenCodeV2MCPObjectKind:
		return CodecOpenCodeV2, true, nil
	case "opencode_global_skill_directory", "opencode_package":
		return "", false, nil
	default:
		if strings.HasPrefix(kind, "opencode_") {
			return "", false, fmt.Errorf("unsupported OpenCode native object kind %q", kind)
		}
		return "", false, nil
	}
}
