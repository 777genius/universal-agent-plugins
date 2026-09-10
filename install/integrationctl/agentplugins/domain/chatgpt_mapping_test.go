package domain

import "testing"

func TestPersonalChatGPTMappingIsBoundToCanonicalIdentityAndOAuthServer(t *testing.T) {
	mapping := ChatGPTLocalMapping{ProductID: "context7", Repository: "upstash/context7", PackagePath: "plugins/agent-plugins/context7", Server: "context7", URL: Context7OAuthURL, AppID: "plugin_asdk_app_0123456789abcdef0123456789abcdef"}
	envelope := PackageEnvelope{Manifest: PluginManifest{Name: "context7"}, Source: SourceIdentity{Repository: mapping.Repository, PackageSubpath: mapping.PackagePath}, MCP: MCPComponent{Enabled: true, Servers: map[string]MCPServer{"context7": {Type: "streamable-http", Decoded: map[string]any{"url": Context7OAuthURL}}}}}
	if err := mapping.ValidatePackage(envelope); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*PackageEnvelope){
		func(e *PackageEnvelope) { e.Manifest.Name = "other" },
		func(e *PackageEnvelope) { e.Source.Repository = "other/context7" },
		func(e *PackageEnvelope) { e.Source.PackageSubpath = "other" },
		func(e *PackageEnvelope) { e.MCP.Enabled = false },
		func(e *PackageEnvelope) {
			e.MCP.Servers = map[string]MCPServer{"context7": {Type: "streamable-http", Decoded: map[string]any{"url": "https://mcp.context7.com/mcp"}}}
		},
		func(e *PackageEnvelope) {
			e.MCP.Servers = map[string]MCPServer{"context7": {Type: "stdio", Decoded: map[string]any{"url": Context7OAuthURL}}}
		},
		func(e *PackageEnvelope) {
			e.MCP.Servers = map[string]MCPServer{"context7": envelope.MCP.Servers["context7"], "other": {Type: "streamable-http"}}
		},
	} {
		altered := envelope
		change(&altered)
		if err := mapping.ValidatePackage(altered); err == nil {
			t.Fatalf("accepted mismatched package %+v", altered)
		}
	}
}
