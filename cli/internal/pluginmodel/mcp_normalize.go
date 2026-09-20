package pluginmodel

import (
	"slices"
	"strings"
)

func normalizePortableMCPFile(file *PortableMCPFile) {
	file.APIVersion = strings.TrimSpace(file.APIVersion)
	file.Format = strings.TrimSpace(file.Format)
	if file.Servers == nil {
		file.Servers = map[string]PortableMCPServer{}
	}
	for alias, server := range file.Servers {
		server.Description = strings.TrimSpace(server.Description)
		server.Type = strings.ToLower(strings.TrimSpace(server.Type))
		if server.Stdio != nil {
			server.Stdio.Command = strings.TrimSpace(server.Stdio.Command)
			server.Stdio.Args = normalizeStringSlice(server.Stdio.Args)
			server.Stdio.Env = normalizeStringMap(server.Stdio.Env)
		}
		if server.Remote != nil {
			server.Remote.Protocol = normalizeRemoteProtocol(server.Remote.Protocol)
			server.Remote.URL = strings.TrimSpace(server.Remote.URL)
			server.Remote.Headers = normalizeStringMap(server.Remote.Headers)
		}
		server.Targets = normalizePortableMCPTargets(server.Targets)
		server.Overrides = normalizePortableMCPObjectMap(server.Overrides)
		server.Passthrough = normalizePortableMCPObjectMap(server.Passthrough)
		file.Servers[alias] = server
	}
}

func normalizeStringSlice(values []string) []string {
	var out []string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func normalizeStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizePortableMCPTargets(values []string) []string {
	var out []string
	for _, value := range values {
		target := NormalizeTarget(value)
		if target == "" {
			continue
		}
		out = append(out, target)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func normalizePortableMCPObjectMap(values map[string]map[string]any) map[string]map[string]any {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]map[string]any, len(values))
	for key, value := range values {
		target := NormalizeTarget(key)
		if target == "" || len(value) == 0 {
			continue
		}
		out[target] = cloneAnyMap(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeRemoteProtocol(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "http":
		return "streamable_http"
	default:
		return value
	}
}

func canonicalizePortableMCPFile(file *PortableMCPFile) {
	file.APIVersion = APIVersionV1
	file.Format = ""
	file.Version = 0
}
