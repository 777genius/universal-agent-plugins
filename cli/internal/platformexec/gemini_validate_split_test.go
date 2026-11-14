package platformexec

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestValidateGeminiDirNameReportsBasenameMismatch(t *testing.T) {
	t.Parallel()

	diagnostics := validateGeminiDirName(t.TempDir(), "demo-extension")
	if text := diagnosticsText(diagnostics); !strings.Contains(text, "does not match extension name") {
		t.Fatalf("diagnostics missing dir-name mismatch:\n%s", text)
	}
}

func TestReadGeminiGeneratedExtensionReportsUnreadableManifest(t *testing.T) {
	t.Parallel()

	_, ok, diagnostics := readGeminiGeneratedExtension(t.TempDir())
	if ok {
		t.Fatal("expected unreadable extension")
	}
	if text := diagnosticsText(diagnostics); !strings.Contains(text, "is not readable") {
		t.Fatalf("diagnostics missing unreadable manifest failure:\n%s", text)
	}
}

func TestLoadGeminiValidateMetaPreservesPackageMetadataPath(t *testing.T) {
	t.Parallel()

	state := pluginmodel.NewTargetState("gemini")
	state.SetDoc("package_metadata", filepath.Join("targets", "gemini", "package.yaml"))

	_, err := loadGeminiValidateMeta(t.TempDir(), state)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "parse targets/gemini/package.yaml:") {
		t.Fatalf("error = %v", err)
	}
}
