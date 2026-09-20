package app

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
	"github.com/777genius/plugin-kit-ai/cli/internal/validate"
)

func TestBuildExportServiceResultUsesRenderedLines(t *testing.T) {
	t.Parallel()

	result := buildExportServiceResult(exportServiceContext{
		platform: "claude",
		project:  runtimecheck.Project{Root: "/tmp/demo", Runtime: "node"},
	}, exportArchivePlan{
		outputPath: "/tmp/demo/demo_claude_node_bundle.tar.gz",
		files:      []string{"plugin/plugin.yaml"},
		metadata: exportMetadata{
			RuntimeRequirement: "Node.js 20+ installed on the machine running the plugin",
		},
	})
	if !strings.Contains(strings.Join(result.Lines, "\n"), "Exported bundle: /tmp/demo/demo_claude_node_bundle.tar.gz") {
		t.Fatalf("lines = %#v", result.Lines)
	}
}

func TestExportBlockingFailuresPreservesNonGeneratedContractFailures(t *testing.T) {
	t.Parallel()

	failures := exportBlockingFailures([]validate.Failure{
		{Kind: validate.FailureGeneratedContractInvalid},
		{Kind: validate.FailureLauncherInvalid},
	})
	if len(failures) != 1 || failures[0].Kind != validate.FailureLauncherInvalid {
		t.Fatalf("failures = %#v", failures)
	}
}
