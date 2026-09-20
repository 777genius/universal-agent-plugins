package main

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
)

func TestPublicationDoctorTextLinesAppendsLocalRootLines(t *testing.T) {
	t.Parallel()

	got := publicationDoctorTextLines(publicationDiagnosis{
		Lines: []string{"status"},
	}, &app.PluginPublicationVerifyRootResult{
		Lines: []string{"local-root"},
	})
	if len(got) != 2 || got[0] != "status" || got[1] != "local-root" {
		t.Fatalf("lines = %+v", got)
	}
}

func TestAppendPublicationDoctorLocalRootLinesSkipsNilRoot(t *testing.T) {
	t.Parallel()

	got := appendPublicationDoctorLocalRootLines([]string{"status"}, nil)
	if len(got) != 1 || got[0] != "status" {
		t.Fatalf("lines = %+v", got)
	}
}

func TestPublicationDoctorRendererForFormatRejectsUnknownValues(t *testing.T) {
	t.Parallel()

	if _, err := publicationDoctorRendererForFormat("yaml"); err == nil {
		t.Fatal("expected unsupported format error")
	}
}
