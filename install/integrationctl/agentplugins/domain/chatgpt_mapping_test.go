package domain

import "testing"

func TestPersonalChatGPTMappingBindsCanonicalPackageToChatGPTEndpoint(t *testing.T) {
	mapping := ChatGPTLocalMapping{ProductID: "context7", Repository: "upstash/context7", PackagePath: "plugins/agent-plugins/context7", Server: "context7", URL: Context7ChatGPTURL, AppID: "asdk_app_0123456789abcdef0123456789abcdef"}
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

func TestLegacyContext7RegistrationIsNarrowlyRecognized(t *testing.T) {
	legacy := ChatGPTLocalMapping{ProductID: "context7", Repository: "upstash/context7", PackagePath: "plugins/agent-plugins/context7", Server: "context7", URL: Context7OAuthURL, AppID: "plugin_asdk_app_0123456789abcdef0123456789abcdef"}
	if !legacy.IsLegacyContext7Registration() {
		t.Fatal("legacy 0.1.56 receipt was not recognized")
	}
	legacy.URL = Context7ChatGPTURL
	if legacy.IsLegacyContext7Registration() {
		t.Fatal("current registration was classified as legacy")
	}
	legacy.URL = Context7OAuthURL
	legacy.AppID = "plugin_asdk_app_invalid"
	if legacy.IsLegacyContext7Registration() {
		t.Fatal("malformed legacy registration was recognized")
	}
}
