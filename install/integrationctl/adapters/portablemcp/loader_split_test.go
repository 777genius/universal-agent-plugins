package portablemcp

import (
	"context"
	"path/filepath"
	"testing"

	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestLoadForTargetPrefersAuthoredLayoutAndFiltersTargets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePortableMCPTestFile(t, filepath.Join(root, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  docs:\n    type: remote\n    targets: [codex, opencode, codex]\n    remote:\n      protocol: STREAMABLE_HTTP\n      url: https://example.com/mcp\n  gemini-only:\n    type: stdio\n    targets: [gemini]\n    stdio:\n      command: node\n")
	writePortableMCPTestFile(t, filepath.Join(root, "plugin", "plugin.yaml"), "api_version: v1\nname: demo\nversion: 0.1.0\ndescription: demo\ntargets:\n  - codex-package\n")
	writePortableMCPTestFile(t, filepath.Join(root, "mcp", "servers.yaml"), "api_version: v1\nservers:\n  fallback:\n    type: stdio\n    stdio:\n      command: node\n")

	loaded, err := Loader{FS: fsadapter.OS{}}.LoadForTarget(context.Background(), root, domain.TargetCodex)
	if err != nil {
		t.Fatalf("LoadForTarget error = %v", err)
	}
	if loaded.Path != filepath.Join(root, "plugin", "mcp", "servers.yaml") {
		t.Fatalf("path = %q", loaded.Path)
	}
	if len(loaded.Servers) != 1 {
		t.Fatalf("servers len = %d", len(loaded.Servers))
	}
	server := loaded.Servers["docs"]
	if server.Remote == nil || server.Remote.Protocol != "streamable_http" {
		t.Fatalf("server remote = %#v", server.Remote)
	}
	if len(server.Targets) != 2 || server.Targets[0] != "codex" || server.Targets[1] != "opencode" {
		t.Fatalf("targets = %#v", server.Targets)
	}
}

func TestLoadForTargetSupportsLegacySrcLayout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePortableMCPTestFile(t, filepath.Join(root, "src", "plugin.yaml"), "api_version: v1\nname: demo\nversion: 0.1.0\ndescription: demo\ntargets:\n  - codex-package\n")
	writePortableMCPTestFile(t, filepath.Join(root, "src", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  docs:\n    type: remote\n    remote:\n      protocol: STREAMABLE_HTTP\n      url: https://example.com/mcp\n")

	loaded, err := Loader{FS: fsadapter.OS{}}.LoadForTarget(context.Background(), root, domain.TargetCodex)
	if err != nil {
		t.Fatalf("LoadForTarget error = %v", err)
	}
	if loaded.Path != filepath.Join(root, "src", "mcp", "servers.yaml") {
		t.Fatalf("path = %q", loaded.Path)
	}
	if len(loaded.Servers) != 1 {
		t.Fatalf("servers len = %d", len(loaded.Servers))
	}
}

func TestLoadForTargetRejectsInvalidAlias(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePortableMCPTestFile(t, filepath.Join(root, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  Invalid_Alias:\n    type: stdio\n    stdio:\n      command: node\n")

	_, err := Loader{FS: fsadapter.OS{}}.LoadForTarget(context.Background(), root, domain.TargetCodex)
	if err == nil || err.Error() != "portable MCP alias \"Invalid_Alias\" is invalid" {
		t.Fatalf("err = %v", err)
	}
}

func writePortableMCPTestFile(t *testing.T, path, body string) {
	t.Helper()
	fs := fsadapter.OS{}
	if err := fs.MkdirAll(context.Background(), filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := fs.WriteFileAtomic(context.Background(), path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
