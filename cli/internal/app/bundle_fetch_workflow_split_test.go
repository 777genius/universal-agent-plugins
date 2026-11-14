package app

import (
	"strings"
	"testing"
)

func TestRequireBundleFetchDestRejectsBlank(t *testing.T) {
	t.Parallel()

	err := requireBundleFetchDest(PluginBundleFetchOptions{})
	if err == nil || !strings.Contains(err.Error(), "--dest") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildBundleFetchResultIncludesRuntimeMetadata(t *testing.T) {
	t.Parallel()

	result := buildBundleFetchResult(exportMetadata{
		PluginName:         "demo",
		Platform:           "claude",
		Runtime:            "node",
		Manager:            "npm",
		RuntimeRequirement: "Node.js 20+ installed on the machine running the plugin",
		RuntimeInstallHint: "Go is the recommended path when you want users to avoid installing Node.js before running the plugin",
		Next:               []string{"plugin-kit-ai doctor /tmp/demo"},
	}, bundleRemoteSource{
		BundleSource:   "github release demo/demo@v1.0.0",
		ChecksumSource: "release asset checksums.txt",
	}, "/tmp/demo")

	text := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Bundle: plugin=demo platform=claude runtime=node manager=npm",
		"Runtime requirement: Node.js 20+ installed on the machine running the plugin",
		"Runtime install hint: Go is the recommended path when you want users to avoid installing Node.js before running the plugin",
		"Checksum source: release asset checksums.txt",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("result missing %q:\n%s", want, text)
		}
	}
}

func TestBuildBundleFetchBaseLinesIncludesBundleSource(t *testing.T) {
	t.Parallel()

	lines := buildBundleFetchBaseLines(exportMetadata{
		PluginName: "demo",
		Platform:   "claude",
		Runtime:    "node",
		Manager:    "npm",
	}, bundleRemoteSource{
		BundleSource:   "https://example.com/demo.tar.gz",
		ChecksumSource: "flag --sha256",
	}, "/tmp/demo")

	text := strings.Join(lines, "\n")
	for _, want := range []string{
		"Bundle source: https://example.com/demo.tar.gz",
		"Checksum source: flag --sha256",
		"Installed path: /tmp/demo",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("lines missing %q:\n%s", want, text)
		}
	}
}

func TestValidateBundleFetchWorkflowInputDelegatesModeValidation(t *testing.T) {
	t.Parallel()

	err := validateBundleFetchWorkflowInput(PluginBundleFetchOptions{
		URL:      "https://example.com/demo.tar.gz",
		Dest:     "/tmp/demo",
		Platform: "claude",
	})
	if err == nil || !strings.Contains(err.Error(), "--asset-name, --platform, or --runtime") {
		t.Fatalf("error = %v", err)
	}
}
