package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func bootstrapPython(ctx context.Context, project runtimecheck.Project) ([]string, error) {
	root := project.Root
	shape := project.Python
	lines := []string{fmt.Sprintf("Detected Python manager: %s", shape.ManagerDisplay())}
	switch shape.Manager {
	case runtimecheck.PythonManagerUV:
		if !shape.ManagerAvailable {
			return nil, fmt.Errorf("bootstrap failed: uv not found in PATH")
		}
		if err := runBootstrapCommand(ctx, root, "uv", "sync"); err != nil {
			return nil, err
		}
		lines = append(lines, "Ran: uv sync")
	case runtimecheck.PythonManagerPoetry:
		if !shape.ManagerAvailable {
			return nil, fmt.Errorf("bootstrap failed: poetry not found in PATH")
		}
		if err := runBootstrapCommand(ctx, root, "poetry", "install", "--no-root"); err != nil {
			return nil, err
		}
		lines = append(lines, "Ran: poetry install --no-root")
	case runtimecheck.PythonManagerPipenv:
		if !shape.ManagerAvailable {
			return nil, fmt.Errorf("bootstrap failed: pipenv not found in PATH")
		}
		if fileExists(filepath.Join(root, "Pipfile.lock")) {
			if err := runBootstrapCommand(ctx, root, "pipenv", "sync"); err != nil {
				return nil, err
			}
			lines = append(lines, "Ran: pipenv sync")
		} else {
			if err := runBootstrapCommand(ctx, root, "pipenv", "install"); err != nil {
				return nil, err
			}
			lines = append(lines, "Ran: pipenv install")
		}
	default:
		venvPython, created, err := ensureProjectVenv(ctx, root)
		if err != nil {
			return nil, err
		}
		if created {
			lines = append(lines, "Ran: python -m venv .venv")
		}
		switch shape.Manager {
		case runtimecheck.PythonManagerRequirements:
			if err := runBootstrapCommand(ctx, root, venvPython, "-m", "pip", "install", "-r", "requirements.txt"); err != nil {
				return nil, err
			}
			lines = append(lines, "Ran: python -m pip install -r requirements.txt")
		default:
			if !created {
				lines = append(lines, "Project virtualenv is already available; no dependency bootstrap was required")
			}
		}
	}
	lines = append(lines, "Canonical Python environment source: "+shape.CanonicalEnvSourceDisplay())
	return lines, nil
}

func ensureProjectVenv(ctx context.Context, root string) (string, bool, error) {
	if venvPython := runnableVenvPython(root); venvPython != "" {
		return venvPython, false, nil
	}
	if hasVenv(root) {
		return "", false, fmt.Errorf("bootstrap failed: found .venv but no runnable interpreter; recreate .venv or repair the virtualenv")
	}
	systemPython, err := findSystemPython()
	if err != nil {
		return "", false, err
	}
	if err := runBootstrapCommand(ctx, root, systemPython, "-m", "venv", ".venv"); err != nil {
		return "", false, err
	}
	venvPython := runnableVenvPython(root)
	if venvPython == "" {
		return "", false, fmt.Errorf("bootstrap failed: created .venv but no runnable interpreter was found")
	}
	return venvPython, true, nil
}

func findSystemPython() (string, error) {
	for _, name := range pythonPathNames() {
		if _, err := runtimecheck.LookPath(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("bootstrap failed: python runtime required; install Python 3.10+ or provide python3/python in PATH")
}
