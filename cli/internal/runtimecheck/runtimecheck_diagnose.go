package runtimecheck

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

func Diagnose(project Project) Diagnosis {
	nextValidate := validateCommand(project.Targets)
	if project.Runtime == "" {
		return Diagnosis{
			Status: StatusReady,
			Reason: "no launcher-based runtime configured",
			Next:   []string{nextValidate},
		}
	}
	if !project.LauncherExists {
		return Diagnosis{
			Status: StatusBlocked,
			Reason: fmt.Sprintf("launcher entrypoint %s is missing", strings.TrimSpace(project.Entrypoint)),
			Next:   []string{nextValidate},
		}
	}
	switch project.Runtime {
	case "go":
		return Diagnosis{
			Status: StatusReady,
			Reason: "Go runtime is configured",
			Next:   []string{nextValidate},
		}
	case "shell":
		return diagnoseShell(project, nextValidate)
	case "python":
		return diagnosePython(project, nextValidate)
	case "node":
		return diagnoseNode(project, nextValidate)
	default:
		return Diagnosis{
			Status: StatusBlocked,
			Reason: fmt.Sprintf("unsupported runtime %q", project.Runtime),
			Next:   []string{nextValidate},
		}
	}
}

func diagnoseShell(project Project, nextValidate string) Diagnosis {
	if runtime.GOOS != "windows" && !project.LauncherExecutable {
		return Diagnosis{
			Status: StatusBlocked,
			Reason: fmt.Sprintf("launcher %s is not executable", filepath.ToSlash(project.LauncherPath)),
			Next:   []string{nextValidate},
		}
	}
	return Diagnosis{
		Status: StatusReady,
		Reason: "shell launcher is present",
		Next:   []string{nextValidate},
	}
}
