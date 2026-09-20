package app

import (
	"fmt"
	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
	"strings"
)

func resolveInitConfig(name string, opts InitOptions) (resolvedInitConfig, error) {
	templateName := scaffold.NormalizeTemplate(opts.Template)
	if !scaffold.IsKnownTemplate(templateName) {
		return resolvedInitConfig{}, fmt.Errorf("unknown template %q", opts.Template)
	}
	config := resolvedInitConfig{
		TemplateName:          templateName,
		Platform:              strings.ToLower(strings.TrimSpace(opts.Platform)),
		Runtime:               strings.ToLower(strings.TrimSpace(opts.Runtime)),
		RuntimePackageVersion: strings.TrimSpace(opts.RuntimePackageVersion),
	}
	if scaffold.IsPackageOnlyJobTemplate(templateName) {
		return resolvePackageOnlyInitConfig(name, opts, config)
	}
	return resolveLauncherInitConfig(name, opts, config)
}
