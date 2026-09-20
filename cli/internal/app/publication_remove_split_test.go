package app

import (
	"strings"
	"testing"
)

func TestBuildPublicationRemoveResultReportsMissingPackageRoot(t *testing.T) {
	t.Parallel()

	result := buildPublicationRemoveResult(publicationContext{
		root:        ".",
		target:      "claude",
		dest:        "/tmp/out",
		packageRoot: "plugins/demo",
		graph:       publicationGraphStub("demo"),
		channel:     publicationChannelStub("claude-marketplace"),
	}, publicationRemovePlan{
		catalogRel: "catalog.json",
	}, true)
	if !strings.Contains(strings.Join(result.Lines, "\n"), "no existing package root") {
		t.Fatalf("lines = %v", result.Lines)
	}
}
