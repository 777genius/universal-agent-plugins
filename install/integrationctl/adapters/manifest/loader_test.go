package manifest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestLoadMapsCursorPluginPackageDelivery(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWriteManifestTestFile(t, filepath.Join(root, "plugin", "plugin.yaml"), "api_version: v1\nname: agent-code-navigator\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n")
	mustWriteManifestTestFile(t, filepath.Join(root, ".cursor-plugin", "plugin.json"), `{"name":"agent-code-navigator","skills":"./skills/"}`)

	loaded, err := Loader{}.Load(context.Background(), ports.ResolvedSource{LocalPath: root})
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if len(loaded.Deliveries) != 1 {
		t.Fatalf("deliveries = %#v, want one Cursor delivery", loaded.Deliveries)
	}
	delivery := loaded.Deliveries[0]
	if delivery.TargetID != domain.TargetCursor || delivery.DeliveryKind != domain.DeliveryCursorPlugin {
		t.Fatalf("delivery = %#v, want Cursor plugin delivery", delivery)
	}
}

func TestLoadKeepsLegacyCursorMCPDeliveryWithoutPackage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWriteManifestTestFile(t, filepath.Join(root, "plugin", "plugin.yaml"), "api_version: v1\nname: cursor-mcp-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n")

	loaded, err := Loader{}.Load(context.Background(), ports.ResolvedSource{LocalPath: root})
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if len(loaded.Deliveries) != 1 {
		t.Fatalf("deliveries = %#v, want one Cursor delivery", loaded.Deliveries)
	}
	delivery := loaded.Deliveries[0]
	if delivery.TargetID != domain.TargetCursor || delivery.DeliveryKind != domain.DeliveryCursorMCP {
		t.Fatalf("delivery = %#v, want Cursor MCP delivery", delivery)
	}
}

func TestLoaderRejectsUnsafeIntegrationNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"../escape", "/tmp/escape", `C:\\escape`, "CON", "name..part"} {
		name := name
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			mustWriteManifestTestFile(t, filepath.Join(root, "plugin", "plugin.yaml"), "api_version: v1\nname: "+name+"\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n")
			if _, err := (Loader{}).Load(context.Background(), ports.ResolvedSource{LocalPath: root}); err == nil {
				t.Fatalf("unsafe name %q accepted", name)
			}
		})
	}
}

func mustWriteManifestTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
