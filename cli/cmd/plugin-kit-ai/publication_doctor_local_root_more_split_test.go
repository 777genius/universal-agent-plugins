package main

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
)

func TestVerifyPublicationLocalRootResultSkipsWhenVerificationDisabled(t *testing.T) {
	t.Parallel()

	got, ok, err := verifyPublicationLocalRootResult(fakeInspectRunner{}, ".", "gemini", "", "", "ready")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("expected verification to be skipped, got %+v", got)
	}
}

func TestMergePublicationDiagnosisLocalRootIssuesAppendsRootIssues(t *testing.T) {
	t.Parallel()

	got := mergePublicationDiagnosisLocalRootIssues([]publicationIssue{{
		Code: "base",
	}}, "gemini", &app.PluginPublicationVerifyRootResult{
		Issues: []app.PluginPublicationRootIssue{{
			Code:    "missing_root_package",
			Path:    "dist/gemini",
			Message: "package missing",
		}},
	})
	if len(got) != 2 {
		t.Fatalf("issues = %+v", got)
	}
	if got[1].Code != "missing_root_package" || got[1].Path != "dist/gemini" {
		t.Fatalf("issues = %+v", got)
	}
}

func TestShouldMergePublicationLocalRootRejectsNil(t *testing.T) {
	t.Parallel()

	if shouldMergePublicationLocalRoot(nil) {
		t.Fatal("expected nil local root to be skipped")
	}
}
