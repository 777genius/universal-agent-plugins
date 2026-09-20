package validate

import (
	"fmt"
	"os/exec"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func validateNodeRuntime() error {
	path, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("runtime not found: node runtime required; checked PATH for node. Install Node.js 20+")
	}
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("runtime not found: found node at %s but it is not runnable (%v); install or repair Node.js 20+", path, err)
	}
	if err := requireMinVersion("node", string(out), 20, 0); err != nil {
		return fmt.Errorf("runtime not found: found node at %s but %v; install or repair Node.js 20+", path, err)
	}
	return nil
}

func validateNodeRuntimeTarget(root, entrypoint string, report *Report) {
	project, err := runtimecheck.Inspect(runtimecheck.Inputs{
		Root: root,
		Launcher: &pluginmanifest.Launcher{
			Runtime:    "node",
			Entrypoint: entrypoint,
		},
	})
	if err != nil {
		report.Failures = append(report.Failures, Failure{
			Kind:    FailureLauncherInvalid,
			Path:    entrypoint,
			Message: "node runtime inspection failed: " + err.Error(),
		})
		return
	}
	shape := project.Node
	if shape.StructuralIssue != "" {
		report.Failures = append(report.Failures, Failure{
			Kind:    FailureLauncherInvalid,
			Path:    shape.LauncherTarget,
			Message: "node runtime configuration invalid: " + shape.StructuralIssue,
		})
		return
	}
	if shape.RuntimeTargetOK {
		return
	}
	message := "runtime target missing: " + shape.RuntimeTarget
	if shape.UsesBuiltOutput {
		if shape.IsTypeScript {
			message += " (TypeScript scaffold expects built output; run plugin-kit-ai bootstrap . or " + shape.BuildCommandString() + ")"
		} else {
			message += " (launcher points to built output; run plugin-kit-ai bootstrap . or restore the launcher target)"
		}
	} else {
		message += " (restore the generated scaffold target or update the launcher)"
	}
	report.Failures = append(report.Failures, Failure{
		Kind:    FailureRuntimeTargetMissing,
		Path:    shape.RuntimeTarget,
		Message: message,
	})
}
