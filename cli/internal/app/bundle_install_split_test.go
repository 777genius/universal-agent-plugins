package app

import (
	"strings"
	"testing"
)

func TestResolveBundleInstallInputRejectsRemoteURL(t *testing.T) {
	t.Parallel()

	_, err := resolveBundleInstallInput(PluginBundleInstallOptions{
		Archive: "https://example.com/demo.tar.gz",
		Dest:    "/tmp/demo",
	})
	if err == nil || !strings.Contains(err.Error(), "remote URLs") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildBundleInstallResultIncludesRuntimeMetadata(t *testing.T) {
	t.Parallel()

	result := buildBundleInstallResult(exportMetadata{
		PluginName:         "demo",
		Platform:           "codex-runtime",
		Runtime:            "python",
		Manager:            "requirements.txt (pip)",
		RuntimeRequirement: "Python 3.10+ installed on the machine running the plugin",
		RuntimeInstallHint: "Go is the recommended path when you want users to avoid installing Python before running the plugin",
		Next:               []string{"plugin-kit-ai doctor ."},
	}, "/tmp/demo.tar.gz", "/tmp/demo")

	text := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Bundle source: /tmp/demo.tar.gz",
		"Installed path: /tmp/demo",
		"Runtime requirement: Python 3.10+ installed on the machine running the plugin",
		"Runtime install hint: Go is the recommended path when you want users to avoid installing Python before running the plugin",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("result missing %q:\n%s", want, text)
		}
	}
}
