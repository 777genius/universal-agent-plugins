package runtimecheck

import (
	"os"
	"runtime"
	"strings"
)

func Inspect(inputs Inputs) (Project, error) {
	root := strings.TrimSpace(inputs.Root)
	if root == "" {
		root = "."
	}
	project := Project{
		Root:    root,
		Targets: append([]string(nil), inputs.Targets...),
		Lane:    laneSummary(inputs.Targets),
	}
	if inputs.Launcher == nil {
		return project, nil
	}
	project.Runtime = strings.TrimSpace(inputs.Launcher.Runtime)
	project.Entrypoint = strings.TrimSpace(inputs.Launcher.Entrypoint)
	if project.Entrypoint != "" {
		project.LauncherPath = launcherPath(root, project.Entrypoint)
		if info, err := os.Stat(project.LauncherPath); err == nil {
			project.LauncherExists = true
			project.LauncherExecutable = runtime.GOOS == "windows" || info.Mode()&0o111 != 0
		}
	}
	switch project.Runtime {
	case "python":
		project.Python = inspectPython(root)
	case "node":
		project.Node = inspectNode(root, project.Entrypoint)
	}
	return project, nil
}
