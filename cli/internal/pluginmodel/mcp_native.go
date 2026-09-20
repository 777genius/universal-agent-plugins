package pluginmodel

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func MarshalPortableMCPFile(file PortableMCPFile) ([]byte, error) {
	canonicalizePortableMCPFile(&file)
	normalizePortableMCPFile(&file)
	if err := file.Validate(); err != nil {
		return nil, err
	}
	body, err := yaml.Marshal(file)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func ImportedPortableMCPYAML(sourceTarget string, servers map[string]any) ([]byte, error) {
	file, err := PortableMCPFileFromNative(sourceTarget, servers)
	if err != nil {
		return nil, err
	}
	return MarshalPortableMCPFile(file)
}

func PortableMCPFileFromNative(sourceTarget string, servers map[string]any) (PortableMCPFile, error) {
	target := NormalizeTarget(sourceTarget)
	file := PortableMCPFile{
		APIVersion: APIVersionV1,
		Servers:    map[string]PortableMCPServer{},
	}
	for alias, raw := range servers {
		serverMap, ok := raw.(map[string]any)
		if !ok {
			return PortableMCPFile{}, fmt.Errorf("native MCP server %q must be a JSON object", alias)
		}
		server, err := portableMCPServerFromNative(alias, target, cloneAnyMap(serverMap))
		if err != nil {
			return PortableMCPFile{}, err
		}
		file.Servers[alias] = server
	}
	normalizePortableMCPFile(&file)
	if err := file.Validate(); err != nil {
		return PortableMCPFile{}, err
	}
	return file, nil
}

func portableMCPServerFromNative(alias, target string, raw map[string]any) (PortableMCPServer, error) {
	server := PortableMCPServer{}
	if target != "" {
		server.Targets = []string{target}
	}
	if target == "opencode" {
		if kind, _ := raw["type"].(string); strings.EqualFold(strings.TrimSpace(kind), "local") {
			command, err := anyStringSlice(raw["command"])
			if err != nil || len(command) == 0 {
				return PortableMCPServer{}, fmt.Errorf("native OpenCode MCP server %q local command must be a non-empty string array", alias)
			}
			server.Type = "stdio"
			server.Stdio = &PortableMCPStdio{
				Command: command[0],
				Args:    append([]string(nil), command[1:]...),
				Env:     anyToStringMap(raw["environment"]),
			}
			delete(raw, "type")
			delete(raw, "command")
			delete(raw, "environment")
			return withPortableMCPPassthrough(server, target, raw), nil
		}
		if kind, _ := raw["type"].(string); strings.EqualFold(strings.TrimSpace(kind), "remote") {
			server.Type = "remote"
			server.Remote = &PortableMCPRemote{
				Protocol: "streamable_http",
				URL:      optionalAnyString(raw["url"]),
				Headers:  anyToStringMap(raw["headers"]),
			}
			delete(raw, "type")
			delete(raw, "url")
			delete(raw, "headers")
			return withPortableMCPPassthrough(server, target, raw), nil
		}
	}
	if httpURL := optionalAnyString(raw["httpUrl"]); httpURL != "" {
		server.Type = "remote"
		server.Remote = &PortableMCPRemote{
			Protocol: "streamable_http",
			URL:      httpURL,
			Headers:  anyToStringMap(raw["headers"]),
		}
		delete(raw, "httpUrl")
		delete(raw, "headers")
		return withPortableMCPPassthrough(server, target, raw), nil
	}
	if command := optionalAnyString(raw["command"]); command != "" {
		server.Type = "stdio"
		server.Stdio = &PortableMCPStdio{
			Command: command,
			Args:    mustStringSlice(raw["args"]),
			Env:     anyToStringMap(raw["env"]),
		}
		delete(raw, "command")
		delete(raw, "args")
		delete(raw, "env")
		return withPortableMCPPassthrough(server, target, raw), nil
	}
	if url := optionalAnyString(raw["url"]); url != "" {
		protocol := "streamable_http"
		switch strings.ToLower(strings.TrimSpace(optionalAnyString(raw["type"]))) {
		case "sse":
			protocol = "sse"
		case "http", "remote":
			protocol = "streamable_http"
		}
		if target == "gemini" {
			protocol = "sse"
		}
		server.Type = "remote"
		server.Remote = &PortableMCPRemote{
			Protocol: protocol,
			URL:      url,
			Headers:  anyToStringMap(raw["headers"]),
		}
		delete(raw, "type")
		delete(raw, "url")
		delete(raw, "headers")
		return withPortableMCPPassthrough(server, target, raw), nil
	}
	return PortableMCPServer{}, fmt.Errorf("native MCP server %q could not be normalized into portable MCP v1", alias)
}

func withPortableMCPPassthrough(server PortableMCPServer, target string, raw map[string]any) PortableMCPServer {
	if len(raw) == 0 || target == "" {
		return server
	}
	if server.Passthrough == nil {
		server.Passthrough = map[string]map[string]any{}
	}
	server.Passthrough[target] = raw
	return server
}
