package runtimecheck

import "fmt"

func diagnosePython(project Project, nextValidate string) Diagnosis {
	shape := project.Python
	if shape.BrokenReason != "" {
		return Diagnosis{
			Status: StatusBlocked,
			Reason: shape.BrokenReason,
			Next:   []string{"plugin-kit-ai bootstrap .", nextValidate},
		}
	}
	if shape.ReadySource != PythonEnvSourceMissing {
		return Diagnosis{
			Status: StatusReady,
			Reason: fmt.Sprintf("Python runtime is ready via %s using %s", shape.ManagerDisplay(), shape.ReadySourceDisplay()),
			Next:   []string{nextValidate},
		}
	}
	if !shape.ManagerAvailable {
		return Diagnosis{
			Status: StatusBlocked,
			Reason: fmt.Sprintf("%s not found in PATH", shape.ManagerBinary),
			Next:   []string{"plugin-kit-ai bootstrap .", nextValidate},
		}
	}
	return Diagnosis{
		Status: StatusNeedsBootstrap,
		Reason: fmt.Sprintf("%s environment is not ready", shape.CanonicalSourceDisplay()),
		Next:   []string{"plugin-kit-ai bootstrap .", nextValidate},
	}
}

func inspectPython(root string) PythonShape {
	hasUV, hasPoetry := parsePyProjectTools(root)
	shape := detectPythonManager(root, hasUV, hasPoetry)
	return resolvePythonEnvironment(root, shape)
}
