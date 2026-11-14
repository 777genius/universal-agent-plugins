package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

var devCommandContext = exec.CommandContext

func devAutoBuild(ctx context.Context, root string, graph pluginmanifest.PackageGraph, project runtimecheck.Project) ([]string, error) {
	switch project.Runtime {
	case "go":
		return devBuildGo(ctx, root, graph)
	case "node":
		if strings.TrimSpace(project.Node.BuildScript) != "" {
			return devBuildNode(ctx, root, project.Node)
		}
	}
	return nil, nil
}

func devBuildGo(ctx context.Context, root string, graph pluginmanifest.PackageGraph) ([]string, error) {
	entrypoint := strings.TrimSpace(graph.Launcher.Entrypoint)
	if entrypoint == "" {
		return nil, fmt.Errorf("build requires launcher entrypoint")
	}
	output := filepath.Join(root, strings.TrimPrefix(filepath.Clean(entrypoint), "./"))
	target := filepath.Join(".", "cmd", graph.Manifest.Name)
	if _, err := os.Stat(filepath.Join(root, "cmd", graph.Manifest.Name)); err != nil {
		return nil, fmt.Errorf("build requires %s for automatic Go rebuild", filepath.ToSlash(filepath.Join("cmd", graph.Manifest.Name)))
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(output), ".exe") {
		output += ".exe"
	}
	cmd := devCommandContext(ctx, "go", "build", "-o", output, target)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		return []string{"Build: go build -o " + filepath.ToSlash(output) + " " + filepath.ToSlash(target)}, fmt.Errorf("go build failed: %v\n%s", err, out)
	}
	return []string{"Build: go build -o " + filepath.ToSlash(output) + " " + filepath.ToSlash(target)}, nil
}

func devBuildNode(ctx context.Context, root string, shape runtimecheck.NodeShape) ([]string, error) {
	args := buildCommandArgs(shape.Manager)
	cmd := devCommandContext(ctx, shape.ManagerBinary, args...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		return []string{"Build: " + shape.BuildCommandString()}, fmt.Errorf("node build failed: %v\n%s", err, out)
	}
	return []string{"Build: " + shape.BuildCommandString()}, nil
}
