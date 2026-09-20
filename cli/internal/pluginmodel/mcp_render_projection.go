package pluginmodel

import "fmt"

func renderPortableMCPForTarget(m *PortableMCP, target string) (map[string]any, error) {
	if m == nil {
		return nil, nil
	}
	if m.File == nil {
		return nil, fmt.Errorf("portable MCP model is missing typed file data")
	}
	return renderPortableMCPLegacyProjection(*m.File, target), nil
}

func renderPortableMCPLegacyProjection(file PortableMCPFile, target string) map[string]any {
	target = NormalizeTarget(target)
	out := map[string]any{}
	for alias, server := range file.Servers {
		if !portableMCPServerAppliesTo(server, target) {
			continue
		}
		generated := generatePortableMCPServer(server, target)
		if len(generated) == 0 {
			continue
		}
		out[alias] = generated
	}
	return out
}

func portableMCPServerAppliesTo(server PortableMCPServer, target string) bool {
	if target == "" || len(server.Targets) == 0 {
		return true
	}
	for _, entry := range server.Targets {
		if entry == target {
			return true
		}
	}
	return false
}

func generatePortableMCPServer(server PortableMCPServer, target string) map[string]any {
	var out map[string]any
	switch server.Type {
	case "stdio":
		out = renderPortableMCPStdioTransport(target, server.Stdio)
	case "remote":
		out = renderPortableMCPRemoteTransport(target, server.Remote)
	default:
		return nil
	}
	if target != "" {
		normalized := NormalizeTarget(target)
		if override := server.Overrides[normalized]; len(override) > 0 {
			mergeExtraObject(out, cloneAnyMap(override))
		}
		if passthrough := server.Passthrough[normalized]; len(passthrough) > 0 {
			mergeExtraObject(out, cloneAnyMap(passthrough))
		}
	}
	generated, ok := translatePortableMCPProjectionValue(target, out).(map[string]any)
	if !ok {
		return out
	}
	return generated
}
