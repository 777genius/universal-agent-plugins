package app

import (
	"strings"
	"testing"
)

func TestBuildBundlePublishArtifactUsesCanonicalNames(t *testing.T) {
	t.Parallel()

	artifact := buildBundlePublishArtifact(exportMetadata{
		PluginName: "demo",
		Platform:   "codex-runtime",
		Runtime:    "python",
	}, []byte("bundle"))
	if artifact.BundleName != "demo_codex-runtime_python_bundle.tar.gz" {
		t.Fatalf("bundle name = %q", artifact.BundleName)
	}
	if !strings.Contains(string(artifact.SidecarBody), artifact.BundleName) {
		t.Fatalf("sidecar body = %q", artifact.SidecarBody)
	}
}
