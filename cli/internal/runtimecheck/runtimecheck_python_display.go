package runtimecheck

import "strings"

func (p PythonShape) ManagerDisplay() string {
	switch p.Manager {
	case PythonManagerRequirements:
		return "requirements.txt (pip)"
	case PythonManagerVenv:
		return "venv"
	default:
		return string(p.Manager)
	}
}

func (p PythonShape) ReadySourceDisplay() string {
	switch p.ReadySource {
	case PythonEnvSourceRepoLocal:
		return "repo-local .venv"
	case PythonEnvSourceManagerOwned:
		return "manager-owned env"
	default:
		return "missing env"
	}
}

func (p PythonShape) CanonicalSourceDisplay() string {
	switch p.Manager {
	case PythonManagerPoetry, PythonManagerPipenv:
		return "manager-owned Python"
	default:
		return "project-local Python"
	}
}

func (p PythonShape) CanonicalEnvSourceDisplay() string {
	switch p.Manager {
	case PythonManagerPoetry, PythonManagerPipenv:
		return "manager-owned env"
	default:
		return "repo-local .venv"
	}
}

func (p PythonShape) BootstrapFallbackCommand() string {
	switch p.Manager {
	case PythonManagerUV:
		return "uv sync"
	case PythonManagerPoetry:
		return "poetry install --no-root"
	case PythonManagerPipenv:
		if p.ManifestPath == "Pipfile.lock" {
			return "pipenv sync"
		}
		return "pipenv install"
	case PythonManagerRequirements, PythonManagerVenv:
		if strings.TrimSpace(p.ManagerBinary) != "" {
			return p.ManagerBinary + " -m venv .venv"
		}
	}
	return ""
}
