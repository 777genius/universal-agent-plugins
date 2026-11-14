package main

import "testing"

func TestPublicationRootDefaultsToDot(t *testing.T) {
	t.Parallel()

	if got := publicationRoot(nil); got != "." {
		t.Fatalf("root = %q", got)
	}
}

func TestNormalizedPublicationReportFormatRejectsUnknownValues(t *testing.T) {
	t.Parallel()

	if got := normalizedPublicationReportFormat("yaml"); got != "invalid" {
		t.Fatalf("format = %q", got)
	}
}
