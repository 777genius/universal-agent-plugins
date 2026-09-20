package app

import (
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func TestResolveExportArchiveOutputPathUsesManifestAndRuntime(t *testing.T) {
	t.Parallel()

	path := resolveExportArchiveOutputPath(exportServiceContext{
		root:     "/tmp/demo",
		platform: "claude",
		graph: pluginmanifest.PackageGraph{
			Manifest: pluginmanifest.Manifest{Name: "demo"},
			Launcher: &pluginmanifest.Launcher{Runtime: "node"},
		},
	}, "")
	if path != "/tmp/demo/demo_claude_node_bundle.tar.gz" {
		t.Fatalf("path = %q", path)
	}
}

func TestDropExportArchiveOutputRemovesOutputFromFileList(t *testing.T) {
	t.Parallel()

	files := dropExportArchiveOutput([]string{"plugin/plugin.yaml", "demo_claude_node_bundle.tar.gz"}, "/tmp/demo", "/tmp/demo/demo_claude_node_bundle.tar.gz")
	if slices.Contains(files, "demo_claude_node_bundle.tar.gz") {
		t.Fatalf("files = %#v", files)
	}
}
