package app

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestBuildExportServiceContextPreservesInputAndDeps(t *testing.T) {
	t.Parallel()

	input := exportServiceInput{root: "/tmp/demo", platform: "claude"}
	graph := pluginmanifest.PackageGraph{Manifest: pluginmanifest.Manifest{Name: "demo"}}
	project := runtimecheck.Project{Root: "/tmp/demo", Runtime: "node"}
	ctx := buildExportServiceContext(input, graph, project)
	if ctx.root != input.root || ctx.platform != input.platform {
		t.Fatalf("context = %#v", ctx)
	}
	if ctx.graph.Manifest.Name != "demo" || ctx.project.Runtime != "node" {
		t.Fatalf("context = %#v", ctx)
	}
}
