package app

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestBuildDoctorResultReportsReadyStatus(t *testing.T) {
	t.Parallel()

	result := buildDoctorResult("/tmp/demo", runtimecheck.Project{
		Root:    "/tmp/demo",
		Targets: []string{"claude"},
	})
	if !result.Ready {
		t.Fatal("expected ready result")
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "Status: ready") {
		t.Fatalf("lines = %#v", result.Lines)
	}
}
