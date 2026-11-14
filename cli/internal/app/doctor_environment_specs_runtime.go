package app

import (
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func appendDoctorRuntimeToolSpecs(specs []doctorToolSpec, project runtimecheck.Project) []doctorToolSpec {
	switch strings.TrimSpace(project.Runtime) {
	case "python":
		return appendDoctorPythonToolSpecs(specs, project)
	case "node":
		return appendDoctorNodeToolSpecs(specs, project)
	case "shell":
		return appendDoctorShellToolSpecs(specs)
	default:
		return specs
	}
}

func appendDoctorPythonToolSpecs(specs []doctorToolSpec, project runtimecheck.Project) []doctorToolSpec {
	specs = appendDoctorToolSpec(specs, doctorToolSpec{
		Label:       "python runtime",
		Commands:    pythonPathNames(),
		VersionArgs: []string{"--version"},
	})
	switch project.Python.Manager {
	case runtimecheck.PythonManagerUV:
		specs = appendDoctorToolSpec(specs, doctorToolSpec{Label: "uv", Commands: []string{"uv"}, VersionArgs: []string{"--version"}})
	case runtimecheck.PythonManagerPoetry:
		specs = appendDoctorToolSpec(specs, doctorToolSpec{Label: "poetry", Commands: []string{"poetry"}, VersionArgs: []string{"--version"}})
	case runtimecheck.PythonManagerPipenv:
		specs = appendDoctorToolSpec(specs, doctorToolSpec{Label: "pipenv", Commands: []string{"pipenv"}, VersionArgs: []string{"--version"}})
	}
	return specs
}

func appendDoctorNodeToolSpecs(specs []doctorToolSpec, project runtimecheck.Project) []doctorToolSpec {
	specs = appendDoctorToolSpec(specs, doctorToolSpec{
		Label:       "node",
		Commands:    []string{"node"},
		VersionArgs: []string{"--version"},
	})
	manager := strings.TrimSpace(project.Node.ManagerBinary)
	if manager != "" && manager != "node" {
		specs = appendDoctorToolSpec(specs, doctorToolSpec{
			Label:       manager,
			Commands:    []string{manager},
			VersionArgs: []string{"--version"},
		})
	}
	return specs
}

func appendDoctorShellToolSpecs(specs []doctorToolSpec) []doctorToolSpec {
	if runtime.GOOS == "windows" {
		specs = appendDoctorToolSpec(specs, doctorToolSpec{Label: "bash", Commands: []string{"bash"}, VersionArgs: []string{"--version"}})
	}
	return specs
}
