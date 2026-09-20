package runtimecheck

import "strings"

func detectPythonManager(root string, hasUV, hasPoetry bool) PythonShape {
	switch {
	case fileExists(joinRoot(root, "uv.lock")) || hasUV:
		return PythonShape{
			Manager:          PythonManagerUV,
			ManagerBinary:    "uv",
			ManifestPath:     firstExisting(root, "uv.lock", "pyproject.toml"),
			ManagerAvailable: lookupBinary("uv"),
		}
	case fileExists(joinRoot(root, "poetry.lock")) || hasPoetry:
		return PythonShape{
			Manager:          PythonManagerPoetry,
			ManagerBinary:    "poetry",
			ManifestPath:     firstExisting(root, "poetry.lock", "pyproject.toml"),
			ManagerAvailable: lookupBinary("poetry"),
		}
	case fileExists(joinRoot(root, "Pipfile.lock")) || fileExists(joinRoot(root, "Pipfile")):
		return PythonShape{
			Manager:          PythonManagerPipenv,
			ManagerBinary:    "pipenv",
			ManifestPath:     firstExisting(root, "Pipfile.lock", "Pipfile"),
			ManagerAvailable: lookupBinary("pipenv"),
		}
	case fileExists(joinRoot(root, "requirements.txt")):
		binary := firstAvailableBinary(pythonPathNames())
		return PythonShape{
			Manager:          PythonManagerRequirements,
			ManagerBinary:    binary,
			ManifestPath:     "requirements.txt",
			ManagerAvailable: binary != "",
		}
	default:
		binary := firstAvailableBinary(pythonPathNames())
		return PythonShape{
			Manager:          PythonManagerVenv,
			ManagerBinary:    binary,
			ManagerAvailable: binary != "",
		}
	}
}

func resolvePythonEnvironment(root string, shape PythonShape) PythonShape {
	shape.HasVenv = hasVenv(root)
	shape.VenvPath = pythonInterpreter(root)
	if shape.VenvPath != "" {
		if version, err := pythonVersion(root, shape.VenvPath); err == nil {
			shape.VenvRunnable = true
			shape.ReadySource = PythonEnvSourceRepoLocal
			shape.ReadyInterpreter = shape.VenvPath
			shape.VersionOutput = version
			return shape
		}
		shape.BrokenReason = "found .venv but no runnable interpreter; recreate .venv or repair the virtualenv"
		shape.ReadySource = PythonEnvSourceBroken
		return shape
	}
	if shape.HasVenv {
		shape.BrokenReason = "found .venv but no runnable interpreter; recreate .venv or repair the virtualenv"
		shape.ReadySource = PythonEnvSourceBroken
		return shape
	}
	switch shape.Manager {
	case PythonManagerPoetry, PythonManagerPipenv:
		shape.ProbeAttempted = shape.ManagerAvailable
		if shape.ManagerAvailable {
			envRoot, ok := probeManagedPythonEnv(root, shape.Manager)
			shape.ProbeAvailable = ok
			if ok {
				shape.ProbedEnvPath = envRoot
				interpreter := pythonInterpreterInEnv(envRoot)
				if interpreter == "" {
					shape.BrokenReason = managerOwnedPythonBrokenReason(shape, envRoot, false)
					shape.ReadySource = PythonEnvSourceBroken
					return shape
				}
				version, err := pythonVersion(root, interpreter)
				if err != nil {
					shape.BrokenReason = managerOwnedPythonBrokenReason(shape, envRoot, true)
					shape.ReadySource = PythonEnvSourceBroken
					return shape
				}
				shape.ReadySource = PythonEnvSourceManagerOwned
				shape.ReadyInterpreter = interpreter
				shape.VersionOutput = version
				return shape
			}
		}
	}
	if strings.TrimSpace(string(shape.ReadySource)) == "" {
		shape.ReadySource = PythonEnvSourceMissing
	}
	return shape
}

func managerOwnedPythonBrokenReason(shape PythonShape, envRoot string, hasInterpreter bool) string {
	if !hasInterpreter {
		return shape.ManagerDisplay() + " reported env " + filepathToSlash(envRoot) + " but no runnable interpreter was found"
	}
	return shape.ManagerDisplay() + " reported env " + filepathToSlash(envRoot) + " but its interpreter is not runnable"
}
