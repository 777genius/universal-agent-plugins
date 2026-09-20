package app

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func loadExportServiceGraph(root, platform string) (pluginmanifest.PackageGraph, error) {
	graph, _, err := pluginmanifest.Discover(root)
	if err != nil {
		return pluginmanifest.PackageGraph{}, err
	}
	if err := validateExportServiceGraph(graph, platform); err != nil {
		return pluginmanifest.PackageGraph{}, err
	}
	return graph, nil
}

func validateExportServiceGraph(graph pluginmanifest.PackageGraph, platform string) error {
	if _, err := graph.Manifest.SelectedTargets(platform); err != nil {
		return err
	}
	if platform != "codex-runtime" && platform != "claude" {
		return fmt.Errorf("export supports only launcher-based interpreted targets: codex-runtime or claude")
	}
	if graph.Launcher == nil {
		return fmt.Errorf("export requires launcher-based target %q with launcher.yaml", platform)
	}
	switch graph.Launcher.Runtime {
	case "python", "node", "shell":
		return nil
	default:
		return fmt.Errorf("export supports only interpreted runtimes (python, node, shell); found %q", graph.Launcher.Runtime)
	}
}
