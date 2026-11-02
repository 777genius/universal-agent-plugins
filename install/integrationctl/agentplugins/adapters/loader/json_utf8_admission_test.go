package loader

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// RFC 8259 UTF-8 validity is a document boundary. Go's encoding/json would
// silently replace malformed octets in both opaque keys and regular values.
func TestPortableManifestRejectsMalformedUTF8WithoutReplacement(t *testing.T) {
	invalid := string([]byte{0xff})
	for _, tc := range []struct {
		name, extra string
	}{
		{"known-value", ",\"description\":\"" + invalid + "\""},
		{"unknown-key", ",\"future" + invalid + "\":true"},
		{"opaque-extension-value", ",\"extensions\":{\"com.example.future\":{\"note\":\"" + invalid + "\"}}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			body := "{\"$schema\":\"" + domain.PluginSchemaV1 + "\",\"name\":\"utf8-plugin\"" + tc.extra + "}"
			if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			writeLoaderFile(t, filepath.Join(root, "skills", "healthy", "SKILL.md"), "---\nname: healthy\ndescription: Healthy\n---\n")
			_, err := testLoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
			var loadErr *domain.LoadError
			if !errors.As(err, &loadErr) || loadErr.Diagnostic.Code != "plugin_manifest_malformed" {
				t.Fatalf("invalid manifest accepted or misclassified: %v", err)
			}
		})
	}
}

func TestPortableMCPMalformedUTF8DisablesOnlyMCP(t *testing.T) {
	invalid := string([]byte{0xff})
	for _, tc := range []struct {
		name, brokenEntry string
	}{
		{"server-key", "\"broken" + invalid + "\":{\"type\":\"stdio\",\"command\":\"node\"}"},
		{"server-arg", "\"broken\":{\"type\":\"stdio\",\"command\":\"node\",\"args\":[\"" + invalid + "\"]}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeMinimalPlugin(t, root, "valid-plugin")
			writeLoaderFile(t, filepath.Join(root, "skills", "healthy", "SKILL.md"), "---\nname: healthy\ndescription: Healthy\n---\n")
			body := "{\"$schema\":\"" + domain.MCPSchemaV1 + "\",\"mcpServers\":{\"good\":{\"type\":\"streamable-http\",\"url\":\"https://example.test/mcp\"}," + tc.brokenEntry + "}}"
			if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			envelope, err := testLoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
			if err != nil {
				t.Fatal(err)
			}
			if !envelope.MCP.Present || envelope.MCP.Enabled || len(envelope.MCP.Servers) != 0 || !hasDiagnostic(envelope.Diagnostics, "mcp_malformed") {
				t.Fatalf("malformed MCP must be document-scoped: component=%+v diagnostics=%+v", envelope.MCP, envelope.Diagnostics)
			}
			if _, ok := envelope.Skills["healthy"]; !ok {
				t.Fatalf("independent valid skill disappeared: %+v", envelope.Skills)
			}
		})
	}
}

func TestPortableJSONPreservesValidUnicode(t *testing.T) {
	root := t.TempDir()
	writeLoaderFile(t, filepath.Join(root, "plugin.json"), "{\"$schema\":\""+domain.PluginSchemaV1+"\",\"name\":\"utf8-plugin\",\"description\":\"Пример\",\"extensions\":{\"com.example.future\":{\"note\":\"café\"}}}")
	writeLoaderFile(t, filepath.Join(root, "mcp.json"), "{\"$schema\":\""+domain.MCPSchemaV1+"\",\"mcpServers\":{\"сервис\":{\"type\":\"streamable-http\",\"url\":\"https://example.test/mcp\"}}}")
	envelope, err := testLoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if err != nil || envelope.Manifest.Description != "Пример" || !envelope.MCP.Enabled {
		t.Fatalf("valid UTF-8 rejected: package=%+v error=%v", envelope, err)
	}
	if _, ok := envelope.MCP.Servers["сервис"]; !ok {
		t.Fatalf("Unicode JSON selector was changed: %+v", envelope.MCP.Servers)
	}
}
