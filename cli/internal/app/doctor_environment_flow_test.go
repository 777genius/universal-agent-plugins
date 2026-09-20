package app

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestLoadDoctorEnvironmentSpecsUsesProjectRuntime(t *testing.T) {
	t.Parallel()

	specs := loadDoctorEnvironmentSpecs("", runtimecheck.Project{
		Runtime: "python",
		Python:  runtimecheck.PythonShape{Manager: runtimecheck.PythonManagerPoetry},
	})
	if len(specs) != 2 {
		t.Fatalf("specs = %#v", specs)
	}
}

func TestRenderDoctorEnvironmentDelegatesToLineBuilder(t *testing.T) {
	t.Parallel()

	lines := renderDoctorEnvironment(".", nil)
	if lines != nil {
		t.Fatalf("lines = %#v", lines)
	}
}
