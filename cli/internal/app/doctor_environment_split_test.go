package app

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestDoctorToolSpecsIncludePythonManager(t *testing.T) {
	t.Parallel()

	specs := doctorToolSpecs("", runtimecheck.Project{
		Runtime: "python",
		Python:  runtimecheck.PythonShape{Manager: runtimecheck.PythonManagerPoetry},
	})
	if len(specs) != 2 {
		t.Fatalf("specs = %#v", specs)
	}
	if specs[0].Label != "python runtime" || specs[1].Label != "poetry" {
		t.Fatalf("specs = %#v", specs)
	}
}

func TestBuildDoctorEnvironmentLinesAddsHintWhenToolMissing(t *testing.T) {
	// This fixture replaces a package global and must not run in parallel.

	restoreLookPath := runtimecheck.LookPath
	runtimecheck.LookPath = func(name string) (string, error) {
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() {
		runtimecheck.LookPath = restoreLookPath
	})

	lines := buildDoctorEnvironmentLines(".", []doctorToolSpec{{
		Label:    "node",
		Commands: []string{"node"},
	}})
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "node: missing from PATH") || !strings.Contains(text, "Hint:") {
		t.Fatalf("lines = %v", lines)
	}
}
