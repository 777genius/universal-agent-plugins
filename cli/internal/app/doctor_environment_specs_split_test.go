package app

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestAppendDoctorRuntimeToolSpecsAddsNodeManagerBinary(t *testing.T) {
	t.Parallel()

	specs := appendDoctorRuntimeToolSpecs(nil, runtimecheck.Project{
		Runtime: "node",
		Node:    runtimecheck.NodeShape{ManagerBinary: "pnpm"},
	})
	if len(specs) != 2 || specs[0].Label != "node" || specs[1].Label != "pnpm" {
		t.Fatalf("specs = %#v", specs)
	}
}

func TestAppendDoctorToolSpecDeduplicatesLabels(t *testing.T) {
	t.Parallel()

	specs := appendDoctorToolSpec(nil,
		doctorToolSpec{Label: "go", Commands: []string{"go"}},
		doctorToolSpec{Label: "go", Commands: []string{"go1.22"}},
	)
	if len(specs) != 1 {
		t.Fatalf("specs = %#v", specs)
	}
}
