package app

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestBuildDoctorEnvironmentDetailLinesMarksMissingTools(t *testing.T) {
	// This fixture replaces a package global and must not run in parallel.

	restoreLookPath := runtimecheck.LookPath
	runtimecheck.LookPath = func(name string) (string, error) {
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { runtimecheck.LookPath = restoreLookPath })

	lines, missing := buildDoctorEnvironmentDetailLines(".", []doctorToolSpec{{
		Label:    "node",
		Commands: []string{"node"},
	}})
	if !missing || !strings.Contains(strings.Join(lines, "\n"), "node: missing from PATH") {
		t.Fatalf("lines = %#v missing=%v", lines, missing)
	}
}

func TestAppendDoctorEnvironmentHintNoopsWithoutMissing(t *testing.T) {
	t.Parallel()

	lines := appendDoctorEnvironmentHint([]string{"Environment:"}, false)
	if len(lines) != 1 {
		t.Fatalf("lines = %#v", lines)
	}
}
