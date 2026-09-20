package app

import "github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"

func loadDoctorEnvironmentSpecs(root string, project runtimecheck.Project) []doctorToolSpec {
	return doctorToolSpecs(root, project)
}

func renderDoctorEnvironment(root string, specs []doctorToolSpec) []string {
	return buildDoctorEnvironmentLines(root, specs)
}
