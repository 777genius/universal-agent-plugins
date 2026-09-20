package app

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func exportRuntimeRequirement(runtime string) string {
	runtime = normalizeExportRuntime(runtime)
	switch runtime {
	case "python":
		return "Python 3.10+ installed on the machine running the plugin"
	case "node":
		return "Node.js 20+ installed on the machine running the plugin"
	case "shell":
		return "POSIX shell on Unix, or bash in PATH on Windows"
	default:
		return ""
	}
}

func exportRuntimeInstallHint(runtime string) string {
	runtime = normalizeExportRuntime(runtime)
	switch runtime {
	case "python":
		return "Go is the recommended path when you want users to avoid installing Python before running the plugin"
	case "node":
		return "Go is the recommended path when you want users to avoid installing Node.js before running the plugin"
	default:
		return ""
	}
}

func exportManager(project runtimecheck.Project) string {
	switch normalizeExportRuntime(project.Runtime) {
	case "python":
		return project.Python.ManagerDisplay()
	case "node":
		return project.Node.ManagerDisplay()
	default:
		return "none"
	}
}

func exportBootstrapModel(project runtimecheck.Project) string {
	switch normalizeExportRuntime(project.Runtime) {
	case "python":
		return project.Python.CanonicalEnvSourceDisplay()
	case "node":
		if project.Node.IsTypeScript {
			return "recipient-side install and build"
		}
		return "recipient-side install"
	case "shell":
		return "launcher plus executable shell scripts"
	default:
		return "n/a"
	}
}

func normalizeExportRuntime(runtime string) string {
	return strings.TrimSpace(runtime)
}
