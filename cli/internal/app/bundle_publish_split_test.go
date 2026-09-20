package app

import (
	"strings"
	"testing"
)

func TestResolveBundlePublishInputDefaultsRoot(t *testing.T) {
	t.Parallel()

	input, err := resolveBundlePublishInput(PluginBundlePublishOptions{
		Platform: "codex-runtime",
		Repo:     "o/r",
		Tag:      "v1",
	}, bundlePublishDeps{
		GitHub: &fakeBundlePublisher{},
		Export: func(PluginExportOptions) (PluginExportResult, error) { return PluginExportResult{}, nil },
	})
	if err != nil {
		t.Fatalf("resolveBundlePublishInput: %v", err)
	}
	if input.root != "." || input.owner != "o" || input.repo != "r" {
		t.Fatalf("input = %#v", input)
	}
}

func TestBuildBundlePublishResultIncludesFetchAndInstallNextSteps(t *testing.T) {
	t.Parallel()

	result := buildBundlePublishResult(bundlePublishInput{
		ref:   "o/r",
		tag:   "v1",
		owner: "o",
		repo:  "r",
	}, bundlePublishArtifact{
		Metadata: exportMetadata{
			PluginName: "demo",
			Platform:   "codex-runtime",
			Runtime:    "python",
			Manager:    "requirements",
		},
		BundleName:  "demo_codex-runtime_python_bundle.tar.gz",
		SidecarName: "demo_codex-runtime_python_bundle.tar.gz.sha256",
	}, nil, "created published release")

	text := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Release: o/r@v1",
		"plugin-kit-ai bundle fetch o/r --tag v1 --platform codex-runtime --runtime python --dest <path>",
		"plugin-kit-ai bundle install ./demo_codex-runtime_python_bundle.tar.gz --dest <path>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("result missing %q:\n%s", want, text)
		}
	}
}
