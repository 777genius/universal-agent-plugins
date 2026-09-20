package app

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

type doctorToolSpec struct {
	Label       string
	Commands    []string
	VersionArgs []string
}

func doctorToolSpecs(root string, project runtimecheck.Project) []doctorToolSpec {
	var specs []doctorToolSpec
	specs = appendDoctorGoToolSpecs(specs, root, project)
	specs = appendDoctorRuntimeToolSpecs(specs, project)
	return specs
}

func appendDoctorGoToolSpecs(specs []doctorToolSpec, root string, project runtimecheck.Project) []doctorToolSpec {
	if fileExists(joinDoctorRoot(root, "go.mod")) || strings.TrimSpace(project.Runtime) == "go" {
		specs = appendDoctorToolSpec(specs,
			doctorToolSpec{Label: "go", Commands: []string{"go"}, VersionArgs: []string{"version"}},
			doctorToolSpec{Label: "gofmt", Commands: []string{"gofmt"}},
		)
	}
	return specs
}

func appendDoctorToolSpec(dst []doctorToolSpec, specs ...doctorToolSpec) []doctorToolSpec {
	for _, spec := range specs {
		if strings.TrimSpace(spec.Label) == "" || len(spec.Commands) == 0 {
			continue
		}
		duplicate := false
		for _, existing := range dst {
			if existing.Label == spec.Label {
				duplicate = true
				break
			}
		}
		if !duplicate {
			dst = append(dst, spec)
		}
	}
	return dst
}
