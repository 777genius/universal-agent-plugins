package validate

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"strings"
)

type validationContext struct {
	root                 string
	rawRequestedPlatform string
	requestedPlatform    string
	manifest             pluginmanifest.Manifest
	graph                pluginmanifest.PackageGraph
	report               Report
}

func newValidationContext(root, platform string, manifest pluginmanifest.Manifest) validationContext {
	return validationContext{
		root:                 root,
		rawRequestedPlatform: platform,
		requestedPlatform:    strings.TrimSpace(platform),
		manifest:             manifest,
		report: Report{
			Platform: strings.Join(manifest.EnabledTargets(), ","),
			Checks:   []string{"plugin_manifest", "package_graph", "publication", "generated_artifacts", "runtime"},
		},
	}
}

func validatePluginProject(root, platform string) (Report, error) {
	manifest, warnings, err := pluginmanifest.LoadWithWarnings(root)
	if err != nil {
		msg := err.Error()
		return Report{}, invalidProjectReport(FailureManifestInvalid, extractFailurePath(msg), msg)
	}

	ctx := newValidationContext(root, platform, manifest)
	ctx.addManifestWarnings(warnings)
	ctx.validateRequestedPlatform()

	graph, _, err := pluginmanifest.Discover(root)
	if err != nil {
		msg := err.Error()
		ctx.addFailure(Failure{
			Kind:    FailureManifestInvalid,
			Path:    extractFailurePath(msg),
			Message: msg,
		})
		return normalizeReport(ctx.report), nil
	}

	ctx.graph = graph
	ctx.validateSourceFiles()
	ctx.validateTargets()
	ctx.validateDrift()
	ctx.validateRuntime()
	return normalizeReport(ctx.report), nil
}
