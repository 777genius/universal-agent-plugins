package runtimecheck

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func parsePyProjectTools(root string) (bool, bool) {
	body, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err != nil {
		return false, false
	}
	var project pyProject
	if err := toml.Unmarshal(body, &project); err != nil {
		return false, false
	}
	_, hasUV := project.Tool["uv"]
	_, hasPoetry := project.Tool["poetry"]
	return hasUV, hasPoetry
}

func probeManagedPythonEnv(root string, manager PythonManager) (string, bool) {
	switch manager {
	case PythonManagerPoetry:
		out, err := RunCommand(root, "poetry", "env", "info", "--path")
		if err != nil {
			return "", false
		}
		return strings.TrimSpace(out), strings.TrimSpace(out) != ""
	case PythonManagerPipenv:
		out, err := RunCommand(root, "pipenv", "--venv")
		if err != nil {
			return "", false
		}
		return strings.TrimSpace(out), strings.TrimSpace(out) != ""
	default:
		return "", false
	}
}
