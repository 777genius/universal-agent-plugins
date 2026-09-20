package main

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func TestDiagnoseMissingPublicationPackageArtifactSkipsUnknownTargets(t *testing.T) {
	t.Parallel()

	if issue, ok := diagnoseMissingPublicationPackageArtifact(t.TempDir(), publicationmodel.Package{Target: "unknown"}); ok {
		t.Fatalf("unexpected issue = %+v", issue)
	}
}

func TestDiagnoseMissingPublicationChannelArtifactReportsExpectedMessage(t *testing.T) {
	t.Parallel()

	issue, ok := diagnoseMissingPublicationChannelArtifact(t.TempDir(), publicationmodel.Channel{Family: "codex-marketplace"})
	if !ok {
		t.Fatal("expected missing artifact issue")
	}
	if !strings.Contains(issue.Message, "channel codex-marketplace is missing generated publication artifact") {
		t.Fatalf("issue = %+v", issue)
	}
}
