package pluginmodel

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func (file PortableMCPFile) Validate() error {
	switch {
	case file.APIVersion != "":
		if file.APIVersion != APIVersionV1 {
			return fmt.Errorf("portable MCP api_version must be %q", APIVersionV1)
		}
		if file.Format != "" || file.Version != 0 {
			return fmt.Errorf("portable MCP api_version may not be mixed with legacy format/version markers")
		}
	case file.Format != "" || file.Version != 0:
		if file.Format != PortableMCPLegacyFormatMarker {
			return fmt.Errorf("portable MCP legacy format must be %q", PortableMCPLegacyFormatMarker)
		}
		if file.Version != 1 {
			return fmt.Errorf("portable MCP legacy version %d is unsupported", file.Version)
		}
	default:
		return fmt.Errorf("portable MCP api_version must be %q", APIVersionV1)
	}
	if len(file.Servers) == 0 {
		return fmt.Errorf("portable MCP servers must not be empty")
	}
	supportedTargets := setOf(platformmeta.IDs())
	for alias, server := range file.Servers {
		if !portableMCPAliasRe.MatchString(strings.TrimSpace(alias)) {
			return fmt.Errorf("portable MCP server %q must use lowercase letters, digits, and hyphens only", alias)
		}
		switch server.Type {
		case "stdio":
			if server.Stdio == nil {
				return fmt.Errorf("portable MCP server %q type stdio requires stdio config", alias)
			}
			if strings.TrimSpace(server.Stdio.Command) == "" {
				return fmt.Errorf("portable MCP server %q type stdio requires stdio.command", alias)
			}
			if server.Remote != nil {
				return fmt.Errorf("portable MCP server %q type stdio may not define remote config", alias)
			}
		case "remote":
			if server.Remote == nil {
				return fmt.Errorf("portable MCP server %q type remote requires remote config", alias)
			}
			if strings.TrimSpace(server.Remote.URL) == "" {
				return fmt.Errorf("portable MCP server %q type remote requires remote.url", alias)
			}
			switch server.Remote.Protocol {
			case "streamable_http", "sse":
			default:
				return fmt.Errorf("portable MCP server %q remote.protocol must be streamable_http or sse", alias)
			}
			if server.Stdio != nil {
				return fmt.Errorf("portable MCP server %q type remote may not define stdio config", alias)
			}
		default:
			return fmt.Errorf("portable MCP server %q type must be stdio or remote", alias)
		}
		for _, target := range server.Targets {
			if !supportedTargets[target] {
				return fmt.Errorf("portable MCP server %q targets contains unsupported target %q", alias, target)
			}
		}
		for target := range server.Overrides {
			if !supportedTargets[target] {
				return fmt.Errorf("portable MCP server %q overrides contains unsupported target %q", alias, target)
			}
			if managed := portableMCPManagedKeys(server, target); hasManagedConflict(server.Overrides[target], managed) {
				return fmt.Errorf("portable MCP server %q overrides for target %q may not override managed MCP projection keys", alias, target)
			}
		}
		for target := range server.Passthrough {
			if !supportedTargets[target] {
				return fmt.Errorf("portable MCP server %q passthrough contains unsupported target %q", alias, target)
			}
			if managed := portableMCPManagedKeys(server, target); hasManagedConflict(server.Passthrough[target], managed) {
				return fmt.Errorf("portable MCP server %q passthrough for target %q may not override managed MCP projection keys", alias, target)
			}
		}
	}
	return nil
}

func portableMCPManagedKeys(server PortableMCPServer, target string) map[string]struct{} {
	target = NormalizeTarget(target)
	keys := map[string]struct{}{}
	switch server.Type {
	case "stdio":
		switch target {
		case "opencode":
			keys["type"] = struct{}{}
			keys["command"] = struct{}{}
			keys["environment"] = struct{}{}
		default:
			keys["command"] = struct{}{}
			keys["args"] = struct{}{}
			keys["env"] = struct{}{}
		}
	case "remote":
		switch target {
		case "gemini":
			if server.Remote != nil && server.Remote.Protocol == "sse" {
				keys["url"] = struct{}{}
			} else {
				keys["httpUrl"] = struct{}{}
			}
			keys["headers"] = struct{}{}
		default:
			keys["type"] = struct{}{}
			keys["url"] = struct{}{}
			keys["headers"] = struct{}{}
		}
	}
	return keys
}

func hasManagedConflict(extra map[string]any, managed map[string]struct{}) bool {
	for key := range extra {
		if _, ok := managed[key]; ok {
			return true
		}
	}
	return false
}
