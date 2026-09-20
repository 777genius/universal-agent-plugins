package validate

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/platformexec"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/targetcontracts"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func (ctx *validationContext) validateTargets() {
	for _, targetName := range ctx.manifest.EnabledTargets() {
		ctx.validateTarget(targetName)
	}
}

func (ctx *validationContext) validateTarget(targetName string) {
	ctx.validateForbiddenFiles(targetName)

	entry, ok := targetcontracts.Lookup(targetName)
	if !ok {
		return
	}
	profile, _ := platformmeta.Lookup(targetName)
	tc := ctx.graph.Targets[targetName]
	ctx.validatePortableKinds(targetName, entry.PortableComponentKinds)
	ctx.report.Failures = append(ctx.report.Failures, validatePortableContractCoverage(targetName, profile, ctx.graph)...)
	ctx.validateNativeKinds(targetName, tc, entry.TargetComponentKinds)

	validateTargetExtraDocs(ctx.root, targetName, tc, &ctx.report)
	validateUnsupportedTargetSurfaces(ctx.root, targetName, &ctx.report)
	if adapter, ok := platformexec.Lookup(targetName); ok {
		ctx.applyTargetDiagnostics(targetName, tc, adapter)
	}
}

func (ctx *validationContext) validateForbiddenFiles(targetName string) {
	rule, ok := LookupRule(targetName)
	if !ok {
		return
	}
	for _, rel := range rule.ForbiddenFiles {
		if !fileExists(filepath.Join(ctx.root, rel)) {
			continue
		}
		ctx.addFailure(Failure{
			Kind:    FailureForbiddenFilePresent,
			Path:    rel,
			Target:  targetName,
			Message: fmt.Sprintf("target %s forbids %s", targetName, rel),
		})
	}
}

func (ctx *validationContext) validatePortableKinds(targetName string, kinds []string) {
	supportedPortable := setOf(kinds)
	if len(ctx.graph.Portable.Paths("skills")) > 0 && !supportedPortable["skills"] {
		ctx.addFailure(Failure{
			Kind:    FailureUnsupportedTargetKind,
			Path:    unsupportedPortablePath(ctx.graph.Portable, "skills"),
			Target:  targetName,
			Message: fmt.Sprintf("target %s does not support portable component kind skills", targetName),
		})
	}
	if ctx.graph.Portable.MCP != nil && !supportedPortable["mcp_servers"] {
		ctx.addFailure(Failure{
			Kind:    FailureUnsupportedTargetKind,
			Path:    unsupportedPortablePath(ctx.graph.Portable, "mcp_servers"),
			Target:  targetName,
			Message: fmt.Sprintf("target %s does not support portable component kind mcp_servers", targetName),
		})
	}
}

func (ctx *validationContext) validateNativeKinds(targetName string, tc pluginmanifest.TargetComponents, kinds []string) {
	supportedNative := setOf(kinds)
	for _, kind := range pluginmanifest.DiscoveredTargetKinds(tc) {
		if supportedNative[kind] {
			continue
		}
		ctx.addFailure(Failure{
			Kind:    FailureUnsupportedTargetKind,
			Path:    unsupportedTargetKindPath(targetName, tc, kind),
			Target:  targetName,
			Message: fmt.Sprintf("target %s does not support target-native component kind %s", targetName, kind),
		})
	}
}

func (ctx *validationContext) applyTargetDiagnostics(targetName string, tc pluginmanifest.TargetComponents, adapter platformexec.Adapter) {
	diagnostics, err := adapter.Validate(ctx.root, ctx.graph, tc)
	if err != nil {
		msg := err.Error()
		ctx.addFailure(Failure{
			Kind:    FailureManifestInvalid,
			Path:    extractFailurePath(msg),
			Target:  targetName,
			Message: msg,
		})
		return
	}
	applyAdapterDiagnostics(&ctx.report, diagnostics)
}
