package app

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

var bootstrapCommandContext = exec.CommandContext

type PluginBootstrapOptions struct {
	Root string
}

type PluginBootstrapResult struct {
	Lines []string
}

func (PluginService) Bootstrap(ctx context.Context, opts PluginBootstrapOptions) (PluginBootstrapResult, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}
	graph, _, err := pluginmanifest.Discover(root)
	if err != nil {
		return PluginBootstrapResult{}, err
	}
	project, err := runtimecheck.Inspect(runtimecheck.Inputs{
		Root:     root,
		Targets:  graph.Manifest.EnabledTargets(),
		Launcher: graph.Launcher,
	})
	if err != nil {
		return PluginBootstrapResult{}, err
	}

	lines := []string{project.ProjectLine()}
	if requirement := exportRuntimeRequirement(project.Runtime); strings.TrimSpace(requirement) != "" {
		lines = append(lines, "Runtime requirement: "+requirement)
	}
	if hint := exportRuntimeInstallHint(project.Runtime); strings.TrimSpace(hint) != "" {
		lines = append(lines, "Runtime install hint: "+hint)
	}
	nextValidate := "Next: " + runtimecheck.ValidateCommand(project.Targets)
	if project.Runtime == "" {
		lines = append(lines,
			fmt.Sprintf("Bootstrap not required for %s: no launcher-based runtime is configured.", project.Lane),
			nextValidate,
		)
		return PluginBootstrapResult{Lines: lines}, nil
	}

	switch project.Runtime {
	case "go":
		lines = append(lines,
			"Bootstrap not required for Go projects: run your normal Go build/test workflow.",
			nextValidate,
		)
		return PluginBootstrapResult{Lines: lines}, nil
	case "shell":
		lines = append(lines,
			"Bootstrap not required for shell runtime projects: ensure the shell target is executable on Unix.",
			nextValidate,
		)
		return PluginBootstrapResult{Lines: lines}, nil
	case "python":
		bootstrapLines, err := bootstrapPython(ctx, project)
		if err != nil {
			return PluginBootstrapResult{}, err
		}
		lines = append(lines, bootstrapLines...)
	case "node":
		bootstrapLines, err := bootstrapNode(ctx, project)
		if err != nil {
			return PluginBootstrapResult{}, err
		}
		lines = append(lines, bootstrapLines...)
	default:
		return PluginBootstrapResult{}, fmt.Errorf("unsupported bootstrap runtime %q", project.Runtime)
	}
	lines = append(lines, nextValidate)
	return PluginBootstrapResult{Lines: lines}, nil
}
