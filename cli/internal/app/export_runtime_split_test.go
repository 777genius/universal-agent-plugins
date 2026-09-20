package app

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestExportRuntimeRequirementNormalizesWhitespace(t *testing.T) {
	t.Parallel()

	if got := exportRuntimeRequirement(" node "); got != "Node.js 20+ installed on the machine running the plugin" {
		t.Fatalf("requirement = %q", got)
	}
}

func TestExportBootstrapModelUsesTypeScriptBuildHint(t *testing.T) {
	t.Parallel()

	got := exportBootstrapModel(runtimecheck.Project{
		Runtime: "node",
		Node:    runtimecheck.NodeShape{IsTypeScript: true},
	})
	if got != "recipient-side install and build" {
		t.Fatalf("bootstrap model = %q", got)
	}
}
