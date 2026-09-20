package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func validatePythonRuntime(root string, targets []string, launcher *pluginmanifest.Launcher) error {
	project, err := runtimecheck.Inspect(runtimecheck.Inputs{
		Root:     root,
		Targets:  targets,
		Launcher: launcher,
	})
	if err != nil {
		return fmt.Errorf("runtime not found: python runtime inspection failed: %v", err)
	}
	diagnosis := runtimecheck.Diagnose(project)
	if diagnosis.Status != runtimecheck.StatusReady {
		return fmt.Errorf("runtime not found: %s. %s", diagnosis.Reason, pythonRecoveryMessage(project.Python))
	}
	if err := requireMinVersion("python", project.Python.VersionOutput, 3, 10); err != nil {
		return fmt.Errorf("runtime not found: found %s interpreter at %s but %v. %s",
			project.Python.ReadySourceDisplay(),
			filepath.ToSlash(project.Python.ReadyInterpreter),
			err,
			pythonRecoveryMessage(project.Python),
		)
	}
	return nil
}

func pythonRecoveryMessage(shape runtimecheck.PythonShape) string {
	message := "Run plugin-kit-ai doctor ., then plugin-kit-ai bootstrap ."
	if fallback := shape.BootstrapFallbackCommand(); strings.TrimSpace(fallback) != "" {
		message += " If needed, fall back to " + fallback + "."
	}
	return message
}
