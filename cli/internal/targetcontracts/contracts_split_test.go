package targetcontracts

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestByTargetNormalizesInput(t *testing.T) {
	entries := ByTarget("  CODEX-PACKAGE  ")
	if len(entries) != 1 {
		t.Fatalf("ByTarget returned %d entries", len(entries))
	}
	if entries[0].Target != "codex-package" {
		t.Fatalf("ByTarget target = %q", entries[0].Target)
	}
}

func TestAuthoringDocPathCanonicalizesToPluginRoot(t *testing.T) {
	want := filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "package.yaml")
	if got := authoringDocPath(filepath.Join("targets", "gemini", "package.yaml")); got != want {
		t.Fatalf("authoringDocPath without plugin = %q", got)
	}
	if got := authoringDocPath(filepath.Join(pluginmodel.LegacySourceDirName, "targets", "gemini", "package.yaml")); got != want {
		t.Fatalf("authoringDocPath with legacy src = %q", got)
	}
}

func TestTableUsesDashForEmptyCollections(t *testing.T) {
	body := string(Table([]Entry{{
		Target:              "demo",
		PlatformFamily:      "family",
		TargetClass:         "class",
		LauncherRequirement: "required",
		ProductionClass:     "portable",
		RuntimeContract:     "contract",
		Summary:             "summary",
	}}))
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 2 {
		t.Fatalf("table line count = %d", len(lines))
	}
	want := []string{"demo", "family", "class", "required", "portable", "contract", "no", "no", "no", "-", "-", "-", "-", "-", "summary"}
	if got := strings.Fields(lines[1]); !reflect.DeepEqual(got, want) {
		t.Fatalf("table fields = %#v", got)
	}
}
