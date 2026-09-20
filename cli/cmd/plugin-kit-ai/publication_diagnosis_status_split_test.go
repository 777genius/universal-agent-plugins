package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func TestMissingChannelPublicationIssuesSortsTargets(t *testing.T) {
	t.Parallel()

	issues, missingTargets := missingChannelPublicationIssues("plugin", []publicationmodel.Package{
		{Target: "gemini"},
		{Target: "claude"},
	})

	if len(issues) != 2 {
		t.Fatalf("issues = %+v", issues)
	}
	if !reflect.DeepEqual(missingTargets, []string{"claude", "gemini"}) {
		t.Fatalf("missing targets = %+v", missingTargets)
	}
}

func TestAppendPublicationDiagnosisIssuesFormatsLines(t *testing.T) {
	t.Parallel()

	lines := appendPublicationDiagnosisIssues(nil, []publicationIssue{{Code: "missing_channel", Message: "demo"}})
	if len(lines) != 1 || lines[0] != "Issue[missing_channel]: demo" {
		t.Fatalf("lines = %+v", lines)
	}
}

func TestAppendPublicationDiagnosisNextStepsProjectsNextSection(t *testing.T) {
	t.Parallel()

	lines := appendPublicationDiagnosisNextSteps(nil, []string{"step one"})
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Next:") || !strings.Contains(joined, "  step one") {
		t.Fatalf("lines = %+v", lines)
	}
}
