package specregistry

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestRegistryValidatesPinnedSchemas(t *testing.T) {
	t.Parallel()
	registry, err := New()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	plugin := map[string]any{"$schema": domain.PluginSchemaV1, "name": "context7"}
	if err := registry.Validate(domain.PluginSchemaV1, plugin); err != nil {
		t.Fatalf("validate plugin: %v", err)
	}
	mcp := map[string]any{
		"$schema": domain.MCPSchemaV1,
		"mcpServers": map[string]any{
			"context7": map[string]any{"type": "stdio", "command": "npx"},
		},
	}
	if err := registry.Validate(domain.MCPSchemaV1, mcp); err != nil {
		t.Fatalf("validate mcp: %v", err)
	}
}

func TestRegistryRejectsUnsupportedAndInvalidDocuments(t *testing.T) {
	t.Parallel()
	registry, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Validate("https://example.com/future.json", map[string]any{}); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported schema error = %v", err)
	}
	if err := registry.Validate(domain.PluginSchemaV1, map[string]any{"$schema": domain.PluginSchemaV1, "name": "Unsafe Name"}); err == nil {
		t.Fatal("invalid plugin accepted")
	}
}

func TestRegistryReportsPinnedDigests(t *testing.T) {
	t.Parallel()
	registry, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if digest, ok := registry.Digest(domain.PluginSchemaV1); !ok || digest != pluginSchemaDigest {
		t.Fatalf("plugin digest = %q, %v", digest, ok)
	}
	if digest, ok := registry.Digest(domain.MCPSchemaV1); !ok || digest != mcpSchemaDigest {
		t.Fatalf("mcp digest = %q, %v", digest, ok)
	}
}
