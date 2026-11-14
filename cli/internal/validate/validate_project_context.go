package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func (ctx *validationContext) addWarning(warning Warning) {
	ctx.report.Warnings = append(ctx.report.Warnings, warning)
}

func (ctx *validationContext) addFailure(failure Failure) {
	ctx.report.Failures = append(ctx.report.Failures, failure)
}

func (ctx *validationContext) addManifestWarnings(warnings []pluginmanifest.Warning) {
	for _, warning := range warnings {
		ctx.addWarning(Warning{
			Kind:    mapManifestWarningKind(warning.Kind),
			Path:    warning.Path,
			Message: warning.Message,
		})
	}
}

func (ctx *validationContext) validateRequestedPlatform() {
	if ctx.requestedPlatform == "" || slices.Contains(ctx.manifest.EnabledTargets(), ctx.requestedPlatform) {
		return
	}
	ctx.addFailure(Failure{
		Kind:    FailureManifestInvalid,
		Path:    authoredProjectPath(ctx.root, pluginmanifest.FileName),
		Message: fmt.Sprintf("plugin.yaml does not enable target %q", ctx.rawRequestedPlatform),
	})
}

func (ctx *validationContext) validateSourceFiles() {
	for _, rel := range ctx.graph.SourceFiles {
		if _, err := os.Stat(filepath.Join(ctx.root, rel)); err != nil {
			ctx.addFailure(Failure{
				Kind:    FailureSourceFileMissing,
				Path:    rel,
				Message: "referenced source file missing: " + rel,
			})
		}
	}
}

func (ctx *validationContext) validateDrift() {
	drift, err := pluginmanifest.Drift(ctx.root, targetOrAll(ctx.requestedPlatform))
	if err != nil {
		msg := err.Error()
		ctx.addFailure(Failure{
			Kind:    FailureGeneratedContractInvalid,
			Path:    extractFailurePath(msg),
			Message: msg,
		})
		return
	}
	for _, rel := range drift {
		ctx.addFailure(Failure{
			Kind:    FailureGeneratedContractInvalid,
			Path:    rel,
			Message: "generated artifact drift: " + rel,
		})
	}
}

func (ctx *validationContext) validateRuntime() {
	validatePluginRuntimeFiles(ctx.root, ctx.manifest, ctx.graph.Launcher, &ctx.report)
}
