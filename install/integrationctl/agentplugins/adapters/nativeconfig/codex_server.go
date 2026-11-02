package nativeconfig

import (
	"fmt"
	"strings"
)

func projectCodexServer(server Server, placeholders Placeholders) (map[string]any, error) {
	if server.OpenCodeV2 != nil {
		return nil, fmt.Errorf("codex does not accept OpenCode V2 options")
	}
	if strings.EqualFold(strings.TrimSpace(server.Type), "remote") && server.RemoteTransport == "streamable-http" {
		server.RemoteTransport = ""
	}
	projected, err := projectBaseServer(CodecMCPServers, server, placeholders)
	if err != nil {
		return nil, err
	}
	if headers, ok := projected["headers"]; ok {
		delete(projected, "headers")
		projected["http_headers"] = headers
	}
	if server.CodexAuth != nil {
		if server.Type != "remote" {
			return nil, fmt.Errorf("codex stdio servers do not accept remote authentication options")
		}
		if server.CodexAuth.BearerTokenEnvVar != "" {
			projected["bearer_token_env_var"] = server.CodexAuth.BearerTokenEnvVar
		}
		if server.CodexAuth.OAuthResource != "" {
			projected["oauth_resource"] = server.CodexAuth.OAuthResource
		}
	}
	return projected, nil
}
