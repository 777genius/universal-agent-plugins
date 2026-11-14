package app

import (
	"strings"
	"testing"
)

func TestBuildPublicationVerifyRootResultReportsMissingPackageRoot(t *testing.T) {
	t.Parallel()
	ctx := publicationContext{
		root:        ".",
		target:      "claude",
		dest:        "/tmp/out",
		packageRoot: "plugins/demo",
	}
	result := buildPublicationVerifyRootResult(ctx, publicationVerifyPlan{
		catalogRel: ".claude-plugin/marketplace.json",
		issues: []PluginPublicationRootIssue{{
			Code:    "missing_materialized_package_root",
			Path:    "plugins/demo",
			Message: "materialized package root plugins/demo is missing",
		}},
	})
	lines := strings.Join(result.Lines, "\n")
	if !strings.Contains(lines, "Status: needs_sync") {
		t.Fatalf("lines = %v", result.Lines)
	}
	if !strings.Contains(lines, "publication materialize . --target claude --dest /tmp/out") {
		t.Fatalf("lines = %v", result.Lines)
	}
	if result.Ready || result.Status != "needs_sync" || result.IssueCount != 1 {
		t.Fatalf("result = %+v", result)
	}
}
