package app

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestResolveExportServiceInputDefaultsRootAndRequiresPlatform(t *testing.T) {
	t.Parallel()

	input, err := resolveExportServiceInput(PluginExportOptions{Platform: "claude"})
	if err != nil {
		t.Fatalf("resolveExportServiceInput: %v", err)
	}
	if input.root != "." || input.platform != "claude" {
		t.Fatalf("input = %#v", input)
	}
	if _, err := resolveExportServiceInput(PluginExportOptions{}); err == nil {
		t.Fatal("expected missing platform error")
	}
}

func TestBuildExportResultLinesIncludesRuntimeMetadata(t *testing.T) {
	t.Parallel()

	lines := buildExportResultLines(exportServiceContext{
		platform: "claude",
		project:  runtimecheck.Project{Root: "/tmp/out", Runtime: "node"},
	}, exportArchivePlan{
		outputPath: "/tmp/out/demo_claude_node_bundle.tar.gz",
		files:      []string{"plugin/plugin.yaml"},
		metadata: exportMetadata{
			RuntimeRequirement: "Node.js 20+ installed on the machine running the plugin",
			RuntimeInstallHint: "Go is the recommended path when you want users to avoid installing Node.js before running the plugin",
		},
	})
	text := strings.Join(lines, "\n")
	for _, want := range []string{
		"Runtime requirement: Node.js 20+ installed on the machine running the plugin",
		"Runtime install hint: Go is the recommended path when you want users to avoid installing Node.js before running the plugin",
		"tar -xzf demo_claude_node_bundle.tar.gz",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("lines missing %q:\n%s", want, text)
		}
	}
}

func TestBuildExportMetadataIncludesStrictValidateStep(t *testing.T) {
	t.Parallel()

	metadata := buildExportMetadata(exportServiceContext{
		platform: "claude",
		graph: pluginmanifest.PackageGraph{
			Manifest: pluginmanifest.Manifest{Name: "demo"},
			Launcher: &pluginmanifest.Launcher{Runtime: "node"},
		},
		project: runtimecheck.Project{Runtime: "node"},
	})
	text := strings.Join(metadata.Next, "\n")
	if !strings.Contains(text, "plugin-kit-ai validate . --platform claude --strict") {
		t.Fatalf("next steps = %#v", metadata.Next)
	}
	if metadata.PluginName != "demo" || metadata.Runtime != "node" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestValidateExportServiceGraphRejectsMissingLauncher(t *testing.T) {
	t.Parallel()

	err := validateExportServiceGraph(pluginmanifest.PackageGraph{
		Manifest: pluginmanifest.Manifest{
			Name:    "demo",
			Targets: []string{"claude"},
		},
	}, "claude")
	if err == nil || !strings.Contains(err.Error(), "launcher.yaml") {
		t.Fatalf("error = %v", err)
	}
}
